package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/manifest"
)

func TestManifestIntegrityConsumers(t *testing.T) {
	schema := "../../examples/ai-safe-orm/schema.json"
	policy := "../../examples/ai-safe-orm/policies.json"
	for _, tc := range []struct {
		name   string
		code   int
		mutate func(*manifest.Manifest)
	}{
		{"fresh", 0, func(*manifest.Manifest) {}},
		{"unknown_version", 2, func(m *manifest.Manifest) { m.Version = "CANARY_SECRET_VERSION" }},
		{"missing_version", 2, func(m *manifest.Manifest) { m.Version = "" }},
		{"type", 1, func(m *manifest.Manifest) { m.Tables[0].Columns[0].Type = "CANARY_SECRET_TYPE" }},
		{"policy", 1, func(m *manifest.Manifest) { m.Tables[0].Policies[0].Column = "CANARY_SECRET_POLICY" }},
		{"stale_hash", 1, func(m *manifest.Manifest) { m.SchemaFingerprint = "sha256:CANARY_SECRET_HASH" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, err := buildManifestFromFlags("mysql", schema, policy, "", "", nil)
			if err != nil {
				t.Fatal("generation failed")
			}
			tc.mutate(m)
			p := filepath.Join(t.TempDir(), "manifest.json")
			writeJSON(t, p, m)
			for _, command := range []string{"verify", "diff", "doctor"} {
				for _, format := range []string{"pretty", "json"} {
					args := []string{"manifest", command, "--manifest", p, "--schema", schema, "--policy", policy, "--format", format}
					if command == "doctor" {
						args = []string{"doctor", "--manifest", p, "--schema", schema, "--policy", policy}
					}
					var out, diag bytes.Buffer
					if code := run(args, &out, &diag); code != tc.code {
						t.Fatalf("%s/%s: incorrect exit %d", command, format, code)
					}
					if strings.Contains(out.String()+diag.String(), "CANARY_SECRET") || strings.Contains(out.String()+diag.String(), p) {
						t.Fatal("public output leaked source")
					}
				}
			}
			loaded, err := loadMCPManifest(p, schema, policy, "", nil)
			if err != nil {
				t.Fatal("MCP loading failed")
			}
			if loaded.Verification == nil || loaded.Verification.Fresh != (tc.code == 0) {
				t.Fatal("MCP current-input verification mismatch")
			}
			if b, _ := json.Marshal(loaded.Verification.PublicView()); bytes.Contains(b, []byte("CANARY_SECRET")) {
				t.Fatal("MCP verification leak")
			}
			for _, format := range []string{"json", "sarif"} {
				var out, diag bytes.Buffer
				code := run([]string{"review", "--manifest", p, "--schema", schema, "--policy", policy, "--require-fresh-manifest", "--format", format, t.TempDir()}, &out, &diag)
				if tc.code != 0 && code != 3 {
					t.Fatalf("review %s did not refuse stale manifest: %d", format, code)
				}
				if strings.Contains(out.String()+diag.String(), "CANARY_SECRET") {
					t.Fatal("review output leaked source")
				}
			}
		})
	}
}

func TestExampleManifestRegeneration(t *testing.T) {
	// Generated-code hashing includes the literal repo-relative path.
	t.Chdir("../..")
	schema := "examples/ai-safe-orm/schema.json"
	policy := "examples/ai-safe-orm/policies.json"
	database := "examples/ai-safe-orm/storedmodel/mysql-database.json"
	code := "examples/ai-safe-orm/storedmodel/users_generated.go"
	stored, err := manifest.Load("examples/ai-safe-orm/goquent.manifest.json")
	if err != nil {
		t.Fatal("fixture loading failed")
	}
	a, err := buildManifestFromFlags("mysql", schema, policy, database, "", []string{code})
	if err != nil {
		t.Fatal("generation failed")
	}
	b, err := buildManifestFromFlags("mysql", schema, policy, database, "", []string{code})
	if err != nil {
		t.Fatal("regeneration failed")
	}
	a.GeneratedAt = stored.GeneratedAt
	b.GeneratedAt = stored.GeneratedAt
	x, _ := a.ToJSON()
	y, _ := b.ToJSON()
	z, _ := stored.ToJSON()
	if !bytes.Equal(x, y) || !bytes.Equal(x, z) {
		t.Fatal("example regeneration differs beyond generation time")
	}
	legacy, err := manifest.Load("orm/manifest/testdata/legacy-example.json")
	if err != nil {
		t.Fatal("legacy fixture loading failed")
	}
	v := manifest.Verify(legacy, a, a.GeneratedAt)
	if v.Fresh {
		t.Fatal("legacy fixture accepted")
	}
	for _, c := range legacy.Tables[0].Columns {
		if c.TypeSource != "" {
			t.Fatal("legacy type source evidence changed")
		}
	}
	// Capture through the same guarded artifact route used by CLI consumers.
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "TF_BUILD", "BUILDKITE", "CIRCLECI", "TRAVIS", "JENKINS_URL", "TEAMCITY_VERSION"} {
		t.Setenv(key, "")
	}
	p := filepath.Join(t.TempDir(), "capture.json")
	var out, diag bytes.Buffer
	if run([]string{"manifest", "--dialect", "mysql", "--schema", schema, "--policy", policy, "--code", code, "--database-schema", database, "--unsafe-local-output", p}, &out, &diag) != 0 {
		t.Fatal("capture failed")
	}
	captured, err := manifest.Load(p)
	if err != nil {
		t.Fatal("capture load failed")
	}
	captured.GeneratedAt = stored.GeneratedAt
	c, _ := captured.ToJSON()
	if !bytes.Equal(c, z) {
		t.Fatal("CLI capture differs")
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal("capture absent")
	}
}
