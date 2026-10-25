// Package storedmodel owns the saved example's string-tenant repository model.
// It is independent of the handwritten executable's integer-tenant model.
package storedmodel

import (
	"fmt"
	"github.com/recoweft/goquent/orm/migration"
)

const CanonicalTable = "users"

// ExpectedDatabase declares the reader representation before any connection.
// Only table and its index identity may be instantiated for an owned native run.
// Versions, partial coverage and unknown defaults are deliberately not normalized.
func ExpectedDatabase(dialect, table string) migration.Schema {
	catalog, namespace, version := "def", "testdb", "8.4.6"
	types := []string{"bigint", "varchar(64)", "varchar(255)", "varchar(255)", "timestamp"}
	coverage := migration.SchemaCoverage{Tables: "partial", Columns: "partial", Indexes: "partial", ForeignKeys: "partial", Checks: "partial", Identity: "complete", Version: "complete"}
	if dialect == "postgres" {
		catalog, namespace, version = "testdb", "public", "16.10 (Debian 16.10-1.pgdg13+1)"
		types = []string{"bigint", "character varying", "character varying", "character varying", "timestamp without time zone"}
		coverage.Indexes, coverage.ForeignKeys, coverage.Checks = "complete", "complete", "complete"
	} else if dialect != "mysql" {
		panic("unsupported declaration dialect")
	}
	tc := coverage
	tc.Tables, tc.Version = "", ""
	t := migration.TableSchema{Name: table, Identity: migration.ObjectIdentity{Catalog: catalog, Schema: namespace, Name: table, State: "complete"}, Coverage: tc}
	for i, name := range []string{"id", "tenant_id", "name", "email", "deleted_at"} {
		t.Columns = append(t.Columns, migration.ColumnSchema{Name: name, Type: types[i], Nullable: i == 4, NullableKnown: true, DefaultState: "unknown", DefaultSource: dialect + ".information_schema.columns"})
	}
	index := migration.IndexSchema{Name: "idx_" + table + "_tenant_id", Columns: []string{"tenant_id"}, Keys: []migration.IndexKey{{Column: "tenant_id", Order: "asc", Nulls: "engine_default"}}, Method: "btree", Visibility: "visible", Attributes: "complete", Predicate: migration.ExpressionDefinition{State: "none"}}
	index.RawDefinition = "[{\"String\":\"tenant_id\",\"Valid\":true},{\"String\":\"\",\"Valid\":false},{\"Int64\":0,\"Valid\":false},{\"String\":\"A\",\"Valid\":true},\"BTREE\",\"YES\",1,1]\n"
	if dialect == "postgres" {
		index.Keys[0].Nulls = "last"
		index.RawDefinition = fmt.Sprintf("CREATE INDEX idx_%s_tenant_id ON public.%s USING btree (tenant_id)", table, table)
	}
	t.Indexes = []migration.IndexSchema{index}
	return migration.Schema{Context: &migration.SchemaContext{Dialect: dialect, Catalog: catalog, Schema: namespace, ServerVersion: version, VersionSource: "catalog_select", Scope: "tables", Tables: []string{table}, Visibility: "catalog_visible", Coverage: coverage}, Tables: []migration.TableSchema{t}}
}

// OwnedDDL describes the saved schema, not main.go. Callers must allocate a new
// safe identifier and never call this against an existing application table.
func OwnedDDL(table string) []string {
	return []string{
		"CREATE TABLE " + table + " (id BIGINT NOT NULL, tenant_id VARCHAR(64) NOT NULL, name VARCHAR(255) NOT NULL, email VARCHAR(255) NOT NULL, deleted_at TIMESTAMP NULL)",
		"CREATE INDEX idx_" + table + "_tenant_id ON " + table + " (tenant_id)",
	}
}
