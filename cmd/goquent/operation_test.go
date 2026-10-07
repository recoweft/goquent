package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/operation"
	"github.com/recoweft/goquent/orm/query"
)

func TestOperationCompileCommand(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	specPath := filepath.Join(dir, "operation.json")
	valuesPath := filepath.Join(dir, "values.json")

	m := operationTestManifest(false)
	m.Tables[0].Columns[3].TenantScope = false
	m.Tables[0].Policies[0].Type = "required_filter"
	writeJSON(t, manifestPath, m)
	limit := int64(10)
	writeJSON(t, specPath, operation.OperationSpec{
		Operation: operation.OperationSelect,
		Model:     "User",
		Select:    []string{"id", "name"},
		Filters:   []operation.FilterSpec{{Field: "tenant_id", Op: "=", ValueRef: "ordinary_value"}},
		Limit:     &limit,
	})
	writeJSON(t, valuesPath, map[string]any{"ordinary_value": "tenant-1"})

	var stdout, stderr bytes.Buffer
	code := run([]string{"operation", "compile", "--manifest", manifestPath, "--spec", specPath, "--values", valuesPath, "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected operation compile success, got %d stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"operation":"select"`)) {
		t.Fatalf("expected QueryPlan JSON, got %s", stdout.String())
	}
	if bytes.Contains(stdout.Bytes(), []byte(`tenant-1`)) {
		t.Fatalf("expected resolved value_ref param, got %s", stdout.String())
	}
}

func TestOperationCompileCommandRejectsStaleManifest(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	specPath := filepath.Join(dir, "operation.json")

	writeJSON(t, manifestPath, operationTestManifest(true))
	writeJSON(t, specPath, operation.OperationSpec{
		Operation: operation.OperationSelect,
		Model:     "User",
		Select:    []string{"id"},
		Filters:   []operation.FilterSpec{{Field: "tenant_id", Op: "=", Value: "tenant-1"}},
	})

	var stdout, stderr bytes.Buffer
	code := run([]string{"operation", "compile", "--manifest", manifestPath, "--spec", specPath, "--require-fresh-manifest"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected stale manifest compile error, got %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func TestOperationSchemaCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"operation", "schema"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected operation schema success, got %d stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Goquent OperationSpec")) {
		t.Fatalf("expected OperationSpec schema, got %s", stdout.String())
	}
}

func operationTestManifest(stale bool) manifest.Manifest {
	m := manifest.Manifest{
		Version:          manifest.Version,
		GeneratedAt:      time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
		GeneratorVersion: manifest.Generator,
		Tables: []manifest.Table{{
			Name:  "users",
			Model: "User",
			Columns: []manifest.Column{
				{Name: "deleted_at", SoftDelete: true, Nullable: true},
				{Name: "id", Primary: true},
				{Name: "name"},
				{Name: "tenant_id", RequiredFilter: true, TenantScope: true},
			},
			Policies: []manifest.Policy{
				{Type: "tenant_scope", Column: "tenant_id", Mode: query.PolicyModeEnforce},
				{Type: "soft_delete", Column: "deleted_at", Mode: query.PolicyModeEnforce},
			},
		}},
	}
	if stale {
		m.Verification = &manifest.Verification{Fresh: false, CheckedAt: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)}
	}
	return m
}

func TestOperationValuesSingleDocumentPrecision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "values.json")
	for _, input := range []string{`{"n":9007199254740993}`, `{"n":1} {}`, `{"n":1} bad`, `{"n":`} {
		if e := os.WriteFile(path, []byte(input), 0600); e != nil {
			t.Fatal(e)
		}
		values, e := loadOperationValues(path)
		if input == `{"n":9007199254740993}` {
			if e != nil || values["n"] != json.Number("9007199254740993") {
				t.Fatal(values, e)
			}
		} else if e == nil || values != nil {
			t.Fatal("failed input was returned", values, e)
		}
	}
}
