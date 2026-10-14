// Package typedfixture supplies fictional declarations shared by generator and
// execution tests. These declarations do not attest any live database.
package typedfixture

import (
	"github.com/recoweft/goquent/orm/manifest"
	"time"
)

func Manifest(dialect string) *manifest.Manifest {
	temporal := "datetime(6)"
	if dialect == "postgres" {
		temporal = "timestamp(6) without time zone"
	}
	cols := []manifest.Column{
		{Name: "id", Type: "bigint", Primary: true}, {Name: "segment", Type: "integer", Primary: true},
		{Name: "status", Type: "varchar(12)", EnumValues: []string{"ready", "closed", "a-b", "a_b", "type"}},
		{Name: "active", Type: "boolean"}, {Name: "flag", Type: "boolean", Nullable: true},
		{Name: "note", Type: "text", Nullable: true}, {Name: "balance", Type: "decimal(30,4)"},
		{Name: "happened", Type: "date"}, {Name: "clock", Type: "time(6)"}, {Name: "stamp", Type: temporal},
		{Name: "immutable", Type: "text", Readonly: true}, {Name: "computed", Type: "integer", Generated: true},
		{Name: "hidden", Type: "text", Forbidden: true},
	}
	for i := range cols {
		cols[i].TypeSource = "sql"
		cols[i].NullableKnown = true
	}
	return &manifest.Manifest{Version: "1", Dialect: dialect, GeneratorVersion: "fixture", GeneratedAt: time.Unix(1, 0).UTC(), SchemaFingerprint: "fictional-schema", PolicyFingerprint: "fictional-policy", Tables: []manifest.Table{{Name: "gq08_records", Columns: cols}}}
}
func Options(dialect string) manifest.RepositorySkeletonOptions {
	prefix := "MySQLRecord"
	if dialect == "postgres" {
		prefix = "PostgresRecord"
	}
	return manifest.RepositorySkeletonOptions{Typed: true, PackageName: "typedfixture", RowTypeName: prefix + "Row", RepositoryTypeName: prefix + "Repository", Projections: []manifest.RepositoryProjection{{Name: "Summary", Columns: []string{"id", "segment", "status", "active", "flag", "note", "balance", "happened", "clock", "stamp"}}, {Name: "Identity", Columns: []string{"segment", "id"}}}}
}

func TenantManifest() *manifest.Manifest {
	return &manifest.Manifest{Version: "1", Dialect: "postgres", Tables: []manifest.Table{{Name: "gq08_tenants", Columns: []manifest.Column{
		{Name: "id", Type: "bigint", TypeSource: "sql", NullableKnown: true, Primary: true},
		{Name: "tenant_id", Type: "bigint", TypeSource: "sql", NullableKnown: true, Primary: true, TenantScope: true, RequiredFilter: true},
		{Name: "active", Type: "boolean", TypeSource: "sql", NullableKnown: true},
	}}}}
}
func TenantOptions() manifest.RepositorySkeletonOptions {
	return manifest.RepositorySkeletonOptions{Typed: true, PackageName: "typedfixture", RowTypeName: "TenantRecordRow", RepositoryTypeName: "TenantRecordRepository", Projections: []manifest.RepositoryProjection{{Name: "Identity", Columns: []string{"id", "tenant_id", "active"}}}}
}
