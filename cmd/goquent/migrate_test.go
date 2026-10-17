package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/recoweft/goquent/orm/migration"
	"github.com/recoweft/goquent/orm/publicoutput"
)

func TestMigratePlanFailOnAndJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "001_drop.sql")
	if err := os.WriteFile(path, []byte("DROP TABLE users;"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"migrate", "plan", "--format", "json", "--fail-on", "destructive", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected destructive threshold exit code 1, got %d stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"steps"`)) {
		t.Fatalf("expected JSON migration plan, got %s", stdout.String())
	}
}

func TestMigrateDryRunRequiresApprovalForDestructiveMigration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "001_drop.sql")
	if err := os.WriteFile(path, []byte("ALTER TABLE users DROP COLUMN legacy_id;"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"migrate", "dry-run", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected dry-run without approval to fail, got %d stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"migrate", "dry-run", "--approve", "legacy column retired", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected approved dry-run to pass, got %d stderr=%s", code, stderr.String())
	}
}

func TestMigratePlanBackfillReviewMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "001_backfill.sql")
	if err := os.WriteFile(path, []byte("ALTER TABLE users ALTER COLUMN email SET NOT NULL;"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"migrate", "plan", "--format", "json", "--review-mode", "backfill", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected backfill review plan success, got %d stderr=%s", code, stderr.String())
	}
	if bytes.Contains(stdout.Bytes(), []byte(`"review_mode"`)) ||
		!bytes.Contains(stdout.Bytes(), []byte(migration.WarningMigrationBackfillReview)) {
		t.Fatalf("expected backfill metadata/warning, got %s", stdout.String())
	}
}

func TestMigrateApplyChecksApprovalBeforeDSN(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "001_drop.sql")
	if err := os.WriteFile(path, []byte("DROP TABLE users;"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"migrate", "apply", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected apply without approval to fail before DSN validation, got %d stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"migrate", "apply", "--approve", "approved cleanup", path}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected approved apply without DSN to return config error, got %d stderr=%s", code, stderr.String())
	}
}

func TestMigrateStatusRequiresDriverAndDSN(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"migrate", "status"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected status config error, got %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("requires --driver and --dsn")) {
		t.Fatalf("expected driver/dsn error, got %s", stderr.String())
	}
}

func TestMigrateSchemaRequiresDriverAndDSN(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"migrate", "schema"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected schema config error, got %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("requires --driver and --dsn")) {
		t.Fatalf("expected driver/dsn error, got %s", stderr.String())
	}
}

func TestMigrateDriftCommand(t *testing.T) {
	dir := t.TempDir()
	desiredPath := filepath.Join(dir, "desired.json")
	currentPath := filepath.Join(dir, "current.json")
	writeJSON(t, desiredPath, migration.Schema{Tables: []migration.TableSchema{{
		Name:    "users",
		Columns: []migration.ColumnSchema{{Name: "id", Type: "uuid", DefaultState: migration.DefaultNone}},
	}}})
	writeJSON(t, currentPath, migration.Schema{Tables: []migration.TableSchema{{
		Name:    "users",
		Columns: []migration.ColumnSchema{{Name: "id", Type: "bigint", DefaultState: migration.DefaultNone}},
	}}})

	var stdout, stderr bytes.Buffer
	code := run([]string{"migrate", "drift", "--desired-schema", desiredPath, "--database-schema", currentPath, "--format", "json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected drift exit code 1, got %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var report publicoutput.SummaryView
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode drift report: %v", err)
	}
	if !report.Drifted || report.Count != 1 {
		t.Fatalf("unexpected drift report: %#v", report)
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"migrate", "drift", "--desired-schema", desiredPath, "--database-schema", desiredPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected no-drift exit code 0, got %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"drifted":false`)) {
		t.Fatalf("expected no drift pretty output, got %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"migrate", "drift", "--desired-schema", desiredPath}, &stdout, &stderr)
	if code != 2 || !bytes.Contains(stderr.Bytes(), []byte("PUBLIC_OUTPUT")) {
		t.Fatalf("expected input error, got code=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
}

func TestReadDesiredVersionsFromFlagsAndFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "desired.txt")
	if err := os.WriteFile(path, []byte("\n# comment\n002_add_documents\n003_add_events\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	versions, err := readDesiredVersions([]string{"001_create_users"}, []string{path})
	if err != nil {
		t.Fatalf("read desired versions: %v", err)
	}
	want := []string{"001_create_users", "002_add_documents", "003_add_events"}
	if !reflect.DeepEqual(versions, want) {
		t.Fatalf("versions mismatch:\nwant %#v\ngot  %#v", want, versions)
	}
}

func TestMigrationStatusDialect(t *testing.T) {
	if _, err := migrationStatusDialect("postgresql"); err != nil {
		t.Fatalf("postgresql dialect: %v", err)
	}
	if _, err := migrationStatusDialect("sqlite"); err == nil {
		t.Fatalf("expected unsupported driver error")
	}
}

func TestSchemaComparisonUnknownIsNonExecuting(t *testing.T) {
	t.Setenv("TEST_DB_DSN", "invalid://must-not-connect")
	t.Setenv("TEST_MYSQL_DSN", "invalid://must-not-connect")
	t.Setenv("TEST_POSTGRES_DSN", "invalid://must-not-connect")
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.json")
	writeJSON(t, path, migration.Schema{Tables: []migration.TableSchema{{Name: "secret_identifier", Columns: []migration.ColumnSchema{{Name: "private_column", Type: "text"}}}}})
	var out, errout bytes.Buffer
	code := run([]string{"migrate", "drift", "--desired-schema", path, "--database-schema", path, "--format", "json"}, &out, &errout)
	if code != 2 || !bytes.Contains(out.Bytes(), []byte(`"unknown":true`)) || !bytes.Contains(out.Bytes(), []byte(`"drifted":false`)) {
		t.Fatalf("unknown comparison reported complete: %d", code)
	}
	if bytes.Contains(out.Bytes(), []byte("secret_identifier")) || bytes.Contains(out.Bytes(), []byte("private_column")) {
		t.Fatal("identifier leak")
	}
}

func TestWholeTableUnknownCLI(t *testing.T) {
	t.Setenv("TEST_DB_DSN", "invalid://must-not-connect")
	t.Setenv("TEST_MYSQL_DSN", "invalid://must-not-connect")
	t.Setenv("TEST_POSTGRES_DSN", "invalid://must-not-connect")
	dir := t.TempDir()
	empty, unknown := filepath.Join(dir, "empty.json"), filepath.Join(dir, "unknown.json")
	writeJSON(t, empty, migration.Schema{})
	writeJSON(t, unknown, migration.Schema{Tables: []migration.TableSchema{{Name: "private_table", Columns: []migration.ColumnSchema{{Name: "private_column", DefaultExpression: "private_default"}}, Indexes: []migration.IndexSchema{{Name: "private_index", DefinitionUnknown: true, RawDefinition: "private_sql"}}}}})
	for _, drop := range []bool{false, true} {
		desired, current := unknown, empty
		if drop {
			desired, current = current, desired
		}
		var out, errout bytes.Buffer
		code := run([]string{"migrate", "drift", "--desired-schema", desired, "--database-schema", current, "--format", "json"}, &out, &errout)
		if code != 2 || !bytes.Contains(out.Bytes(), []byte(`"unknown":true`)) || !bytes.Contains(out.Bytes(), []byte(`"known":false`)) || !bytes.Contains(out.Bytes(), []byte(`"drifted":true`)) {
			t.Fatalf("unknown table reported complete: %d %s", code, out.String())
		}
		if bytes.Contains(out.Bytes(), []byte("private_")) || bytes.Contains(errout.Bytes(), []byte("private_")) {
			t.Fatal("public leak")
		}
	}
}
