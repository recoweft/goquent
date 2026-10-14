// Package typedfixture supplies fixed fictional generation inputs.
package typedfixture

import (
	"embed"
	"encoding/json"
	"github.com/recoweft/goquent/orm/manifest"
)

//go:embed inputs/*.json
var snapshots embed.FS

func Manifest(dialect string) *manifest.Manifest {
	b, e := snapshots.ReadFile("inputs/" + dialect + ".json")
	if e != nil {
		panic("fixture input unavailable")
	}
	var m manifest.Manifest
	if json.Unmarshal(b, &m) != nil {
		panic("fixture input invalid")
	}
	return &m
}
func TenantManifest() *manifest.Manifest { return Manifest("tenant") }

func Options(dialect string) manifest.RepositorySkeletonOptions {
	prefix := "MySQLRecord"
	if dialect == "postgres" {
		prefix = "PostgresRecord"
	}
	return manifest.RepositorySkeletonOptions{Typed: true, PackageName: "typedfixture", RowTypeName: prefix + "Row", RepositoryTypeName: prefix + "Repository", Projections: []manifest.RepositoryProjection{{Name: "Summary", Columns: []string{"id", "segment", "status", "active", "flag", "note", "balance", "happened", "clock", "stamp"}}, {Name: "Identity", Columns: []string{"segment", "id"}}}}
}

func TenantOptions() manifest.RepositorySkeletonOptions {
	return manifest.RepositorySkeletonOptions{Typed: true, PackageName: "typedfixture", RowTypeName: "TenantRecordRow", RepositoryTypeName: "TenantRecordRepository", Projections: []manifest.RepositoryProjection{{Name: "Identity", Columns: []string{"id", "tenant_id", "active"}}}}
}
