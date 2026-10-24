package review

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/query"
)

// diagnosticDecision mirrors the ordered RunCI display branches only for
// attribution. Every observed item must agree with the actual branch outcome.
// Causes below are test-local vocabulary, never public reasons or CI inputs.
func diagnosticDecision(f ciFinding, prior []ciFinding, contextOK, selectionValid, sameContext, matched bool) (string, string) {
	state, cause := "unknown", "diagnostic_changed"
	switch {
	case !contextOK || !selectionValid:
		cause = "input_correspondence"
	case f.file == "":
		cause = "missing_file"
	default:
		switch {
		case len(prior) == 0 && f.known:
			state = "new"
		case len(prior) == 1 && worsened(prior[0], f):
			state = "worsened"
		case len(prior) == 1 && f.known && prior[0].known && sameDiagnostic(prior[0], f) && sameContext:
			state = "unchanged"
		case len(prior) == 1 && f.known && prior[0].known && !bytes.Equal(prior[0].source, f.source):
			state = "new"
		case len(prior) > 1:
			cause = "ambiguous_prior"
		case !f.known:
			cause = "head_identity_unknown"
		case len(prior) == 1 && !prior[0].known:
			cause = "prior_identity_unknown"
		case len(prior) == 1 && sameDiagnostic(prior[0], f) && !sameContext:
			cause = "whole_tree_context"
		}
	}
	if matched {
		state = "matched"
	}
	if state != "unknown" {
		cause = ""
	}
	return state, cause
}

type comparisonCensus struct {
	States     map[string]int
	Primary    map[string]int
	Overlap    map[string]int
	Population int
}

func censusTrace(t *testing.T, trace ciComparisonTrace, result CIResult, ci CIOptions) comparisonCensus {
	t.Helper()
	out := comparisonCensus{map[string]int{}, map[string]int{}, map[string]int{}, len(trace.head)}
	prior := map[string][]ciFinding{}
	for _, f := range trace.base {
		prior[f.key()] = append(prior[f.key()], f)
	}
	_, matched := applyBaseline(ci.Baseline, ci.Now, trace.base, trace.head, trace.sameContext)
	if len(trace.states) != len(trace.head) {
		t.Fatal("trace population mismatch")
	}
	for i, f := range trace.head {
		previous := prior[f.key()]
		state, cause := diagnosticDecision(f, previous, trace.contextOK, trace.selectionValid, trace.sameContext, matched[i])
		if state != trace.states[i] {
			t.Fatal("diagnostic branch parity failed")
		}
		out.States[state]++
		if state != "unknown" {
			continue
		}
		out.Primary[cause]++
		conditions := map[string]bool{
			"input_correspondence":  !trace.contextOK || !trace.selectionValid,
			"whole_tree_changed":    !equalTrees(trace.baseTree, trace.headTree),
			"head_identity_unknown": !f.known,
			"operation_missing":     f.operation == "",
			"unsupported":           f.f.AnalysisPrecision == query.AnalysisUnsupported,
			"ambiguous_prior":       len(previous) > 1,
			"missing_file":          f.file == "",
			"no_prior":              len(previous) == 0,
		}
		if len(previous) == 1 {
			conditions["prior_identity_unknown"] = !previous[0].known
			conditions["same_diagnostic"] = sameDiagnostic(previous[0], f)
		}
		for k, v := range conditions {
			if v {
				out.Overlap[k]++
			}
		}
	}
	counts := FindingCounts{out.States["matched"], out.States["new"], out.States["worsened"], out.States["unknown"], out.States["unchanged"]}
	total := 0
	for _, n := range out.Primary {
		total += n
	}
	if counts != result.findings || total != counts.Unknown || out.Population != len(result.report.Findings)+len(result.report.SuppressedFindings) {
		t.Fatal("census total mismatch")
	}
	return out
}

func observedCI(t *testing.T, opts Options, ci CIOptions) (CIResult, ciComparisonTrace) {
	t.Helper()
	var trace ciComparisonTrace
	called := false
	result, err := runCI(opts, ci, func(v ciComparisonTrace) { trace = v; called = true })
	if err != nil || !called {
		t.Fatal("comparison trace unavailable")
	}
	return result, trace
}

// A real same-context control, legitimate context edit and unlisted edit check
// the mirror against RunCI, full gates and operation retention together.
func TestComparisonCensusContextControls(t *testing.T) {
	base, head := ciFixture(t)
	ci := CIOptions{Root: head, BaseRoot: base, Diff: []byte{}, FailOn: query.RiskMedium, FailOnPrecision: query.AnalysisPartial}
	for _, mode := range []string{"same", "listed", "unlisted"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "listed" {
				if os.WriteFile(filepath.Join(head, "context.txt"), []byte("context\n"), 0600) != nil {
					t.Fatal("fixture write")
				}
				ci.Diff = changedFixturePatch(nil, map[string]string{"context.txt": "context\n"})
			}
			if mode == "unlisted" {
				ci.Diff = []byte{}
			}
			r, tr := observedCI(t, Options{}, ci)
			census := censusTrace(t, tr, r, ci)
			if !r.risk || !r.precision || len(r.selected.privateOperations) != len(r.report.privateOperations) {
				t.Fatal("full gates or operations lost")
			}
			switch mode {
			case "same":
				if r.invalid || census.States["unchanged"] == 0 || census.Primary["whole_tree_context"] != 0 {
					t.Fatal("same context control")
				}
			case "listed":
				if r.invalid || census.Primary["whole_tree_context"] == 0 || census.States["unchanged"] != 0 {
					t.Fatal("listed context control")
				}
			case "unlisted":
				if !r.invalid || census.Primary["input_correspondence"] != census.Population {
					t.Fatal("unlisted input control")
				}
			}
			plain, err := RunCI(Options{}, ci)
			if err != nil || plain.findings != r.findings || plain.ExitCode() != r.ExitCode() {
				t.Fatal("observer changed result")
			}
		})
	}
}

// Local opt-in collection uses the same implementation as the CLI. Source
// reports, captures, keys and bytes are retained only in protected local files.
func TestLocalComparisonCensus(t *testing.T) {
	output := os.Getenv("GOQUENT_COMPARISON_OUTPUT")
	if output == "" {
		t.Skip("local diagnostic not selected")
	}
	head, base, patch := os.Getenv("GOQUENT_COMPARISON_HEAD"), os.Getenv("GOQUENT_COMPARISON_BASE"), os.Getenv("GOQUENT_COMPARISON_DIFF")
	for _, p := range []string{output, head, base, patch} {
		if !filepath.IsAbs(p) {
			t.Fatal("absolute local inputs required")
		}
	}
	resolved, err := filepath.EvalSymlinks(output)
	if err != nil || resolved != output {
		t.Fatal("protected output required")
	}
	st, err := os.Lstat(output)
	if err != nil || !st.IsDir() || st.Mode().Perm() != 0700 {
		t.Fatal("protected output required")
	}
	for _, root := range []string{head, base, moduleRoot(".")} {
		rel, err := filepath.Rel(root, output)
		if err != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			t.Fatal("output must be outside analyzed roots")
		}
	}
	write := func(name string, v any) {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal("private encoding failed")
		}
		f, err := os.OpenFile(filepath.Join(output, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal("private output unavailable")
		}
		_, err = f.Write(b)
		ce := f.Close()
		if err != nil || ce != nil {
			t.Fatal("private output failed")
		}
	}
	diff, err := ReadCIInput(patch, MaxDiffBytes)
	if err != nil || len(diff) == 0 {
		t.Fatal("nonempty diff required")
	}
	ci := CIOptions{Root: head, BaseRoot: base, Diff: diff, FailOn: query.RiskHigh, FailOnPrecision: query.AnalysisPartial}
	r, tr := observedCI(t, Options{}, ci)
	census := censusTrace(t, tr, r, ci)
	write("census.json", census)
	write("head-report.json", tr.headReport)
	write("base-report.json", tr.baseReport)
	write("head-tree.json", tr.headTree)
	write("base-tree.json", tr.baseTree)
	for name, c := range map[string]*ciCapture{"head": tr.headCapture, "base": tr.baseCapture} {
		write(name+"-capture.json", struct {
			Inputs         map[string][]byte
			Valid          bool
			UnusedTestMain map[string]bool
		}{c.inputs, c.valid, c.unusedTestMain})
	}
	type item struct {
		Index                int
		Key, File, Operation string
		Known                bool
		Source               []byte
		Start, End           int
		State                string
	}
	items := []item{}
	for i, f := range tr.head {
		items = append(items, item{i, f.key(), f.file, f.operation, f.known, f.source, f.startLine, f.endLine, tr.states[i]})
	}
	write("finding-identities.json", items)
	write("diff.json", diff)
	write("gates.json", struct {
		InputFailed, RiskFailed, PrecisionFailed, ContextOK, SelectionValid, SameContext bool
		Exit                                                                             int
	}{r.invalid, r.risk, r.precision, tr.contextOK, tr.selectionValid, tr.sameContext, r.ExitCode()})
	// Full ordinary analysis is a separate axis; compare only complete private
	// findings/operation counts, never use the public projection as input.
	ordinary, ot := observedCI(t, Options{Paths: []string{head}}, CIOptions{Root: head, FailOn: query.RiskHigh})
	write("ordinary-report.json", ot.headReport)
	write("ordinary-capture.json", ot.headCapture.inputs)
	if !reflect.DeepEqual(ordinary.report.Findings, r.report.Findings) || ordinary.report.OperationCount != r.report.OperationCount {
		t.Fatal("ordinary head population differs")
	}
	t.Logf("population=%d unknown=%d new=%d worsened=%d unchanged=%d matched=%d input_failed=%t exit=%d", census.Population, r.findings.Unknown, r.findings.New, r.findings.Worsened, r.findings.Unchanged, r.findings.Matched, r.invalid, r.ExitCode())
}

func TestComparisonCensusUnknownIdentityAndAmbiguity(t *testing.T) {
	for _, mode := range []string{"unsupported", "ambiguous"} {
		t.Run(mode, func(t *testing.T) {
			base, head := ciFixture(t)
			opts := Options{}
			if mode == "unsupported" {
				for _, root := range []string{base, head} {
					data, err := os.ReadFile(filepath.Join(root, "sample.go"))
					if err != nil {
						t.Fatal("fixture read")
					}
					data = append(data, []byte("func invalid(){ missingSymbol() }\n")...)
					if os.WriteFile(filepath.Join(root, "sample.go"), data, 0600) != nil {
						t.Fatal("fixture write")
					}
				}
			} else {
				opts.ShowSuppressed = true
				opts.ConfigSuppressions = []ConfigSuppression{{Code: query.WarningSelectStarUsed, Reason: "bounded synthetic duplicate control"}}
			}
			ci := CIOptions{Root: head, BaseRoot: base, Diff: []byte{}, FailOn: query.RiskMedium, FailOnPrecision: query.AnalysisPartial}
			r, tr := observedCI(t, opts, ci)
			c := censusTrace(t, tr, r, ci)
			if r.invalid || !r.precision || len(r.selected.privateOperations) != len(r.report.privateOperations) {
				t.Fatal("unknown identity gate control")
			}
			if mode == "unsupported" && (c.Primary["head_identity_unknown"] == 0 || c.Overlap["unsupported"] == 0) {
				t.Fatal("unsupported identity control")
			}
			if mode == "ambiguous" && c.Primary["ambiguous_prior"] == 0 {
				t.Fatal("ambiguous prior control")
			}
		})
	}
}
