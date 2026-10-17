package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/migration"
)

func TestPreflightCLISeparatedModes(t *testing.T) {
	// Exercise local export under an explicit local environment, independently of
	// the host CI runner. The production CI refusal is asserted again below.
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "TF_BUILD", "BUILDKITE", "CIRCLECI", "TRAVIS", "JENKINS_URL", "TEAMCITY_VERSION"} {
		t.Setenv(key, "")
	}

	dir := t.TempDir()
	source := filepath.Join(dir, "plan.json")
	outFile := filepath.Join(dir, "evidence.json")
	schema := migration.Schema{Context: &migration.SchemaContext{Dialect: "postgres", Catalog: "testdb", Schema: "private_ns", Scope: "tables", Tables: []string{"private_table"}, Coverage: migration.SchemaCoverage{Identity: migration.CoverageComplete}}, Tables: []migration.TableSchema{{Name: "private_table", Identity: migration.ObjectIdentity{Catalog: "testdb", Schema: "private_ns", Name: "private_table", State: migration.CoverageComplete}, Columns: []migration.ColumnSchema{{Name: "private_column", Type: "integer", NullableKnown: true, Nullable: true}}}}}
	n := false
	p := migration.MigrationPlan{CurrentSchema: &schema, DesiredSchema: &schema, Steps: []migration.MigrationStep{{Type: migration.AlterNullability, Table: "private_table", Column: "private_column", Nullable: &n, SQL: "SECRET_SQL"}}}
	b, _ := json.Marshal(p)
	if e := os.WriteFile(source, b, 0600); e != nil {
		t.Fatal(e)
	}
	var out, errs bytes.Buffer
	// Planning never receives a DB. A missing connection configuration is valid.
	code := run([]string{"migrate", "preflight", "plan", "--plan-source", source, "--unsafe-local-output", outFile}, &out, &errs)
	if code != 0 {
		t.Fatalf("%d %s", code, errs.String())
	}
	for _, secret := range []string{"private_ns", "private_table", "private_column", "SECRET_SQL"} {
		if strings.Contains(out.String()+errs.String(), secret) {
			t.Fatal("public leak")
		}
	}
	out.Reset()
	errs.Reset()
	code = run([]string{"migrate", "preflight", "verify", "--source", outFile}, &out, &errs)
	if code != 1 {
		t.Fatalf("offline %d %s", code, errs.String())
	}
	for _, args := range [][]string{
		{"migrate", "preflight", "verify", "--source", outFile, "--dsn", "SECRET"},
		{"migrate", "preflight", "plan", "--plan-source", source, "--driver", "postgres"},
		{"migrate", "preflight", "collect", "--source", outFile},
	} {
		out.Reset()
		errs.Reset()
		if run(args, &out, &errs) != 2 || strings.Contains(out.String()+errs.String(), "SECRET") {
			t.Fatal("mode isolation")
		}
	}
	t.Setenv("CI", "true")
	out.Reset()
	errs.Reset()
	if run([]string{"migrate", "preflight", "plan", "--plan-source", source, "--unsafe-local-output", filepath.Join(dir, "ci.json")}, &out, &errs) != 2 {
		t.Fatal("CI export accepted")
	}
}

func TestPreflightCLIRejectsAmbiguousSource(t *testing.T) {
	for _, raw := range []string{`{"version":1,"version":1}`, `{"version":null}`, `{"Version":1}`, `{"version":1,"plan":{"steps":[{"definition":{"type":"add_index","type":"drop_table"}}]}}`} {
		path := filepath.Join(t.TempDir(), "input.json")
		if e := os.WriteFile(path, []byte(raw), 0600); e != nil {
			t.Fatal(e)
		}
		var out, errs bytes.Buffer
		if run([]string{"migrate", "preflight", "verify", "--source", path}, &out, &errs) != 2 {
			t.Fatal("ambiguous accepted")
		}
	}
}
