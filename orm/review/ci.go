package review

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/recoweft/goquent/orm/internal/publicview"
	"github.com/recoweft/goquent/orm/query"
)

// CIOptions explicitly selects bounded display comparison. Roots and source
// inputs are sensitive. Now is a test clock; the CLI always uses current UTC.
type CIOptions struct {
	Root            string
	BaseRoot        string
	Diff            []byte
	Baseline        []byte
	Now             time.Time
	FailOn          query.RiskLevel
	FailOnPrecision query.AnalysisPrecision
}

// CIResult owns full gate inputs separately from its bounded selected display.
// It cannot be decoded from public JSON or a caller-supplied report.
type CIResult struct {
	report                             ReviewReport
	selected                           ReviewReport
	entries                            EntryCounts
	findings                           FindingCounts
	diff                               DiffCounts
	risk, precision, manifest, invalid bool
	compared                           bool
	fullAnalyzed                       bool
	baselineSource                     []byte
}
type GateView struct {
	Risk      bool `json:"risk_failed"`
	Precision bool `json:"precision_failed"`
	Manifest  bool `json:"manifest_failed"`
	Input     bool `json:"input_failed"`
	Full      bool `json:"full_report"`
}
type CIView struct {
	Kind                       string        `json:"kind"`
	Version                    int           `json:"version"`
	Selected                   ReportViewV2  `json:"selected"`
	FullFindingCount           int           `json:"full_finding_count"`
	FullSuppressedFindingCount int           `json:"full_suppressed_finding_count"`
	Gate                       GateView      `json:"gate"`
	Entries                    EntryCounts   `json:"baseline_entries"`
	Findings                   FindingCounts `json:"head_findings"`
	Diff                       DiffCounts    `json:"diff"`
	Compared                   bool          `json:"compared"`
	DetailsOmitted             bool          `json:"details_omitted"`
	BaselineReason             string        `json:"baseline_reason"`
	GitTracking                string        `json:"git_tracking"`
}

func (r CIResult) ExitCode() int {
	if r.invalid || !r.fullAnalyzed {
		return 2
	}
	if r.manifest {
		return 3
	}
	if r.risk || r.precision {
		return 1
	}
	return 0
}
func (r CIResult) PublicView() (CIView, error) {
	v, e := r.selected.PublicViewV2()
	return CIView{Kind: "goquent.review_ci_view", Version: 1, Selected: v, FullFindingCount: len(r.report.Findings), FullSuppressedFindingCount: len(r.report.SuppressedFindings), Gate: GateView{r.risk, r.precision, r.manifest, r.invalid, r.fullAnalyzed}, Entries: r.entries, Findings: r.findings, Diff: r.diff, Compared: r.compared, DetailsOmitted: true, BaselineReason: "review_annotation_only", GitTracking: "not_assessed"}, e
}

// String and Format use only the public view; private baseline text never reaches display.
func (r CIResult) String() string {
	v, e := r.PublicView()
	if e != nil {
		return publicview.ErrSource.Error()
	}
	b, e := json.Marshal(v)
	if e != nil {
		return publicview.ErrInvalid.Error()
	}
	return string(b)
}
func (r CIResult) Format(s fmt.State, _ rune) { _, _ = s.Write([]byte(r.String())) }

func (r CIResult) WriteJSON(w io.Writer) error {
	v, e := r.PublicView()
	if e != nil {
		return publicview.ErrSource
	}
	b, e := json.Marshal(v)
	if e != nil {
		return publicview.ErrInvalid
	}
	return publicview.Write(w, append(b, '\n'))
}

// WriteDisplay preserves existing v1/v2 output formats without feeding them back
// into the complete private gate. New comparison metadata requires explicit CI JSON.
func (r CIResult) WriteDisplay(w io.Writer, format string) error {
	switch format {
	case "sarif":
		return r.WriteSARIF(w)
	case "review-ci-json":
		return r.WriteJSON(w)
	case "json":
		v, e := r.selected.PublicView()
		if e != nil {
			return e
		}
		return WritePublicJSON(w, v)
	case "json-v2":
		v, e := r.selected.PublicViewV2()
		if e != nil {
			return e
		}
		b, e := v.ToJSON()
		if e != nil {
			return e
		}
		return publicview.Write(w, append(b, '\n'))
	case "github":
		return WriteGitHub(w, r.selected)
	default:
		return WritePretty(w, r.selected)
	}
}

type ciFinding struct {
	f               Finding
	file, operation string
	source          []byte
	startLine       int
	endLine         int
	known           bool
}

func itoa(i int) string { return strconv.Itoa(i) }
func (f ciFinding) key() string {
	if f.f.Location == nil {
		return ""
	}
	return f.file + "\x00" + itoa(f.f.Location.Line) + "\x00" + itoa(f.f.Location.Column) + "\x00" + f.f.Code
}
func collectCI(r ReviewReport, root string, tree sourceTree) []ciFinding {
	fs := append(append([]Finding{}, r.Findings...), r.SuppressedFindings...)
	out := make([]ciFinding, 0, len(fs))
	for _, f := range fs {
		c := ciFinding{f: f}
		if f.Location != nil {
			c.file = relativeFile(root, f.Location.File)
			c.source = tree[c.file]
			c.startLine = f.Location.Line
			c.endLine = f.Location.Line
			c.known = c.file != "" && c.source != nil && validPosition(c.source, f.Location.Line, f.Location.Column)
			for _, op := range r.privateOperations {
				if sameLocation(f.Location, op.Location) {
					c.operation = string(op.operation)
					if op.startLine > 0 {
						c.startLine = min(c.startLine, op.startLine)
					}
					c.endLine = max(c.endLine, op.endLine)
					break
				}
			}
		}
		if c.operation == "" || f.AnalysisPrecision == query.AnalysisUnsupported {
			c.known = false
		}
		out = append(out, c)
	}
	return out
}
func validPosition(b []byte, line, column int) bool {
	lines := textLines(b)
	return line > 0 && line <= len(lines) && column >= 0 && column <= len(lines[line-1])+1
}
func captureValid(c *ciCapture, root string, tree sourceTree) bool {
	if !c.valid {
		return false
	}
	for p, b := range c.inputs {
		rel := relativeFile(root, p)
		if rel == "" {
			current, err := ReadCIInput(p, maxTypeFileBytes)
			if err != nil || !bytes.Equal(current, b) {
				return false
			}
			continue
		}
		if !bytes.Equal(tree[rel], b) {
			return false
		}
	}
	return true
}

// External analysis inputs must be identical in both sessions. Each loader may
// independently prove a native test executable unused; its bytes still undergo
// captureValid and Session.unchanged. This is source correspondence only, not a
// claim about physical databases, module provenance or executable contents.
func equalExternalCaptures(a, b *ciCapture, ar, br string) bool {
	x, y := sourceTree{}, sourceTree{}
	for p, v := range a.inputs {
		if relativeFile(ar, p) == "" && !a.unusedTestMain[p] {
			x[p] = v
		}
	}
	for p, v := range b.inputs {
		if relativeFile(br, p) == "" && !b.unusedTestMain[p] {
			y[p] = v
		}
	}
	return equalTrees(x, y)
}
func mapInput(base, head, p string) (string, bool) {
	if p == "" {
		return "", true
	}
	rel := relativeFile(head, p)
	if rel == "" {
		return "", false
	}
	return filepath.Join(base, filepath.FromSlash(rel)), true
}

// RunCI reuses Run's common diagnostics. It never parses diff text as code or
// starts Git, a shell or a network client. Type loading retains Run's contract.
func RunCI(opts Options, ci CIOptions) (CIResult, error) {
	return runCI(opts, ci, nil)
}

// ciComparisonTrace is an internal observation point for test-only diagnostics.
// Nothing here is a public projection or a reusable comparison input.
type ciComparisonTrace struct {
	headReport, baseReport                 ReviewReport
	headTree, baseTree                     sourceTree
	headCapture, baseCapture               *ciCapture
	head, base                             []ciFinding
	states                                 []string
	contextOK, selectionValid, sameContext bool
}

func runCI(opts Options, ci CIOptions, observe func(ciComparisonTrace)) (CIResult, error) {
	var result CIResult
	if opts.Context == nil {
		opts.Context = context.Background()
	}
	if ci.Now.IsZero() {
		ci.Now = time.Now().UTC()
	}
	if ci.FailOn == "" {
		ci.FailOn = query.RiskHigh
	}
	if publicview.Risk(string(ci.FailOn)) == "unknown" || (ci.FailOnPrecision != "" && ci.FailOnPrecision != query.AnalysisPartial && ci.FailOnPrecision != query.AnalysisUnsupported) {
		return result, errCIInput
	}
	comparison := ci.Diff != nil || ci.Baseline != nil || ci.BaseRoot != ""
	result.compared = comparison
	result.baselineSource = append([]byte(nil), ci.Baseline...)
	if comparison && (ci.Diff == nil || ci.BaseRoot == "" || ci.Root == "") {
		return result, errCIInput
	}
	if len(ci.Diff) > MaxDiffBytes || len(ci.Baseline) > MaxBaselineBytes {
		return result, errCIInput
	}
	root := ci.Root
	if root == "" {
		root = "."
	}
	root, err := filepath.Abs(expandEllipsis(root))
	if err != nil {
		return result, errCIInput
	}
	var headTree, oldTree sourceTree
	contextOK := true
	if comparison {
		headTree, err = readTree(opts.Context, root)
		if err != nil {
			result.invalid = true
			result.diff.ReadFailed = 1
			result.diff.Unknown = 1
			return result, nil
		}
		opts.Paths = []string{root}
	}
	hc := &ciCapture{}
	opts.capture = hc
	result.report, err = Run(opts)
	if err != nil {
		result.invalid = true
		result.diff.ReadFailed++
		contextOK = false
	}
	result.selected = result.report
	result.fullAnalyzed = err == nil
	result.risk = HasFindingsAtOrAbove(result.report, ci.FailOn)
	result.precision = ci.FailOnPrecision != "" && HasFindingsAtOrAbovePrecision(result.report, ci.FailOnPrecision)
	result.manifest = opts.RequireFreshManifest && result.report.ManifestStatus != nil && !result.report.ManifestStatus.Fresh
	if !comparison {
		if observe != nil {
			observe(ciComparisonTrace{headReport: result.report, headCapture: hc})
		}
		return result, nil
	}
	after, e := readTree(opts.Context, root)
	contextOK = contextOK && e == nil && equalTrees(headTree, after) && captureValid(hc, root, headTree)
	base, e := filepath.Abs(ci.BaseRoot)
	if e != nil {
		return result, errCIInput
	}
	oldTree, e = readTree(opts.Context, base)
	oldReport := ReviewReport{}
	bc := &ciCapture{}
	if e == nil {
		bo := opts
		bo.Paths = []string{base}
		bo.capture = bc
		bo.Evidence = nil
		var ok bool
		bo.ConfigPath, ok = mapInput(base, root, opts.ConfigPath)
		contextOK = contextOK && ok
		bo.ManifestPath, ok = mapInput(base, root, opts.ManifestPath)
		contextOK = contextOK && ok
		if bo.ConfigPath != "" {
			cfg, ce := LoadConfig(bo.ConfigPath)
			if ce != nil {
				contextOK = false
			} else {
				bo.Rules = cfg.Rules
				bo.ConfigSuppressions = cfg.Suppressions
			}
		}
		if opts.ManifestInputs || opts.CurrentManifest != nil || opts.Evidence != nil {
			contextOK = false
		}
		oldReport, e = Run(bo)
		if e != nil {
			contextOK = false
		}
		after, e = readTree(opts.Context, base)
		contextOK = contextOK && e == nil && equalTrees(oldTree, after) && captureValid(bc, base, oldTree)
	} else {
		result.diff.ReadFailed++
		contextOK = false
	}
	contextOK = contextOK && equalExternalCaptures(hc, bc, root, base)
	selection := parseDiff(ci.Diff, oldTree, headTree)
	selection.counts.ReadFailed += result.diff.ReadFailed
	result.diff = selection.counts
	if !contextOK {
		result.diff.Unknown++
		selection.valid = false
	}
	if !selection.valid {
		result.invalid = true
	}
	old, head := collectCI(oldReport, base, oldTree), collectCI(result.report, root, headTree)
	sameContext := contextOK && selection.valid && equalTrees(oldTree, headTree)
	var matched []bool
	result.entries, matched = applyBaseline(ci.Baseline, ci.Now, old, head, sameContext)
	if result.entries.Invalid+result.entries.Expired+result.entries.Unused+result.entries.Unknown > 0 {
		result.invalid = true
	}
	byKey := map[string][]ciFinding{}
	for _, f := range old {
		byKey[f.key()] = append(byKey[f.key()], f)
	}
	selected := ReviewReport{Version: 1, canonical: true, Findings: []Finding{}, SuppressedFindings: []Finding{}}
	var states []string
	if observe != nil {
		states = make([]string, len(head))
	}
	for i, f := range head {
		previous := byKey[f.key()]
		state := "unknown"
		if contextOK && selection.valid && f.file != "" {
			switch {
			case len(previous) == 0 && f.known:
				state = "new"
			case len(previous) == 1 && worsened(previous[0], f):
				state = "worsened"
			case len(previous) == 1 && f.known && previous[0].known && sameDiagnostic(previous[0], f) && sameContext:
				state = "unchanged"
			case len(previous) == 1 && f.known && previous[0].known && !bytes.Equal(previous[0].source, f.source):
				state = "new"
			}
		}
		if matched[i] {
			state = "matched"
		}
		if observe != nil {
			states[i] = state
		}
		switch state {
		case "matched":
			result.findings.Matched++
		case "new":
			result.findings.New++
		case "worsened":
			result.findings.Worsened++
		case "unchanged":
			result.findings.Unchanged++
		default:
			result.findings.Unknown++
		}
		keep := state != "unchanged" && state != "matched"
		for _, d := range selection.files {
			if d.new == f.file {
				if d.kind == "renamed" || d.kind == "binary" || d.kind == "added" {
					keep = true
				}
				if f.f.Location != nil {
					for l := range d.lines {
						if l >= f.startLine && l <= f.endLine {
							keep = true
						}
					}
				}
			}
		}
		// Baseline annotations never erase the matched diagnostic.
		keep = keep || matched[i]
		if keep {
			if f.f.Suppressed {
				selected.SuppressedFindings = append(selected.SuppressedFindings, f.f)
			} else {
				selected.Findings = append(selected.Findings, f.f)
			}
		}
	}
	// Preserve full operation coverage: warning-free and global unknown operations
	// cannot disappear merely because the diagnostic display was selected.
	selected.Operations = result.report.privateOperations
	selected.privateOperations = result.report.privateOperations
	selected.OperationCount = result.report.OperationCount
	result.selected = selected
	if observe != nil {
		observe(ciComparisonTrace{result.report, oldReport, headTree, oldTree, hc, bc, head, old, states, contextOK, selection.valid, sameContext})
	}
	return result, nil
}

// ReadCIInput bounds file input before allocation and rejects special files.
func ReadCIInput(p string, limit int) ([]byte, error) {
	st, e := os.Lstat(p)
	if e != nil || !st.Mode().IsRegular() || st.Size() > int64(limit) {
		return nil, errCIInput
	}
	f, e := os.Open(p)
	if e != nil {
		return nil, errCIInput
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if e != nil || len(b) > limit {
		return nil, errCIInput
	}
	if b == nil {
		b = []byte{}
	}
	return b, nil
}

// canonicalDisplay drops locations on untrusted source reports. It never copies
// arbitrary messages, URI claims or opaque metadata to SARIF.
func canonicalDisplay(r ReviewReport) ReviewReport {
	r = r.precisionFindings()
	if r.canonical {
		r.Findings = r.canonicalFindings
		r.SuppressedFindings = r.canonicalSuppressed
		return r
	}
	clear := func(fs []Finding) []Finding {
		out := append([]Finding{}, fs...)
		for i := range out {
			out[i].Location = nil
		}
		return out
	}
	r.Findings = clear(r.Findings)
	r.SuppressedFindings = clear(r.SuppressedFindings)
	r.privateFindings = nil
	r.privateSuppressedFindings = nil
	return r
}

func copyCanonical(src []Finding) []Finding {
	out := append([]Finding{}, src...)
	for i := range out {
		out[i].Location = cloneLocation(out[i].Location)
	}
	return out
}
