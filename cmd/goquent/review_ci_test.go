package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm/review"
)

func TestReviewCICommandFormatsAndGate(t *testing.T) {
	base, head := t.TempDir(), t.TempDir()
	for _, root := range []string{base, head} {
		if os.WriteFile(filepath.Join(root, "PRIVATE.sql"), []byte("SELECT * FROM PRIVATE_TABLE\n"), 0600) != nil {
			t.Fatal("write")
		}
	}
	patch := filepath.Join(t.TempDir(), "diff")
	os.WriteFile(patch, []byte{}, 0600)
	for _, format := range []string{"json", "json-v2", "sarif", "review-ci-json"} {
		t.Run(format, func(t *testing.T) {
			var out, err bytes.Buffer
			exit := run([]string{"review", "--format", format, "--diff", patch, "--diff-base-root", base, "--fail-on", "high", head}, &out, &err)
			if exit != 1 || err.Len() != 0 {
				t.Fatalf("gate %d %s", exit, err.String())
			}
			if strings.Contains(out.String(), "PRIVATE") || strings.Contains(out.String(), head) {
				t.Fatal("privacy")
			}
			switch format {
			case "json":
				if _, e := review.DecodeReportView(out.Bytes()); e != nil {
					t.Fatal(e)
				}
			case "json-v2":
				if _, e := review.DecodeReportViewV2(out.Bytes()); e != nil {
					t.Fatal(e)
				}
			case "review-ci-json":
				v, e := review.DecodeCIView(out.Bytes())
				if e != nil || !v.Gate.Full || !v.Gate.Risk || v.GitTracking != "not_assessed" {
					t.Fatal("CI envelope")
				}
			case "sarif":
				var v map[string]any
				if json.Unmarshal(out.Bytes(), &v) != nil || v["version"] != "2.1.0" {
					t.Fatal("sarif")
				}
			}
		})
	}
	// Expired/invalid/unused inputs cannot be turned into a successful risk gate.
	now := time.Now().UTC()
	entry := review.BaselineEntry{File: "PRIVATE.sql", Line: 1, Column: 0, Operation: "select", Code: "LIMIT_MISSING", Risk: "medium", Precision: "partial", SQL: "partial", Columns: "unknown", Bindings: "unknown", Policy: "unknown", Owner: "PRIVATE_OWNER", Reason: "PRIVATE_REASON temporary debt", CreatedAt: now.Add(-2 * time.Hour).Format(time.RFC3339), ExpiresAt: now.Add(-time.Hour).Format(time.RFC3339)}
	b, _ := json.Marshal(review.Baseline{Kind: review.BaselineKind, Version: 1, Entries: []review.BaselineEntry{entry}})
	baseline := filepath.Join(t.TempDir(), "baseline")
	os.WriteFile(baseline, b, 0600)
	var out, err bytes.Buffer
	exit := run([]string{"review", "--format", "review-ci-json", "--diff", patch, "--diff-base-root", base, "--baseline", baseline, head}, &out, &err)
	v, e := review.DecodeCIView(out.Bytes())
	if exit != 2 || e != nil || v.Entries.Expired != 1 || !v.Gate.Risk {
		t.Fatalf("expiry %d %v", exit, e)
	}
}

func TestReviewCICommandRejectsIncompleteInputs(t *testing.T) {
	for _, args := range [][]string{{"--diff", "PRIVATE_MISSING"}, {"--diff-base-root", "PRIVATE_MISSING"}, {"--baseline", "PRIVATE_MISSING"}, {"--diff", "PRIVATE_MISSING", "--diff-base-root", "PRIVATE_MISSING"}} {
		var out, err bytes.Buffer
		if run(append([]string{"review"}, args...), &out, &err) != 2 {
			t.Fatal("invalid accepted")
		}
		if strings.Contains(out.String()+err.String(), "PRIVATE_MISSING") {
			t.Fatal("raw input error")
		}
	}
}

func TestReviewCISARIFManifestAndLegacySource(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "source.json")
	os.WriteFile(p, []byte(`{"version":1,"findings":[{"code":"LIMIT_MISSING","level":"high","analysis_precision":"precise","location":{"file":"PRIVATE_URI","line":999}}]}`), 0600)
	for _, format := range []string{"json", "json-v2", "sarif", "review-ci-json"} {
		var out, err bytes.Buffer
		code := run([]string{"review", "--format", format, "--fail-on-precision", "unsupported", p}, &out, &err)
		if code != 1 || strings.Contains(out.String(), "PRIVATE_URI") || strings.Contains(out.String(), "999") {
			t.Fatalf("source boundary %s %d", format, code)
		}
		out.Reset()
		err.Reset()
		code = run([]string{"review", "--format", format, "--require-fresh-manifest", root}, &out, &err)
		if code != 3 {
			t.Fatalf("manifest gate %s %d", format, code)
		}
	}
}

func TestReviewCIChangedGoTestsFormats(t *testing.T) {
	base, head := t.TempDir(), t.TempDir()
	repo, e := filepath.Abs("../..")
	if e != nil {
		t.Fatal("root")
	}
	mod, e := os.ReadFile(filepath.Join(repo, "go.mod"))
	if e != nil {
		t.Fatal("module")
	}
	mod = []byte(strings.Replace(string(mod), "module github.com/recoweft/goquent", "module fixture.example/cli", 1) + "\nrequire github.com/recoweft/goquent v0.0.0\nreplace github.com/recoweft/goquent => " + filepath.ToSlash(repo) + "\n")
	sum, e := os.ReadFile(filepath.Join(repo, "go.sum"))
	if e != nil {
		t.Fatal("sum")
	}
	before := "package fixture\nimport \"testing\"\nfunc TestBefore(t *testing.T) {}\n"
	after := before + "func TestAfter(t *testing.T) {}\n"
	for root, test := range map[string]string{base: before, head: after} {
		for name, b := range map[string][]byte{"go.mod": mod, "go.sum": sum, "sample_test.go": []byte(test), "sample.go": []byte("package fixture\nimport \"github.com/recoweft/goquent/orm\"\nfunc sample(db *orm.DB) { var rows []map[string]any; _ = db.Table(\"PRIVATE_TABLE_CANARY\").Select(\"*\").Get(&rows) }\n")} {
			if os.WriteFile(filepath.Join(root, name), b, 0600) != nil {
				t.Fatal("fixture")
			}
		}
	}
	patch := filepath.Join(t.TempDir(), "changes.patch")
	if os.WriteFile(patch, []byte("diff --git a/sample_test.go b/sample_test.go\n--- a/sample_test.go\n+++ b/sample_test.go\n@@ -3 +3,2 @@\n func TestBefore(t *testing.T) {}\n+func TestAfter(t *testing.T) {}\n"), 0600) != nil {
		t.Fatal("patch")
	}
	for _, format := range []string{"json", "json-v2", "review-ci-json", "sarif"} {
		t.Run(format, func(t *testing.T) {
			var out, errs bytes.Buffer
			code := run([]string{"review", "--format", format, "--fail-on", "medium", "--fail-on-precision", "partial", "--diff", patch, "--diff-base-root", base, head}, &out, &errs)
			if code != 1 || errs.Len() != 0 {
				t.Fatalf("exit=%d", code)
			}
			for _, secret := range []string{"PRIVATE_", base, head, "sample_test.go", "_testmain.go"} {
				if strings.Contains(out.String(), secret) {
					t.Fatal("public canary leak")
				}
			}
			switch format {
			case "json":
				if _, e := review.DecodeReportView(out.Bytes()); e != nil {
					t.Fatal("json")
				}
			case "json-v2":
				if _, e := review.DecodeReportViewV2(out.Bytes()); e != nil {
					t.Fatal("json v2")
				}
			case "review-ci-json":
				v, e := review.DecodeCIView(out.Bytes())
				if e != nil || v.Gate.Input || !v.Gate.Risk || !v.Gate.Precision || v.Diff.Modified != 1 {
					t.Fatal("comparison gates")
				}
			case "sarif":
				var doc map[string]any
				if json.Unmarshal(out.Bytes(), &doc) != nil || doc["version"] != "2.1.0" {
					t.Fatal("sarif")
				}
			}
		})
	}
	// A stale patch must retain exit-2 precedence over the unchanged full gates.
	if os.WriteFile(filepath.Join(head, "sample_test.go"), []byte(after+"// unlisted mutation\n"), 0600) != nil {
		t.Fatal("write")
	}
	var out, errs bytes.Buffer
	code := run([]string{"review", "--format", "review-ci-json", "--fail-on", "medium", "--fail-on-precision", "partial", "--diff", patch, "--diff-base-root", base, head}, &out, &errs)
	v, e := review.DecodeCIView(out.Bytes())
	if code != 2 || e != nil || !v.Gate.Input || !v.Gate.Risk || !v.Gate.Precision {
		t.Fatal("stale patch gates")
	}
}
