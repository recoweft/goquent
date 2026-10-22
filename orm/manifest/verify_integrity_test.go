package manifest

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm/migration"
	"github.com/recoweft/goquent/orm/query"
)

func TestVerifyIntegrityBothInputs(t *testing.T) {
	makeManifest := func() *Manifest {
		m, err := Generate(Options{Schema: &migration.Schema{Tables: []migration.TableSchema{{Name: "canary_table", Columns: []migration.ColumnSchema{{Name: "canary_column", Type: "text", NullableKnown: true}}, Indexes: []migration.IndexSchema{{Name: "canary_index", Columns: []string{"canary_column"}}}}}}, Policies: []query.TablePolicy{{Table: "canary_table", PIIColumns: []string{"canary_column"}}}})
		if err != nil {
			t.Fatal("generation failed")
		}
		return m
	}
	cases := []struct {
		name, check string
		mutate      func(*Manifest)
	}{
		{"unknown_version", "manifest", func(m *Manifest) { m.Version = "canary_version" }},
		{"missing_version", "manifest", func(m *Manifest) { m.Version = "" }},
		{"missing_time", "manifest", func(m *Manifest) { m.GeneratedAt = time.Time{} }},
		{"missing_generator", "manifest", func(m *Manifest) { m.GeneratorVersion = "" }},
		{"missing_schema_hash", "manifest", func(m *Manifest) { m.SchemaFingerprint = "" }},
		{"missing_policy_hash", "manifest", func(m *Manifest) { m.PolicyFingerprint = "" }},
		{"type", "schema", func(m *Manifest) { m.Tables[0].Columns[0].Type = "canary_type" }},
		{"default", "schema", func(m *Manifest) { m.Tables[0].Columns[0].Default = "canary_default" }},
		{"index", "schema", func(m *Manifest) { m.Tables[0].Indexes[0].Unique = true }},
		{"policy", "policy", func(m *Manifest) { m.Tables[0].Policies[0].Mode = "block" }},
		{"column_policy", "policy", func(m *Manifest) { m.Tables[0].Columns[0].PII = false }},
		{"schema_hash", "schema", func(m *Manifest) { m.SchemaFingerprint = "sha256:canary" }},
		{"policy_hash", "policy", func(m *Manifest) { m.PolicyFingerprint = "sha256:canary" }},
	}
	for _, tc := range cases {
		for _, side := range []string{"stored", "current", "both"} {
			t.Run(tc.name+"/"+side, func(t *testing.T) {
				a, b := makeManifest(), makeManifest()
				if side != "current" {
					tc.mutate(a)
				}
				if side != "stored" {
					tc.mutate(b)
				}
				beforeA, _ := json.Marshal(a)
				beforeB, _ := json.Marshal(b)
				v := Verify(a, b, time.Time{})
				if v.Fresh || !hasCheckStatus(v, tc.check, "stale") {
					t.Fatal("invalid input accepted or incorrectly classified")
				}
				afterA, _ := json.Marshal(a)
				afterB, _ := json.Marshal(b)
				if string(beforeA) != string(afterA) || string(beforeB) != string(afterB) {
					t.Fatal("verification mutated inputs")
				}
			})
		}
	}
	a, b := makeManifest(), makeManifest()
	if v := Verify(a, b, time.Time{}); !v.Fresh || !hasCheckStatus(v, "database", "skipped") || !hasCheckStatus(v, "generated_code", "skipped") {
		t.Fatal("valid inputs rejected")
	}
}
