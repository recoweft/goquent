package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm/migration"
	"github.com/recoweft/goquent/orm/query"
)

type manifestUser struct {
	ID        int64      `db:"id,pk"`
	TenantID  string     `db:"tenant_id"`
	Email     string     `db:"email,pii"`
	DeletedAt *time.Time `db:"deleted_at"`
	Name      string
}

func (manifestUser) TableName() string { return "users" }

func TestGenerateFromModelSchemaAndPolicy(t *testing.T) {
	generatedAt := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	schema := migration.Schema{Tables: []migration.TableSchema{{
		Name: "users",
		Columns: []migration.ColumnSchema{
			{Name: "id", Type: "bigint", Nullable: false},
			{Name: "email", Type: "text", Nullable: false},
		},
		Indexes: []migration.IndexSchema{{Name: "users_email_idx", Columns: []string{"email"}, Unique: true}},
	}}}
	policies := []query.TablePolicy{{
		Table:                 "users",
		TenantColumn:          "tenant_id",
		TenantMode:            query.PolicyModeEnforce,
		SoftDeleteColumn:      "deleted_at",
		SoftDeleteMode:        query.PolicyModeEnforce,
		PIIColumns:            []string{"email"},
		PIIMode:               query.PolicyModeWarn,
		RequiredFilterColumns: []string{"tenant_id"},
		RequiredFilterMode:    query.PolicyModeBlock,
	}}

	m, err := Generate(Options{
		GeneratedAt: generatedAt,
		Dialect:     "postgres",
		Models:      []any{manifestUser{}},
		Schema:      &schema,
		Policies:    policies,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(m); err != nil {
		t.Fatal(err)
	}
	if m.Version != Version || m.Dialect != "postgres" {
		t.Fatalf("unexpected manifest metadata: %#v", m)
	}
	if len(m.Tables) != 1 || m.Tables[0].Name != "users" || m.Tables[0].Model != "manifestUser" {
		t.Fatalf("unexpected tables: %#v", m.Tables)
	}
	if !hasManifestColumnFlag(m.Tables[0], "email", func(c Column) bool { return c.PII }) {
		t.Fatalf("expected email pii flag, got %#v", m.Tables[0].Columns)
	}
	if !hasManifestColumnFlag(m.Tables[0], "tenant_id", func(c Column) bool { return c.TenantScope && c.RequiredFilter }) {
		t.Fatalf("expected tenant policy flags, got %#v", m.Tables[0].Columns)
	}
	if !hasPolicy(m.Tables[0], "soft_delete", "deleted_at") {
		t.Fatalf("expected soft delete policy, got %#v", m.Tables[0].Policies)
	}
	if m.SchemaFingerprint == "" || m.PolicyFingerprint == "" {
		t.Fatalf("expected fingerprints, got schema=%q policy=%q", m.SchemaFingerprint, m.PolicyFingerprint)
	}
}

func TestVerifyDetectsStaleManifest(t *testing.T) {
	generatedAt := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	stored, err := Generate(Options{
		GeneratedAt: generatedAt,
		Schema: &migration.Schema{Tables: []migration.TableSchema{{
			Name:    "users",
			Columns: []migration.ColumnSchema{{Name: "id", Type: "bigint"}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	current, err := Generate(Options{
		GeneratedAt: generatedAt,
		Schema: &migration.Schema{Tables: []migration.TableSchema{{
			Name:    "users",
			Columns: []migration.ColumnSchema{{Name: "id", Type: "uuid"}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}

	v := Verify(stored, current, generatedAt)
	if v.Fresh {
		t.Fatalf("expected stale verification, got %#v", v)
	}
	if !hasCheckStatus(v, "schema", "stale") {
		t.Fatalf("expected stale schema check, got %#v", v.Checks)
	}
	withVerification := AttachVerification(stored, v)
	if withVerification.Verification == nil || withVerification.Verification.Fresh {
		t.Fatalf("expected attached stale verification, got %#v", withVerification.Verification)
	}
}

func TestGeneratedCodeFingerprintAndLoad(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "model.go")
	if err := os.WriteFile(file, []byte("package model\ntype User struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := Generate(Options{GeneratedCodePaths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if m.GeneratedCodeFingerprint == "" {
		t.Fatalf("expected generated code fingerprint")
	}
	path := filepath.Join(dir, "manifest.json")
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.GeneratedCodeFingerprint != m.GeneratedCodeFingerprint {
		t.Fatalf("load mismatch: %s != %s", loaded.GeneratedCodeFingerprint, m.GeneratedCodeFingerprint)
	}
}

func TestJSONSchemaAndValidateVersion(t *testing.T) {
	b, err := JSONSchema()
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["title"] != "Goquent Manifest" {
		t.Fatalf("unexpected schema title: %#v", decoded["title"])
	}
	m, err := Generate(Options{})
	if err != nil {
		t.Fatal(err)
	}
	m.Version = "2"
	if err := Validate(m); err == nil {
		t.Fatalf("expected unsupported version validation error")
	}
}

func hasManifestColumnFlag(table Table, name string, ok func(Column) bool) bool {
	for _, column := range table.Columns {
		if column.Name == name && ok(column) {
			return true
		}
	}
	return false
}

func hasPolicy(table Table, typ, column string) bool {
	for _, policy := range table.Policies {
		if policy.Type == typ && policy.Column == column {
			return true
		}
	}
	return false
}

func hasCheckStatus(v Verification, name, status string) bool {
	for _, check := range v.Checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}
