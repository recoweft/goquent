package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/migration"
	"github.com/recoweft/goquent/orm/query"
	"github.com/recoweft/goquent/orm/review"
)

const cliCanary = "person-canary@example.test_PASSWORD_TOKEN_identifier"

func checkCLIOutput(t *testing.T, out, err *bytes.Buffer) {
	t.Helper()
	if strings.Contains(out.String()+err.String(), cliCanary) {
		t.Fatal("CLI canary escaped")
	}
}
func TestPublicCLIOutputsAndErrors(t *testing.T) {
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, cliCanary+".sql")
	if os.WriteFile(sqlPath, []byte("ALTER TABLE `"+cliCanary+"` DROP COLUMN secret;"), 0600) != nil {
		t.Fatal("fixture")
	}
	schema := filepath.Join(dir, "schema.json")
	writeJSON(t, schema, migration.Schema{Tables: []migration.TableSchema{{Name: cliCanary, Columns: []migration.ColumnSchema{{Name: cliCanary, Type: cliCanary, DefaultExpression: cliCanary}}}}})
	mf := filepath.Join(dir, "manifest.json")
	writeJSON(t, mf, manifest.Manifest{Version: manifest.Version, Tables: []manifest.Table{{Name: cliCanary, Columns: []manifest.Column{{Name: cliCanary, Default: cliCanary}}}}})
	bad := filepath.Join(dir, "invalid.json")
	_ = os.WriteFile(bad, []byte(cliCanary), 0600)
	commands := [][]string{{cliCanary}, {"operation", cliCanary}, {"migrate", cliCanary}, {"review", "--" + cliCanary}, {"review", "--fail-on", cliCanary}, {"review", "--config", bad}, {"review", bad}, {"manifest", "--schema", bad}, {"manifest", "verify", "--manifest", mf, "--schema", schema}, {"doctor", "--manifest", mf, "--schema", schema}, {"operation", "compile", "--manifest", bad, "--spec", bad}, {"mcp", "--manifest", bad}, {"migrate", "status", "--driver", cliCanary, "--dsn", cliCanary}, {"migrate", "schema", "--driver", cliCanary, "--dsn", cliCanary}, {"migrate", "drift", "--desired-schema", schema, "--database-schema", bad}, {"manifest", "repository", "--manifest", mf, "--table", cliCanary}}
	for _, format := range []string{"pretty", "json", "github"} {
		commands = append(commands, []string{"review", "--format", format, sqlPath})
	}
	for _, format := range []string{"pretty", "json"} {
		commands = append(commands, []string{"manifest", "--format", format, "--schema", schema}, []string{"migrate", "plan", "--format", format, sqlPath}, []string{"migrate", "dry-run", "--format", format, sqlPath}, []string{"migrate", "drift", "--format", format, "--desired-schema", schema, "--database-schema", schema})
	}
	for i, args := range commands {
		t.Run(string(rune('A'+i)), func(t *testing.T) { var out, err bytes.Buffer; run(args, &out, &err); checkCLIOutput(t, &out, &err) })
	}
}
func TestPublicReviewTailDoesNotWeakenExit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.json")
	for _, risk := range []query.RiskLevel{query.RiskHigh, query.RiskBlocked} {
		p := query.QueryPlan{Version: 1, Operation: query.OperationSelect, AnalysisPrecision: query.AnalysisPrecise, SQL: cliCanary}
		for range 300 {
			p.Warnings = append(p.Warnings, query.Warning{Code: query.WarningLimitMissing, Level: query.RiskMedium, Message: cliCanary})
		}
		p.Warnings = append(p.Warnings, query.Warning{Code: query.WarningDeleteWithoutWhere, Level: risk, Message: cliCanary})
		writeJSON(t, path, &p)
		var out, err bytes.Buffer
		if code := run([]string{"review", "--format", "json", "--fail-on", string(risk), path}, &out, &err); code != 1 {
			t.Fatalf("tail risk lost: %d", code)
		}
		var v review.ReportView
		if json.Unmarshal(out.Bytes(), &v) != nil || !v.Truncated || len(v.Findings) != 256 || v.FindingCount < 301 {
			t.Fatal("expected bounded full-count view")
		}
		for _, f := range v.Findings {
			if f.Level == string(risk) {
				t.Fatal("tail fixture unexpectedly visible")
			}
		}
		checkCLIOutput(t, &out, &err)
	}
}
func TestExplicitLocalArtifactRestrictions(t *testing.T) {
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "TF_BUILD", "BUILDKITE", "CIRCLECI", "TRAVIS", "JENKINS_URL", "TEAMCITY_VERSION"} {
		t.Setenv(key, "")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "new.json")
	if err := saveLocalArtifact(path, []byte(cliCanary)); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("permissions")
	}
	if err = saveLocalArtifact(path, []byte("overwrite")); err == nil {
		t.Fatal("existing file overwritten")
	}
	b, _ := os.ReadFile(path)
	if string(b) != cliCanary {
		t.Fatal("existing data changed")
	}
	link := filepath.Join(dir, "link")
	if os.Symlink(path, link) != nil {
		t.Fatal("symlink fixture")
	}
	fifo := filepath.Join(dir, "fifo")
	if syscall.Mkfifo(fifo, 0600) != nil {
		t.Fatal("fifo fixture")
	}
	for _, p := range []string{"-", "/dev/stdout", "/proc/self/fd/1", dir, link, fifo} {
		if saveLocalArtifact(p, []byte(cliCanary)) == nil {
			t.Fatal("invalid sink accepted")
		}
	}
	for _, key := range []string{"CI", "GITHUB_ACTIONS"} {
		t.Setenv(key, "true")
		if saveLocalArtifact(filepath.Join(dir, key), []byte(cliCanary)) == nil {
			t.Fatal("CI export accepted")
		}
		t.Setenv(key, "")
	}
	var out, stderr bytes.Buffer
	code := run([]string{"manifest", "--unsafe-local-output", filepath.Join(dir, "manifest.json")}, &out, &stderr)
	if code != 0 {
		t.Fatal("local command failed")
	}
	checkCLIOutput(t, &out, &stderr)
}

func TestPublicOperationAndMCPCLI(t *testing.T) {
	dir := t.TempDir()
	mf := filepath.Join(dir, "manifest.json")
	spec := filepath.Join(dir, "spec.json")
	values := filepath.Join(dir, "values.json")
	writeJSON(t, mf, operationTestManifest(false))
	writeJSON(t, spec, map[string]any{"version": 1, "operation": "select", "model": "users", "select": []string{"id", "name"}, "filters": []map[string]any{{"field": "tenant_id", "op": "=", "value_ref": "tenant"}}})
	writeJSON(t, values, map[string]any{"tenant": cliCanary})
	for _, format := range []string{"json", "pretty"} {
		var out, stderr bytes.Buffer
		code := run([]string{"operation", "compile", "--manifest", mf, "--spec", spec, "--values", values, "--format", format}, &out, &stderr)
		if code != 1 {
			t.Fatalf("compile exit %d", code)
		}
		checkCLIOutput(t, &out, &stderr)
		if out.Len() != 0 || !strings.Contains(stderr.String(), "details omitted") {
			t.Fatal("tenant refusal was not fixed")
		}
	}
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"review_query","arguments":{"sql":"SELECT '` + cliCanary + `'"}}}` + "\n"
	var out, stderr bytes.Buffer
	if runMCP([]string{"--manifest", mf}, strings.NewReader(input), &out, &stderr) != 0 {
		t.Fatal("MCP CLI failed")
	}
	checkCLIOutput(t, &out, &stderr)
}
