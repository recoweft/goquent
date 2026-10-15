package review

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm/analysis"
	"github.com/recoweft/goquent/orm/query"
)

func TestCIUnicodePhysicalLocation(t *testing.T) {
	_, head := ciFixture(t)
	source := "package fixture\nimport \"github.com/recoweft/goquent/orm\"\n//line PRIVATE_FAKE.go:900\nfunc f(db *orm.DB) { /* 日本語 */ var rows []map[string]any; _ = db.Table(\"PRIVATE_T\").Select(\"*\").Get(&rows) }\n"
	p := filepath.Join(head, "sample.go")
	if os.WriteFile(p, []byte(source), 0600) != nil {
		t.Fatal("write")
	}
	r, e := Run(Options{Paths: []string{head}})
	if e != nil {
		t.Fatal(e)
	}
	v, _ := r.PublicView()
	var b bytes.Buffer
	if WriteSARIF(&b, r) != nil {
		t.Fatal("sarif")
	}
	var doc struct {
		Runs []struct {
			Results []struct {
				RuleID     string `json:"ruleId"`
				Properties struct {
					Line, Column int
					Encoding     string `json:"columnEncoding"`
				}
			}
		}
	}
	if json.Unmarshal(b.Bytes(), &doc) != nil {
		t.Fatal("decode")
	}
	found := false
	line := strings.Split(source, "\n")[3]
	for i, f := range v.Findings {
		if f.Code != query.WarningSelectStarUsed {
			continue
		}
		found = true
		// token.Position uses physical, 1-based byte columns, ignoring //line.
		expected := strings.Index(line, "Get(&rows)") + 1
		if f.Line != 4 || f.Column != expected {
			t.Fatalf("position %d:%d expected 4:%d", f.Line, f.Column, expected)
		}
		s := doc.Runs[0].Results[i]
		if s.RuleID != f.Code || s.Properties.Line != f.Line || s.Properties.Column != f.Column || s.Properties.Encoding != "utf8Bytes" {
			t.Fatal("correspondence")
		}
	}
	if !found {
		t.Fatal("missing diagnostic")
	}
	if strings.Contains(b.String(), "PRIVATE_") {
		t.Fatal("privacy")
	}
}

func TestCIMultilineAndUnlistedGlobalUnknown(t *testing.T) {
	base, head := ciFixture(t)
	before := "package fixture\nimport \"github.com/recoweft/goquent/orm\"\nfunc f(db *orm.DB) { var rows []map[string]any\n _ = db.Table(\"PRIVATE_T\").\n Select(\"*\").\n Limit(1).Get(&rows)\n}\n"
	after := strings.Replace(before, "Limit(1)", "Limit(2)", 1)
	for dir, s := range map[string]string{base: before, head: after} {
		if os.WriteFile(filepath.Join(dir, "sample.go"), []byte(s), 0600) != nil {
			t.Fatal("write")
		}
	}
	diff := []byte("diff --git a/sample.go b/sample.go\n--- a/sample.go\n+++ b/sample.go\n@@ -6 +6 @@\n- Limit(1).Get(&rows)\n+ Limit(2).Get(&rows)\n")
	r, e := RunCI(Options{}, CIOptions{Root: head, BaseRoot: base, Diff: diff, FailOn: query.RiskMedium})
	if e != nil || r.invalid {
		t.Fatalf("comparison %v %s", e, r)
	}
	found := false
	for _, f := range r.selected.Findings {
		if f.Code == query.WarningSelectStarUsed {
			found = true
		}
	}
	if !found {
		t.Fatal("multiline finding lost")
	}
	for _, op := range r.report.privateOperations {
		if op.operation == "select" && (op.startLine != 4 || op.endLine != 6) {
			t.Fatal("operation span")
		}
	}
	// An added file absent from the diff remains in the full analysis and selection.
	if os.WriteFile(filepath.Join(head, "extra.go"), []byte("package fixture\nfunc broken(){ unknown.Exec(\"PRIVATE_SQL\") }\n"), 0600) != nil {
		t.Fatal("write")
	}
	r, e = RunCI(Options{}, CIOptions{Root: head, BaseRoot: base, Diff: diff})
	if e != nil || r.ExitCode() != 2 || r.diff.Unlisted != 1 || r.findings.Unknown == 0 {
		t.Fatalf("unlisted %v %s", e, r)
	}
	if len(r.selected.Findings) != len(r.report.Findings) {
		t.Fatal("unknown dropped")
	}
	v, _ := r.PublicView()
	if v.GitTracking != "not_assessed" || !v.Gate.Full {
		t.Fatal("tracking/gate")
	}
}

func TestCIBaselineChangedContextAndAdversarialInputs(t *testing.T) {
	base, head := ciFixture(t)
	now := time.Now().UTC().Truncate(time.Second)
	ci := CIOptions{Root: head, BaseRoot: base, Diff: []byte{}, Now: now}
	initial, e := RunCI(Options{}, ci)
	if e != nil {
		t.Fatal(e)
	}
	raw := baselineFor(t, initial, head, now)
	var b Baseline
	if json.Unmarshal(raw, &b) != nil {
		t.Fatal("decode")
	}
	// Every field must be explicitly present, including a potentially zero column.
	var object map[string]any
	json.Unmarshal(raw, &object)
	entry := object["entries"].([]any)[0].(map[string]any)
	for key, value := range entry {
		delete(entry, key)
		bad, _ := json.Marshal(object)
		if _, e := DecodeBaseline(bad); e == nil {
			t.Fatalf("missing %s accepted", key)
		}
		entry[key] = value
	}
	for _, code := range []string{query.WarningSelectStarUsed, query.WarningLimitMissing, query.WarningWeakPredicate} {
		if !findingSuppressible(code) || !baselineCode(code) {
			t.Fatal("allowlist")
		}
	}
	for _, code := range []string{query.WarningRawSQLUsed, query.WarningStaticReviewUnsupported, query.WarningUpdateWithoutWhere, query.WarningDeleteWithoutWhere, "EVIDENCE_MISSING"} {
		if baselineCode(code) {
			t.Fatal("broad allowlist")
		}
	}
	// Changed helper context cannot inherit a baseline even when the finding bytes agree.
	for _, root := range []string{base, head} {
		os.WriteFile(filepath.Join(root, "helper.go"), []byte("package fixture\n// old\n"), 0600)
	}
	os.WriteFile(filepath.Join(head, "helper.go"), []byte("package fixture\n// new\n"), 0600)
	ci.Diff = []byte("diff --git a/helper.go b/helper.go\n--- a/helper.go\n+++ b/helper.go\n@@ -2 +2 @@\n-// old\n+// new\n")
	ci.Baseline = raw
	r, e := RunCI(Options{}, ci)
	if e != nil || r.entries.Matched != 0 || r.entries.Unknown != 1 || r.ExitCode() != 2 {
		t.Fatalf("context %v %s", e, r)
	}
	if strings.Contains(fmt.Sprintf("%#v", r), "PRIVATE_") {
		t.Fatal("private formatting")
	}
	// Expired entries remain expired even if they have no target.
	b.Entries[0].ExpiresAt = now.Format(time.RFC3339)
	expired, _ := json.Marshal(b)
	counts, _ := applyBaseline(expired, now, nil, nil, true)
	if counts.Expired != 1 || counts.Matched != 0 {
		t.Fatal(counts)
	}
	b.Entries = make([]BaselineEntry, MaxBaselineEntries+1)
	oversize, _ := json.Marshal(b)
	if _, e := DecodeBaseline(oversize); e == nil {
		t.Fatal("too many entries")
	}
}

func TestCIWorsenedAndStructureComparison(t *testing.T) {
	a := ciFinding{file: "a.go", operation: "select", source: []byte("old"), known: true, f: Finding{Code: query.WarningLimitMissing, Level: query.RiskMedium, AnalysisPrecision: query.AnalysisPartial, Location: &query.SourceLocation{File: "a.go", Line: 1, Column: 1}, Detail: analysis.Detail{SQL: analysis.Known, Columns: analysis.Known, Bindings: analysis.Known, Policy: analysis.Unknown}}}
	for name, change := range map[string]func(*ciFinding){"risk": func(f *ciFinding) { f.f.Level = query.RiskHigh }, "precision": func(f *ciFinding) { f.f.AnalysisPrecision = query.AnalysisUnsupported }, "sql": func(f *ciFinding) { f.f.Detail.SQL = analysis.Partial }, "column": func(f *ciFinding) { f.f.Detail.Columns = analysis.Unknown }, "binding": func(f *ciFinding) { f.f.Detail.Bindings = analysis.Unsupported }} {
		t.Run(name, func(t *testing.T) {
			b := a
			change(&b)
			if !worsened(a, b) || sameDiagnostic(a, b) {
				t.Fatal("worsening absorbed")
			}
		})
	}
	b := a
	b.source = []byte("new")
	if sameDiagnostic(a, b) {
		t.Fatal("structure absorbed")
	}
}

func TestCIDiffNewlineAndReadFailures(t *testing.T) {
	diff := []byte("diff --git a/a.sql b/a.sql\n--- a/a.sql\n+++ b/a.sql\n@@ -1 +1 @@\n-old\n+new\n")
	if parseDiff(diff, sourceTree{"a.sql": []byte("old")}, sourceTree{"a.sql": []byte("new\n")}).valid {
		t.Fatal("newline mismatch accepted")
	}
	r, e := RunCI(Options{}, CIOptions{Root: filepath.Join(t.TempDir(), "missing"), BaseRoot: t.TempDir(), Diff: []byte{}})
	v, _ := r.PublicView()
	if e != nil || r.ExitCode() != 2 || v.Gate.Full || v.Diff.ReadFailed != 1 {
		t.Fatal("read failure classified as complete")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, e = RunCI(Options{Context: ctx}, CIOptions{Root: t.TempDir(), BaseRoot: t.TempDir(), Diff: []byte{}})
	if e != nil || r.ExitCode() != 2 {
		t.Fatal("cancellation")
	}
}

func TestCIViewWireAndFullGateTruncation(t *testing.T) {
	fs := make([]Finding, MaxReportViewFindings+10)
	for i := range fs {
		fs[i] = Finding{Code: query.WarningLimitMissing, Level: query.RiskLow, AnalysisPrecision: query.AnalysisPartial}
	}
	fs[len(fs)-1].Level = query.RiskBlocked
	r := CIResult{report: ReviewReport{Findings: fs}, selected: ReviewReport{Findings: fs}, fullAnalyzed: true}
	r.risk = HasFindingsAtOrAbove(r.report, query.RiskHigh)
	var b bytes.Buffer
	r.WriteJSON(&b)
	v, e := DecodeCIView(b.Bytes())
	if e != nil || !v.Selected.Truncated || !v.Gate.Risk || r.ExitCode() != 1 {
		t.Fatal("full gate truncated")
	}
	if _, e := DecodeReportView(b.Bytes()); e == nil {
		t.Fatal("v1 reader accepted CI")
	}
	if _, e := DecodeReportViewV2(b.Bytes()); e == nil {
		t.Fatal("v2 reader accepted CI")
	}
	for _, bad := range [][]byte{bytes.Replace(b.Bytes(), []byte(`"version":1`), []byte(`"version":2`), 1), bytes.Replace(b.Bytes(), []byte(`"kind":`), []byte(`"unknown":true,"kind":`), 1)} {
		if _, e := DecodeCIView(bad); e == nil {
			t.Fatal("invalid envelope")
		}
	}
	v.BaselineReason = "PRIVATE_REASON"
	v.GitTracking = "PRIVATE_TRACKING"
	if strings.Contains(v.String(), "PRIVATE_") {
		t.Fatal("mutable public strings")
	}
	b.Reset()
	if r.WriteSARIF(&b) != nil {
		t.Fatal("sarif")
	}
	var doc map[string]any
	json.Unmarshal(b.Bytes(), &doc)
	if doc["version"] != "2.1.0" {
		t.Fatal("SARIF version")
	}
	run := doc["runs"].([]any)[0].(map[string]any)
	if len(run["results"].([]any)) != MaxReportViewFindings || run["properties"].(map[string]any)["truncated"] != true {
		t.Fatal("SARIF truncation")
	}
}

func TestCISuppressionRetainedAndNoOpaqueMethods(t *testing.T) {
	dir := t.TempDir()
	// The existing suppression is separate from a baseline annotation.
	p := filepath.Join(dir, "q.sql")
	os.WriteFile(p, []byte("SELECT * FROM PRIVATE_TABLE\n"), 0600)
	r, e := Run(Options{Paths: []string{p}, ConfigSuppressions: []ConfigSuppression{{Code: query.WarningRawSQLUsed, Reason: "reviewed read-only fixture", Expires: time.Now().UTC().Add(time.Hour).Format(time.RFC3339)}}})
	if e != nil || len(r.SuppressedFindings) == 0 {
		t.Fatal("suppression fixture")
	}
	var b bytes.Buffer
	if WriteSARIF(&b, r) != nil || !strings.Contains(b.String(), `"justification":"review_suppression_only"`) || strings.Contains(b.String(), "reviewed read-only fixture") {
		t.Fatal("suppression projection")
	}
	hostile := ReviewReport{Findings: []Finding{{Code: query.WarningRawSQLUsed, Message: "PRIVATE_MESSAGE", Evidence: []query.Evidence{{Key: "PRIVATE_KEY", Value: ciOpaque{}}}, Location: &query.SourceLocation{File: "PRIVATE_URI", Line: 999, Column: 999}}}}
	b.Reset()
	if WriteSARIF(&b, hostile) != nil || strings.Contains(b.String(), "PRIVATE") || strings.Contains(b.String(), "999") {
		t.Fatal("untrusted projection")
	}
	if (CIResult{}).ExitCode() != 2 {
		t.Fatal("zero result is not a completed analysis")
	}
}

type ciOpaque struct{}

func (ciOpaque) String() string               { panic("opaque String called") }
func (ciOpaque) MarshalJSON() ([]byte, error) { panic("opaque MarshalJSON called") }

func TestCIWorsenedOutsideChangedLines(t *testing.T) {
	base, head := ciFixture(t)
	old := `{"rules":{"SELECT_STAR_USED":{"severity":"medium"}}}` + "\n"
	current := strings.Replace(old, "medium", "high", 1)
	for root, s := range map[string]string{base: old, head: current} {
		if os.WriteFile(filepath.Join(root, "review.config"), []byte(s), 0600) != nil {
			t.Fatal("write")
		}
	}
	patch := []byte("diff --git a/review.config b/review.config\n--- a/review.config\n+++ b/review.config\n@@ -1 +1 @@\n-" + strings.TrimSuffix(old, "\n") + "\n+" + strings.TrimSuffix(current, "\n") + "\n")
	cfg, e := LoadConfig(filepath.Join(head, "review.config"))
	if e != nil {
		t.Fatal(e)
	}
	r, e := RunCI(Options{ConfigPath: filepath.Join(head, "review.config"), Rules: cfg.Rules}, CIOptions{Root: head, BaseRoot: base, Diff: patch, FailOn: query.RiskHigh})
	if e != nil || r.invalid || r.findings.Worsened == 0 || !r.risk || r.ExitCode() != 1 {
		t.Fatalf("worsened outside patch %v %s", e, r)
	}
	found := false
	for _, f := range r.selected.Findings {
		if f.Code == query.WarningSelectStarUsed && f.Level == query.RiskHigh {
			found = true
		}
	}
	if !found {
		t.Fatal("worsened outside changed file dropped")
	}
}

func TestCIDiffFileAndHunkBudgets(t *testing.T) {
	old, head := sourceTree{}, sourceTree{}
	var b strings.Builder
	for i := 0; i < MaxDiffFiles; i++ {
		p := fmt.Sprintf("f%d.sql", i)
		old[p] = []byte{}
		head[p] = []byte{}
		fmt.Fprintf(&b, "diff --git a/%s b/%s\nold mode 100644\nnew mode 100755\n", p, p)
	}
	if !parseDiff([]byte(b.String()), old, head).valid {
		t.Fatal("exact file budget")
	}
	old["extra.sql"] = []byte{}
	head["extra.sql"] = []byte{}
	b.WriteString("diff --git a/extra.sql b/extra.sql\n")
	if parseDiff([]byte(b.String()), old, head).valid {
		t.Fatal("file budget exceeded")
	}
	b.Reset()
	b.WriteString("diff --git a/f.sql b/f.sql\n--- a/f.sql\n+++ b/f.sql\n")
	for i := 1; i <= MaxDiffHunks; i++ {
		fmt.Fprintf(&b, "@@ -%d +%d @@\n-a\n+b\n", i, i)
	}
	old = sourceTree{"f.sql": []byte(strings.Repeat("a\n", MaxDiffHunks))}
	head = sourceTree{"f.sql": []byte(strings.Repeat("b\n", MaxDiffHunks))}
	if !parseDiff([]byte(b.String()), old, head).valid {
		t.Fatal("exact hunk budget")
	}
	fmt.Fprintf(&b, "@@ -%d +%d @@\n-a\n+b\n", MaxDiffHunks+1, MaxDiffHunks+1)
	old["f.sql"] = append(old["f.sql"], []byte("a\n")...)
	head["f.sql"] = append(head["f.sql"], []byte("b\n")...)
	if parseDiff([]byte(b.String()), old, head).valid {
		t.Fatal("hunk budget exceeded")
	}
}

func TestCISourceDirectoryBudget(t *testing.T) {
	root := t.TempDir()
	for i := 1; i < maxTypeFiles; i++ {
		if os.Mkdir(filepath.Join(root, fmt.Sprint(i)), 0700) != nil {
			t.Fatal("mkdir")
		}
	}
	if _, e := readTree(context.Background(), root); e != nil {
		t.Fatal("exact directory budget")
	}
	os.Mkdir(filepath.Join(root, "extra"), 0700)
	if _, e := readTree(context.Background(), root); e == nil {
		t.Fatal("directory budget exceeded")
	}
}

func TestCIRenameAmbiguity(t *testing.T) {
	old := sourceTree{"a.sql": []byte("x\n")}
	head := sourceTree{"b.sql": []byte("x\n"), "c.sql": []byte("x\n")}
	for _, s := range []string{"diff --git a/a.sql b/b.sql\n", "diff --git a/a.sql b/b.sql\nrename from a.sql\n", "diff --git a/a.sql b/b.sql\nrename from a.sql\nrename to b.sql\ndiff --git a/a.sql b/c.sql\nrename from a.sql\nrename to c.sql\n"} {
		if parseDiff([]byte(s), old, head).valid {
			t.Fatal("ambiguous rename accepted")
		}
	}
}
