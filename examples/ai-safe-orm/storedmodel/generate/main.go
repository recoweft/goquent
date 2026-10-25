// Explicit maintenance only. Verification must never run this command.
package main

import (
	"encoding/json"
	model "github.com/recoweft/goquent/examples/ai-safe-orm/storedmodel"
	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/migration"
	"github.com/recoweft/goquent/orm/query"
	"os"
	"time"
)

func must(err error) {
	if err != nil {
		panic("stored model maintenance failed; details omitted")
	}
}
func read(path string, out any) { b, e := os.ReadFile(path); must(e); must(json.Unmarshal(b, out)) }
func write(path string, value any) {
	b, e := json.MarshalIndent(value, "", "  ")
	must(e)
	must(os.WriteFile(path, append(b, '\n'), 0600))
}
func main() {
	const root = "examples/ai-safe-orm/"
	const dir = root + "storedmodel/"
	var schema migration.Schema
	var policies []query.TablePolicy
	read(root+"schema.json", &schema)
	read(root+"policies.json", &policies)
	// Fixed timestamp and schema/policy-only input avoid a code-hash feedback loop.
	at, e := time.Parse(time.RFC3339Nano, "2026-10-22T17:01:50.310045946Z")
	must(e)
	input, e := manifest.Generate(manifest.Options{Dialect: "mysql", GeneratedAt: at, Schema: &schema, Policies: policies})
	must(e)
	write(dir+"generator-input.json", input)
	code, e := manifest.GenerateRepositorySkeleton(input, manifest.RepositorySkeletonOptions{PackageName: "storedmodel", TableName: "users", RowTypeName: "StoredUserRow", RepositoryTypeName: "StoredUserRepository", Typed: true})
	must(e)
	must(os.WriteFile(dir+"users_generated.go", code, 0600))
	for _, dialect := range []string{"mysql", "postgres"} {
		db := model.ExpectedDatabase(dialect, model.CanonicalTable)
		write(dir+dialect+"-database.json", db)
		expected, e := manifest.Generate(manifest.Options{Dialect: dialect, GeneratedAt: at, Schema: &schema, Policies: policies, GeneratedCodePaths: []string{dir + "users_generated.go"}, DatabaseSchema: &db})
		must(e)
		path := root + "goquent.manifest.json"
		if dialect == "postgres" {
			path = dir + "postgres.manifest.json"
		}
		write(path, expected)
	}
}
