// Explicit maintenance command; verification never invokes this generator.
package main

import (
	"encoding/json"
	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/migration"
	"github.com/recoweft/goquent/orm/query"
	fixture "github.com/recoweft/goquent/tests/manifestfixture"
	"os"
	"time"
)

func must(err error) {
	if err != nil {
		panic("fixture generation failed")
	}
}
func write(path string, value any) {
	b, e := json.MarshalIndent(value, "", "  ")
	must(e)
	must(os.WriteFile(path, append(b, '\n'), 0600))
}
func main() {
	const dir = "tests/manifestfixture/"
	m, e := manifest.Load("examples/ai-safe-orm/goquent.manifest.json")
	must(e)
	src, e := manifest.GenerateRepositorySkeleton(m, manifest.RepositorySkeletonOptions{PackageName: "manifestfixture", TableName: "users", RowTypeName: "UserRow", RepositoryTypeName: "UserRepository", Typed: true})
	must(e)
	must(os.WriteFile(dir+"users_generated.go", src, 0600))
	// Registry input is fixed independently of the verification manifest/code hash.
	write(dir+"generator-input.json", m)
	var schema migration.Schema
	b, e := os.ReadFile("examples/ai-safe-orm/schema.json")
	must(e)
	must(json.Unmarshal(b, &schema))
	var policies []query.TablePolicy
	b, e = os.ReadFile("examples/ai-safe-orm/policies.json")
	must(e)
	must(json.Unmarshal(b, &policies))
	for _, dialect := range []string{"mysql", "postgres"} {
		db := fixture.Expected(dialect, "gq_manifest_fixture")
		write(dir+dialect+"-database.json", db)
		m, e = manifest.Generate(manifest.Options{Dialect: dialect, GeneratedAt: time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC), Schema: &schema, Policies: policies, GeneratedCodePaths: []string{dir + "users_generated.go"}, DatabaseSchema: &db})
		must(e)
		write(dir+dialect+".manifest.json", m)
	}
}
