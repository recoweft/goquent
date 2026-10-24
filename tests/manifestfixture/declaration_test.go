package manifestfixture

import (
	"encoding/json"
	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/migration"
	"github.com/recoweft/goquent/orm/query"
	"os"
	"reflect"
	"testing"
	"time"
)

// This compares independently committed artifacts without rewriting expectations.
func TestManifestEvidenceDeclarations(t *testing.T) {
	t.Chdir("../..")
	const prefix = "tests/manifestfixture/"
	read := func(path string, out any) {
		t.Helper()
		b, e := os.ReadFile(path)
		if e != nil || json.Unmarshal(b, out) != nil {
			t.Fatal("fixture input unavailable")
		}
	}
	var schema migration.Schema
	read("examples/ai-safe-orm/schema.json", &schema)
	var policies []query.TablePolicy
	read("examples/ai-safe-orm/policies.json", &policies)
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var db migration.Schema
			read(prefix+dialect+"-database.json", &db)
			declared := Expected(dialect, "gq_manifest_fixture")
			if !reflect.DeepEqual(db, declared) {
				t.Fatal("committed database declaration differs")
			}
			stored, e := manifest.Load(prefix + dialect + ".manifest.json")
			if e != nil {
				t.Fatal("manifest unavailable")
			}
			current, e := manifest.Generate(manifest.Options{Dialect: dialect, Schema: &schema, Policies: policies, GeneratedCodePaths: []string{prefix + "users_generated.go"}, DatabaseSchema: &db})
			if e != nil {
				t.Fatal("current inputs unavailable")
			}
			v := manifest.Verify(stored, current, time.Time{})
			if !v.Fresh || len(v.Checks) != 4 {
				t.Fatal("committed expectations stale")
			}
			for _, check := range v.Checks {
				if check.Status != "ok" {
					t.Fatal("axis not verified")
				}
			}
		})
	}
}
