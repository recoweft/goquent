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
