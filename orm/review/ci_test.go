package review

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm/query"
)

func ciFixture(t *testing.T) (string, string) {
	t.Helper()
	a, b := t.TempDir(), t.TempDir()
	for _, dir := range []string{a, b} {
		fixtureModule(t, dir)
		e := os.WriteFile(filepath.Join(dir, "sample.go"), []byte("package fixture\nimport \"github.com/recoweft/goquent/orm\"\nfunc sample(db *orm.DB) { var rows []map[string]any; _ = db.Table(\"PRIVATE_TABLE_CANARY\").Select(\"*\").Get(&rows) }\n"), 0600)
		if e != nil {
			t.Fatal(e)
		}
	}
	return a, b
}
func baselineFor(t *testing.T, r CIResult, root string, now time.Time) []byte {
	t.Helper()
	tree, e := readTree(context.Background(), root)
	if e != nil {
		t.Fatal(e)
	}
	for _, f := range collectCI(r.report, root, tree) {
		if f.f.Code != query.WarningSelectStarUsed {
			continue
		}
		d := f.f.Detail
		entry := BaselineEntry{File: f.file, Line: f.f.Location.Line, Column: f.f.Location.Column, Operation: f.operation, Code: f.f.Code, Risk: string(f.f.Level), Precision: string(f.f.AnalysisPrecision), SQL: string(d.SQL), Columns: string(d.Columns), Bindings: string(d.Bindings), Policy: string(d.Policy), Owner: "PRIVATE_OWNER_CANARY", Reason: "PRIVATE_REASON_CANARY temporary review debt", CreatedAt: now.Add(-time.Hour).Format(time.RFC3339), ExpiresAt: now.Add(time.Hour).Format(time.RFC3339)}
		b, e := json.Marshal(Baseline{BaselineKind, 1, []BaselineEntry{entry}})
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	t.Fatal("missing SELECT_STAR_USED")
	return nil
}
func TestCIBaselineActualSourcesAndFullGate(t *testing.T) {
	base, head := ciFixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	o := Options{Paths: []string{head}}
	ci := CIOptions{Root: head, BaseRoot: base, Diff: []byte{}, Now: now, FailOn: query.RiskMedium, FailOnPrecision: query.AnalysisPartial}
	initial, e := RunCI(o, ci)
	if e != nil {
		t.Fatal(e)
	}
	if initial.ExitCode() != 1 {
		t.Fatalf("initial gate %d %+v", initial.ExitCode(), initial.diff)
	}
	ci.Baseline = baselineFor(t, initial, head, now)
	r, e := RunCI(o, ci)
	if e != nil {
		t.Fatal(e)
	}
	if r.entries.Matched != 1 || r.ExitCode() != 1 || r.findings.Matched != 1 {
		t.Fatalf("match/gate: %+v %+v %d", r.entries, r.findings, r.ExitCode())
	}
	for _, format := range []string{"json", "json-v2", "review-ci-json", "sarif", "github", "pretty"} {
		var b bytes.Buffer
		if r.WriteDisplay(&b, format) != nil {
			t.Fatal(format)
		}
		for _, secret := range []string{"PRIVATE_", head, base, "sample.go", "expires_at"} {
			if strings.Contains(b.String(), secret) {
				t.Fatalf("%s leaked %s", format, secret)
			}
		}
	}
	ci.Now = now.Add(time.Hour)
	r, e = RunCI(o, ci)
	if e != nil || r.entries.Expired != 1 || r.ExitCode() != 2 {
		t.Fatalf("expiry %+v %v", r.entries, e)
	}
	ci.Now = now
	f := filepath.Join(head, "helper.go")
	if os.WriteFile(f, []byte("package fixture\n"), 0600) != nil {
		t.Fatal("write")
	}
	r, e = RunCI(o, ci)
	if e != nil || r.entries.Matched != 0 || r.diff.Unlisted != 1 || r.ExitCode() != 2 {
		t.Fatalf("unlisted %+v %+v %v", r.entries, r.diff, e)
	}
}
func TestBaselineStrictAndClockBoundaries(t *testing.T) {
	now := time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
	e := BaselineEntry{File: "a.go", Line: 1, Column: 1, Operation: "select", Code: query.WarningLimitMissing, Risk: "medium", Precision: "partial", SQL: "known", Columns: "known", Bindings: "known", Policy: "unknown", Owner: "team", Reason: "temporary review debt", CreatedAt: now.Format(time.RFC3339), ExpiresAt: now.Add(time.Hour).Format(time.RFC3339)}
	if e.state(now) != "valid" || e.state(now.Add(time.Hour)) != "expired" || e.state(now.Add(-time.Nanosecond)) != "invalid" {
		t.Fatal("clock boundaries")
	}
	cases := map[string]func(*BaselineEntry){"escape": func(e *BaselineEntry) { e.File = "../a.go" }, "glob": func(e *BaselineEntry) { e.File = "*.go" }, "unknown": func(e *BaselineEntry) { e.Code = "UNKNOWN" }, "blocked": func(e *BaselineEntry) { e.Code = query.WarningDeleteWithoutWhere }, "owner": func(e *BaselineEntry) { e.Owner = "" }, "reason": func(e *BaselineEntry) { e.Reason = "" }, "long": func(e *BaselineEntry) { e.ExpiresAt = now.Add(MaxBaselineAge + time.Second).Format(time.RFC3339) }, "missing": func(e *BaselineEntry) { e.ExpiresAt = "" }, "zone": func(e *BaselineEntry) { e.ExpiresAt = "2026-10-15T01:00:00+00:00" }, "axis": func(e *BaselineEntry) { e.SQL = "" }}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			x := e
			change(&x)
			if x.state(now) != "invalid" {
				t.Fatal(x.state(now))
			}
		})
	}
	valid, _ := json.Marshal(Baseline{BaselineKind, 1, []BaselineEntry{e}})
	for _, b := range [][]byte{[]byte(`{"kind":"goquent.review_baseline","version":1,"entries":[],"extra":1}`), bytes.Replace(valid, []byte(`"version":1`), []byte(`"version":2`), 1), bytes.Replace(valid, []byte(`"kind":`), []byte(`"kind":"x","kind":`), 1), []byte(`{"kind":"goquent.review_baseline","version":1,"entries":null}`), bytes.Repeat([]byte("x"), MaxBaselineBytes+1)} {
		if _, err := DecodeBaseline(b); err == nil {
			t.Fatal("accepted malformed")
		}
	}
	dup, _ := json.Marshal(Baseline{BaselineKind, 1, []BaselineEntry{e, e}})
	c, _ := applyBaseline(dup, now, nil, nil, true)
	if c.Invalid != 2 {
		t.Fatal(c)
	}
	c, _ = applyBaseline(valid, now, nil, nil, true)
	if c.Unused != 1 {
		t.Fatal(c)
	}
}
func TestDiffActualContentAndBoundaries(t *testing.T) {
	old := sourceTree{"a.sql": []byte("one\ntwo\nthree\n")}
	head := sourceTree{"a.sql": []byte("one\nchanged\nthree\n")}
	patch := []byte("diff --git a/a.sql b/a.sql\nindex a..b 100644\n--- a/a.sql\n+++ b/a.sql\n@@ -1,3 +1,3 @@\n one\n-two\n+changed\n three\n")
	r := parseDiff(patch, old, head)
	if !r.valid || !r.files[0].lines[2] || r.files[0].lines[1] {
		t.Fatalf("diff %+v", r)
	}
	for _, b := range [][]byte{bytes.Replace(patch, []byte(" one"), []byte(" bad"), 1), bytes.ReplaceAll(patch, []byte("a.sql"), []byte("../escape")), bytes.Replace(patch, []byte("-1,3"), []byte("-90,3"), 1), bytes.Repeat([]byte("x"), MaxDiffBytes+1), []byte("$(touch /tmp/never-evaluated)")} {
		if parseDiff(b, old, head).valid {
			t.Fatal("accepted invalid patch")
		}
	}
	rename := []byte("diff --git a/a.sql b/b.sql\nsimilarity index 100%\nrename from a.sql\nrename to b.sql\n")
	r = parseDiff(rename, old, sourceTree{"b.sql": old["a.sql"]})
	if !r.valid || r.counts.Renamed != 1 {
		t.Fatal(r)
	}
	deleted := []byte("diff --git a/a.sql b/a.sql\ndeleted file mode 100644\n--- a/a.sql\n+++ /dev/null\n@@ -1,3 +0,0 @@\n-one\n-two\n-three\n")
	r = parseDiff(deleted, old, sourceTree{})
	if !r.valid || r.counts.Deleted != 1 {
		t.Fatal(r)
	}
	binary := []byte("diff --git a/a.sql b/a.sql\nBinary files a/a.sql and b/a.sql differ\n")
	r = parseDiff(binary, old, head)
	if r.valid || r.counts.Binary != 1 || r.counts.Unknown == 0 {
		t.Fatal(r)
	}
}
func TestCISourceBoundaries(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "private.sql")
	os.WriteFile(outside, []byte("PRIVATE"), 0600)
	os.Symlink(outside, filepath.Join(root, "link.sql"))
	if _, e := readTree(context.Background(), root); e == nil {
		t.Fatal("followed symlink")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := readTree(ctx, t.TempDir()); e == nil {
		t.Fatal("ignored cancel")
	}
	for _, p := range []string{"../x", "/x", "a/../x", "a\\x", "*.go", "a\x00b"} {
		if exactPath(p) {
			t.Fatal(p)
		}
	}
}
func TestSARIFCanonicalJSONCorrespondence(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "PRIVATE_FILE_CANARY.sql")
	os.WriteFile(p, []byte("SELECT * FROM PRIVATE_TABLE_CANARY"), 0600)
	r, e := Run(Options{Paths: []string{p}})
	if e != nil {
		t.Fatal(e)
	}
	v, _ := r.PublicView()
	var b bytes.Buffer
	if WriteSARIF(&b, r) != nil {
		t.Fatal("write")
	}
	if strings.Contains(b.String(), "PRIVATE_") || strings.Contains(b.String(), "physicalLocation") {
		t.Fatal("leak")
	}
	var doc struct {
		Version string `json:"version"`
		Runs    []struct {
			Results []struct {
				Rule       string `json:"ruleId"`
				Properties struct {
					Line      int    `json:"line"`
					Column    int    `json:"column"`
					Precision string `json:"precision"`
				} `json:"properties"`
			} `json:"results"`
		} `json:"runs"`
	}
	if json.Unmarshal(b.Bytes(), &doc) != nil || doc.Version != "2.1.0" || len(doc.Runs) != 1 || len(doc.Runs[0].Results) != len(v.Findings) {
		t.Fatal("envelope")
	}
	for i, f := range v.Findings {
		x := doc.Runs[0].Results[i]
		if x.Rule != f.Code || x.Properties.Line != f.Line || x.Properties.Column != f.Column || x.Properties.Precision != f.Precision {
			t.Fatal("projection mismatch")
		}
	}
	r.Findings[0].Location = &query.SourceLocation{File: "https://PRIVATE_CANARY", Line: 999}
	b.Reset()
	WriteSARIF(&b, r)
	if strings.Contains(b.String(), "999") {
		t.Fatal("mutable location adopted")
	}
	untrusted := ReviewReport{Findings: []Finding{{Code: "PRIVATE_CODE", Message: "PRIVATE_MESSAGE", Location: &query.SourceLocation{File: "PRIVATE_URI", Line: 999}}}}
	b.Reset()
	WriteSARIF(&b, untrusted)
	if strings.Contains(b.String(), "PRIVATE_") || strings.Contains(b.String(), "999") {
		t.Fatal("untrusted projection")
	}
}
