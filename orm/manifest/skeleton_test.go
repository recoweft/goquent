package manifest

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateRepositorySkeletonForTable(t *testing.T) {
	src, err := GenerateRepositorySkeletonForTable(Table{
		Name: "users",
		Columns: []Column{
			{Name: "id", Type: "bigint", Primary: true},
			{Name: "tenant_id", Type: "uuid", TenantScope: true, RequiredFilter: true},
			{Name: "email", Type: "text", PII: true},
			{Name: "payload_json", Type: "jsonb"},
			{Name: "deleted_at", Type: "timestamp", Nullable: true, SoftDelete: true},
		},
	}, RepositorySkeletonOptions{PackageName: "infra"})
	if err != nil {
		t.Fatalf("GenerateRepositorySkeletonForTable: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "users_repository.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated code should parse: %v\n%s", err, string(src))
	}
	out := string(src)
	for _, want := range []string{
		"package infra",
		"type UserRow struct",
		"ID          int64",
		"`db:\"id,pk\"`",
		"TenantID    string",
		"`db:\"tenant_id\"`",
		"PayloadJSON orm.JSONField[map[string]any]",
		"DeletedAt   *time.Time",
		"orm.RequirePredicate(\"users\", \"tenant_id\")",
		"q = q.WhereNull(\"deleted_at\")",
		"func UserTenantIDScope(value any) orm.Scope",
		"func (r *UserRepository) FindByID",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected generated code to contain %q:\n%s", want, out)
		}
	}
}

func TestGenerateRepositorySkeletonRequiresTableWhenAmbiguous(t *testing.T) {
	_, err := GenerateRepositorySkeleton(&Manifest{Tables: []Table{{Name: "users"}, {Name: "documents"}}}, RepositorySkeletonOptions{})
	if err == nil || !strings.Contains(err.Error(), "--table is required") {
		t.Fatalf("expected table selection error, got %v", err)
	}
}
