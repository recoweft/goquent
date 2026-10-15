package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/analysis"
	"github.com/recoweft/goquent/orm/query"
)

func TestSourceReportLegacyPrecision(t *testing.T) {
	for _, version := range []string{"", `"version":0,`, `"version":1,`} {
		for _, operations := range []string{"", `,"operations":[]`, `,"operations":[{"detail":{"sql":"known","columns":"known","bindings":"known","policy":"known"},"correspondence":{"source":"matched"}}]`} {
			for _, detail := range []string{"", `,"detail":{"sql":"known","columns":"known","bindings":"known","policy":"known"}`, `,"detail":{"sql":"unsupported"}`} {
				for _, precision := range []string{"precise", "unsupported"} {
					t.Run(fmt.Sprintf("v%s/o%d/d%d/%s", version, len(operations), len(detail), precision), func(t *testing.T) {
						input := `{` + version + `"findings":[{"code":"STATIC_REVIEW_PARTIAL","level":"high","message":"SOURCE_REPORT_CANARY","location":{"file":"SOURCE_REPORT_CANARY","line":7,"column":3},"analysis_precision":"` + precision + `"` + detail + `}]` + operations + `}`
						var r ReviewReport
						if err := json.Unmarshal([]byte(input), &r); err != nil {
							t.Fatal(err)
						}
						if len(r.Findings) != 1 || r.Findings[0].AnalysisPrecision != query.AnalysisPrecision(precision) || r.Findings[0].Location.Line != 7 || r.Findings[0].Level != query.RiskHigh {
							t.Fatal("source diagnostic changed")
						}
						expected := "partial"
						if precision == "unsupported" || strings.Contains(detail, "unsupported") {
							expected = "unsupported"
						}
						if !HasFindingsAtOrAbovePrecision(r, query.AnalysisPrecision(expected)) || !HasFindingsAtOrAbove(r, query.RiskHigh) {
							t.Fatal("gate lost diagnostic")
						}
						check := func() {
							t.Helper()
							v, _ := r.PublicView()
							v2, _ := r.PublicViewV2()
							for _, fs := range [][]FindingView{v.Findings, v2.Findings} {
								if len(fs) != 1 || fs[0].Precision != expected || fs[0].Line != 7 || fs[0].Column != 3 || fs[0].Level != "high" {
									t.Fatal("projection lost canonical diagnostic")
								}
							}
							for _, op := range v2.Operations {
								if op.Detail.SQL != "unknown" || op.Correspondence.Source != analysis.Unconfirmed {
									t.Fatal("wire claims restored evidence")
								}
							}
							b, _ := v2.ToJSON()
							if strings.Contains(string(b), "SOURCE_REPORT_CANARY") {
								t.Fatal("source leaked")
							}
						}
						check()
						// Mutable source views cannot re-create decoded precision or source evidence.
						r.Findings[0].AnalysisPrecision = query.AnalysisPrecise
						r.Findings[0].Location.Line = 99
						r.Operations = append(r.Operations, OperationReview{Detail: analysis.Detail{SQL: analysis.Known, Columns: analysis.Known, Bindings: analysis.Known, Policy: analysis.Known}, Correspondence: analysis.Correspondence{Source: analysis.Matched}})
						check()
					})
				}
			}
		}
	}
}

func TestSourceReportSuppressionAndAtomicDecode(t *testing.T) {
	for _, show := range []bool{false, true} {
		f := `{"code":"STATIC_REVIEW_PARTIAL","level":"medium","analysis_precision":"precise","suppressed":true,"suppression":{"code":"STATIC_REVIEW_PARTIAL","reason":"SOURCE_REPORT_CANARY"}}`
		active := `[]`
		if show {
			active = `[` + f + `]`
		}
		var r ReviewReport
		if err := json.Unmarshal([]byte(`{"findings":`+active+`,"suppressed_findings":[`+f+`]}`), &r); err != nil {
			t.Fatal(err)
		}
		if HasFindingsAtOrAbovePrecision(r, query.AnalysisPartial) {
			t.Fatal("legal suppression changed")
		}
		v, _ := r.PublicView()
		v2, _ := r.PublicViewV2()
		if len(v.Findings) != len(r.Findings) || v.SuppressedFindings[0].Precision != "partial" || !v.SuppressedFindings[0].Suppressed || v2.SuppressedFindings[0].Precision != "partial" {
			t.Fatal("suppressed projection promoted")
		}
	}
	var r ReviewReport
	if err := json.Unmarshal([]byte(`{"findings":[{"analysis_precision":"precise"}]}`), &r); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(r)
	beforeView, _ := r.PublicViewV2()
	for _, bad := range []string{`{"version":99}`, `{"version":1,"version":1}`, `{"findings":42}`, `{"findings":[{"detail":{"version":99}}]}`, `{"findings":`} {
		if err := json.Unmarshal([]byte(bad), &r); err == nil {
			t.Fatal("invalid decode accepted")
		}
		after, _ := json.Marshal(r)
		v, _ := r.PublicViewV2()
		if string(before) != string(after) || !reflect.DeepEqual(beforeView, v) || !HasFindingsAtOrAbovePrecision(r, query.AnalysisPartial) {
			t.Fatal("failed decode changed receiver")
		}
	}
	// A legitimate in-process diagnostic is not treated as a serialized claim.
	inProcess := ReviewReport{Findings: []Finding{{AnalysisPrecision: query.AnalysisPrecise}}}
	if HasFindingsAtOrAbovePrecision(inProcess, query.AnalysisPartial) {
		t.Fatal("in-process compatibility changed")
	}
	v, _ := inProcess.PublicView()
	if v.Findings[0].Precision != "precise" {
		t.Fatal("in-process precision changed")
	}
}

func TestSourceReportFileBoundary(t *testing.T) {
	for _, body := range []string{
		`{"findings":[{"analysis_precision":"precise"}]}`,
		`{"version":0,"findings":[],"operations":[]}`,
		`{"version":1,"findings":[],"summary":{"total":0}}`,
		`{"version":99,"findings":[]}`,
		`{"version":1,"version":1,"findings":[]}`,
		`{"findings":[],"findings":[]}`,
		`{"findings":[`, `{"findings":`,
		`{"operations":[{"detail":{"sql":"known","columns":"known","bindings":"known","policy":"known"},"correspondence":{"source":"matched"}}]}`,
		`{"suppressed_findings":[{"code":"DELETE_WITHOUT_WHERE","analysis_precision":"precise","suppressed":true,"location":{"file":"SOURCE_REPORT_CANARY","line":88},"message":"// goquent:suppress STATIC_REVIEW_UNSUPPORTED reason=SOURCE_REPORT_CANARY"}]}`,
		`{"kind":"goquent.report_view","version":1,"findings":[],"summary":{"total":0}}`,
	} {
		t.Run(fmt.Sprint(len(body), body[:min(20, len(body))]), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "report.json")
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			r, err := Run(Options{Paths: []string{path}})
			if err != nil {
				t.Fatal(err)
			}
			if len(r.Findings) != 1 || r.Findings[0].Code != query.WarningStaticReviewUnsupported || r.Findings[0].Location.File != path || r.Findings[0].Location.Line != 1 || r.Findings[0].Suppressed || !HasFindingsAtOrAbovePrecision(r, query.AnalysisUnsupported) {
				t.Fatal("source report disappeared/imported claims")
			}
			v, _ := r.PublicView()
			v2, _ := r.PublicViewV2()
			for _, fs := range [][]FindingView{v.Findings, v2.Findings} {
				if len(fs) != 1 || fs[0].Precision != "unsupported" || fs[0].Line != 1 {
					t.Fatal("view changed marker")
				}
			}
			b, _ := v2.ToJSON()
			if strings.Contains(string(b), "SOURCE_REPORT_CANARY") || strings.Contains(string(b), path) {
				t.Fatal("source leaked")
			}
		})
	}
	for _, body := range []string{`{}`, `{"hello":"findings"}`, `{"nested":{"findings":[]}}`, `{"findings":"ordinary data"}`, `{"operation":"select","model":"users","select":["id"]}`} {
		if sourceReportEnvelope([]byte(body)) {
			t.Fatal("unrelated input classified")
		}
	}
	v, _ := (ReviewReport{}).PublicView()
	b, _ := v.ToJSON()
	if sourceReportEnvelope(b) {
		t.Fatal("public v1 misclassified")
	}
	v2, _ := (ReviewReport{}).PublicViewV2()
	b, _ = v2.ToJSON()
	if sourceReportEnvelope(b) {
		t.Fatal("public v2 misclassified")
	}
}

func TestSourceReportFileConfigExpiry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, []byte(`{"findings":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, expiry := range []string{"2099-01-01", "2000-01-01"} {
		for _, show := range []bool{false, true} {
			r, err := Run(Options{Paths: []string{path}, ShowSuppressed: show, ConfigSuppressions: []ConfigSuppression{{Code: query.WarningStaticReviewUnsupported, Path: path, Reason: "bounded fixture suppression for regression testing", Owner: "test", Expires: expiry}}})
			if err != nil {
				t.Fatal(err)
			}
			if expiry == "2099-01-01" {
				if len(r.SuppressedFindings) != 1 || HasFindingsAtOrAbovePrecision(r, query.AnalysisUnsupported) {
					t.Fatal("config suppression not retained")
				}
				if show && len(r.Findings) != 1 {
					t.Fatalf("ShowSuppressed count %d codes %v", len(r.Findings), func() []string {
						var codes []string
						for _, f := range r.Findings {
							codes = append(codes, f.Code)
						}
						return codes
					}())
				}
			} else if !hasFinding(r.Findings, query.WarningSuppressionExpired) || !HasFindingsAtOrAbovePrecision(r, query.AnalysisUnsupported) {
				t.Fatal("expiry bypass")
			}
		}
	}
}

func TestSourceReportBoundedEnvelope(t *testing.T) {
	for _, body := range []string{
		`{"Findings":[]}`, `{"findings":null}`,
		`{"findings":[],"nested":` + strings.Repeat(`[`, 65),
		`{"findings":[],"nested":[` + strings.Repeat(`0,`, 65536),
		`{"findings":[],"message":"` + strings.Repeat("x", maxTypeFileBytes),
	} {
		if !sourceReportEnvelope([]byte(body)) {
			t.Fatal("identified source report lost on damaged/bounded input")
		}
	}
	for _, body := range []string{
		`{"nested":` + strings.Repeat(`[`, 65),
		`{"message":"` + strings.Repeat("x", maxTypeFileBytes),
	} {
		if sourceReportEnvelope([]byte(body)) {
			t.Fatal("unidentified input invented a report")
		}
	}
}

func TestSourceReportMixedQueryPlanVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, []byte(`{"version":99,"operation":"select","sql":"SELECT 1","findings":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := Run(Options{Paths: []string{path}})
	if !errors.Is(err, query.ErrJSONVersion) || !hasFinding(r.Findings, query.WarningStaticReviewUnsupported) {
		t.Fatal("mixed envelope weakened existing version error")
	}
}

func TestDecodedUnsupportedOperationAndEmptySnapshot(t *testing.T) {
	for _, body := range []string{`{"operations":[{"detail":{"bindings":"unsupported"}}]}`, `{"findings":[]}`} {
		var r ReviewReport
		if err := json.Unmarshal([]byte(body), &r); err != nil {
			t.Fatal(err)
		}
		v, _ := r.PublicViewV2()
		if len(v.Operations) > 0 && (v.Operations[0].Precision != "unsupported" || !HasFindingsAtOrAbovePrecision(r, query.AnalysisUnsupported)) {
			t.Fatal("unsupported operation weakened")
		}
		originalCount := len(v.Operations)
		r.Operations = append(r.Operations, OperationReview{Detail: analysis.Detail{SQL: analysis.Known, Columns: analysis.Known, Bindings: analysis.Known, Policy: analysis.Known}, Correspondence: analysis.Correspondence{Source: analysis.Matched}})
		v, _ = r.PublicViewV2()
		if len(v.Operations) != originalCount {
			t.Fatal("mutable decoded view created evidence")
		}
	}
}

func TestSourceReportMixedPlanRetainsRisk(t *testing.T) {
	for _, body := range []string{
		`{"operation":"delete","sql":"DELETE FROM users","findings":[]}`,
		`{"sql":"DROP TABLE users","findings":[]}`,
	} {
		path := filepath.Join(t.TempDir(), "mixed.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		r, err := Run(Options{Paths: []string{path}})
		if err != nil {
			t.Fatal(err)
		}
		if !hasFinding(r.Findings, query.WarningStaticReviewUnsupported) || !HasFindingsAtOrAbove(r, query.RiskHigh) {
			t.Fatal("mixed report hid existing plan risk")
		}
	}
}
