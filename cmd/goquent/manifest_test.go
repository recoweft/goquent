package main

import (
	"bytes"
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/migration"
)

func TestManifestCommandGeneratesJSONAndSchema(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	writeJSON(t, schemaPath, migration.Schema{Tables: []migration.TableSchema{{
		Name: "users",
		Columns: []migration.ColumnSchema{
			{Name: "id", Type: "bigint", Nullable: false},
			{Name: "email", Type: "text", Nullable: false},
		},
	}}})

	var stdout, stderr bytes.Buffer
	code := run([]string{"manifest", "--format", "json", "--schema", schemaPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected manifest generation success, got %d stderr=%s", code, stderr.String())
	}
	var m manifest.Manifest
	if err := json.Unmarshal(stdout.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Tables) != 1 || m.Tables[0].Name != "users" {
		t.Fatalf("unexpected manifest output: %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"manifest", "schema"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected manifest schema success, got %d stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Goquent Manifest")) {
		t.Fatalf("expected schema output, got %s", stdout.String())
	}
}

func TestManifestVerifyDetectsStaleAndDoctor(t *testing.T) {
	dir := t.TempDir()
	oldSchemaPath := filepath.Join(dir, "old-schema.json")
	newSchemaPath := filepath.Join(dir, "new-schema.json")
	writeJSON(t, oldSchemaPath, migration.Schema{Tables: []migration.TableSchema{{
		Name:    "users",
		Columns: []migration.ColumnSchema{{Name: "id", Type: "bigint"}},
	}}})
	writeJSON(t, newSchemaPath, migration.Schema{Tables: []migration.TableSchema{{
		Name:    "users",
		Columns: []migration.ColumnSchema{{Name: "id", Type: "uuid"}},
	}}})

	stored, err := manifest.Generate(manifest.Options{
		GeneratedAt: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
		Schema:      loadTestSchema(t, oldSchemaPath),
	})
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	writeJSON(t, manifestPath, stored)

	var stdout, stderr bytes.Buffer
	code := run([]string{"manifest", "verify", "--manifest", manifestPath, "--schema", newSchemaPath, "--format", "json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected stale verify exit code 1, got %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"fresh": false`)) {
		t.Fatalf("expected stale JSON verification, got %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"doctor", "--manifest", manifestPath, "--schema", newSchemaPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected doctor stale exit code 1, got %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Manifest Verification")) {
		t.Fatalf("expected doctor verification output, got %s", stdout.String())
	}
}

func TestManifestVerifyAgainstDBFlagControlsDatabaseFingerprint(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.json")
	oldDBSchemaPath := filepath.Join(dir, "old-db-schema.json")
	newDBSchemaPath := filepath.Join(dir, "new-db-schema.json")
	writeJSON(t, schemaPath, migration.Schema{Tables: []migration.TableSchema{{
		Name:    "users",
		Columns: []migration.ColumnSchema{{Name: "id", Type: "bigint"}},
	}}})
	writeJSON(t, oldDBSchemaPath, migration.Schema{Tables: []migration.TableSchema{{
		Name:    "users",
		Columns: []migration.ColumnSchema{{Name: "id", Type: "bigint"}},
	}}})
	writeJSON(t, newDBSchemaPath, migration.Schema{Tables: []migration.TableSchema{{
		Name:    "users",
		Columns: []migration.ColumnSchema{{Name: "id", Type: "uuid"}},
	}}})

	stored, err := manifest.Generate(manifest.Options{
		GeneratedAt:    time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
		Schema:         loadTestSchema(t, schemaPath),
		DatabaseSchema: loadTestSchema(t, oldDBSchemaPath),
	})
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	writeJSON(t, manifestPath, stored)

	var stdout, stderr bytes.Buffer
	code := run([]string{"manifest", "verify", "--manifest", manifestPath, "--schema", schemaPath, "--database-schema", newDBSchemaPath, "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected database fingerprint to be ignored without --against-db, got %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var verification manifest.Verification
	if err := json.Unmarshal(stdout.Bytes(), &verification); err != nil {
		t.Fatal(err)
	}
	if statusForCheck(verification, "database") != "skipped" {
		t.Fatalf("expected skipped database check, got %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"manifest", "verify", "--manifest", manifestPath, "--schema", schemaPath, "--database-schema", newDBSchemaPath, "--against-db", "--format", "json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected stale database fingerprint with --against-db, got %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), &verification); err != nil {
		t.Fatal(err)
	}
	if statusForCheck(verification, "database") != "stale" {
		t.Fatalf("expected stale database check, got %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"manifest", "verify", "--manifest", manifestPath, "--schema", schemaPath, "--against-db"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected --against-db without --database-schema to fail, got %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
}

func TestManifestRepositoryCommandGeneratesSkeleton(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	writeJSON(t, manifestPath, manifest.Manifest{
		Version:          manifest.Version,
		GeneratedAt:      time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
		GeneratorVersion: manifest.Generator,
		Tables: []manifest.Table{{
			Name: "users",
			Columns: []manifest.Column{
				{Name: "id", Type: "bigint", Primary: true},
				{Name: "tenant_id", Type: "uuid", TenantScope: true, RequiredFilter: true},
				{Name: "email", Type: "text"},
			},
		}},
	})

	var stdout, stderr bytes.Buffer
	code := run([]string{"manifest", "repository", "--manifest", manifestPath, "--table", "users", "--package", "infra"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected repository skeleton success, got %d stderr=%s", code, stderr.String())
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "users_repository.go", stdout.Bytes(), parser.AllErrors); err != nil {
		t.Fatalf("generated code should parse: %v\n%s", err, stdout.String())
	}
	for _, want := range [][]byte{
		[]byte("package infra"),
		[]byte("type UserRepository struct"),
		[]byte("orm.RequirePredicate(\"users\", \"tenant_id\")"),
		[]byte("func UserTenantIDScope(value any) orm.Scope"),
	} {
		if !bytes.Contains(stdout.Bytes(), want) {
			t.Fatalf("expected skeleton to contain %s:\n%s", want, stdout.String())
		}
	}
}

func TestReviewCommandCanFailOnStaleManifest(t *testing.T) {
	dir := t.TempDir()
	stored, err := manifest.Generate(manifest.Options{
		GeneratedAt: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
		Schema: &migration.Schema{Tables: []migration.TableSchema{{
			Name:    "users",
			Columns: []migration.ColumnSchema{{Name: "id", Type: "bigint"}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	current, err := manifest.Generate(manifest.Options{
		GeneratedAt: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
		Schema: &migration.Schema{Tables: []migration.TableSchema{{
			Name:    "users",
			Columns: []migration.ColumnSchema{{Name: "id", Type: "uuid"}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	stored = manifest.AttachVerification(stored, manifest.Verify(stored, current, time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)))
	manifestPath := filepath.Join(dir, "manifest.json")
	writeJSON(t, manifestPath, stored)

	var stdout, stderr bytes.Buffer
	code := run([]string{"review", "--manifest", manifestPath, "--require-fresh-manifest", dir}, &stdout, &stderr)
	if code != 3 {
		t.Fatalf("expected stale manifest exit code 3, got %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func statusForCheck(v manifest.Verification, name string) string {
	for _, check := range v.Checks {
		if check.Name == name {
			return check.Status
		}
	}
	return ""
}

func loadTestSchema(t *testing.T, path string) *migration.Schema {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var schema migration.Schema
	if err := json.Unmarshal(b, &schema); err != nil {
		t.Fatal(err)
	}
	return &schema
}
