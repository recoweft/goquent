package review

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/query"
)

const reportCanary = "fictional-view@example.invalid gq_fake_token_06 gq_fake_password_06 gq_fake_person_06"

type reportBomb struct{}

func (reportBomb) String() string               { panic("unexpected Stringer") }
func (reportBomb) MarshalJSON() ([]byte, error) { panic("unexpected Marshaler") }

type failingPublicWriter struct{ short bool }

func (w failingPublicWriter) Write(b []byte) (int, error) {
	if w.short {
		return len(b) - 1, nil
	}
	return 0, errors.New(reportCanary)
}
func assertReportSafe(t *testing.T, ss ...string) {
	t.Helper()
	for _, s := range ss {
		for _, c := range strings.Fields(reportCanary) {
			if strings.Contains(s, c) {
				t.Fatal("public report leaked canary")
			}
		}
	}
}
func exerciseReportView(t *testing.T, v ReportView) {
	t.Helper()
	b, err := v.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	assertReportSafe(t, string(b), v.String(), fmt.Sprintf("%v %+v %#v %s %q %x", v, v, &v, v, &v, v))
	for _, x := range []any{v, &v, struct{ View ReportView }{v}, []ReportView{v}} {
		b, e := json.Marshal(x)
		if e != nil {
			t.Fatal(e)
		}
		assertReportSafe(t, string(b))
	}
	for _, writer := range []func(io.Writer, ReportView) error{WritePublicJSON, WritePublicPretty, WritePublicGitHub} {
		var out bytes.Buffer
		if err = writer(&out, v); err != nil {
			t.Fatal(err)
		}
		assertReportSafe(t, out.String())
		for _, w := range []io.Writer{nil, failingPublicWriter{}, failingPublicWriter{short: true}} {
			err = writer(w, v)
			if err == nil {
				t.Fatal("writer error missing")
			}
			assertReportSafe(t, err.Error(), fmt.Sprintf("%#v", err))
			if errors.Unwrap(err) != nil {
				t.Fatal("unsafe error retained")
			}
		}
	}
	decoded, err := DecodeReportView(b)
	if err != nil {
		t.Fatal(err)
	}
	assertReportSafe(t, decoded.String())
}
func TestPublicReportViewOmissionAndTampering(t *testing.T) {
	cycle := map[string]any{}
	cycle[reportCanary] = cycle
	f := Finding{Code: reportCanary, Level: query.RiskLevel(reportCanary), Message: reportCanary, Hint: reportCanary, AnalysisPrecision: query.AnalysisPrecision(reportCanary), Location: &query.SourceLocation{File: reportCanary, Line: 5, Column: 2}, Evidence: []query.Evidence{{Key: reportCanary, Value: reportBomb{}}, {Key: "nested_plan", Value: &query.QueryPlan{SQL: reportCanary, Metadata: cycle}}}, Suppression: &query.Suppression{Reason: reportCanary, Owner: reportCanary, Code: reportCanary}}
	r := ReviewReport{Findings: []Finding{f}, SuppressedFindings: []Finding{f}, Summary: ReviewSummary{ByLevel: map[query.RiskLevel]int{query.RiskLevel(reportCanary): 123}}, ManifestStatus: &ManifestStatus{Path: reportCanary, State: reportCanary}}
	v, err := r.PublicView()
	if err != nil {
		t.Fatal(err)
	}
	if v.Findings[0].Code != "unknown" || v.Findings[0].Level != "unknown" || v.Findings[0].Precision != "unknown" {
		t.Fatal("unknown classification")
	}
	exerciseReportView(t, v)
	v.Findings[0].Code = reportCanary
	v.Findings[0].Message = reportCanary
	v.Findings[0].Level = reportCanary
	v.Findings[0].Precision = reportCanary
	v.SuppressedFindings[0] = v.Findings[0]
	exerciseReportView(t, v)
	fview := v.Findings[0]
	b, _ := json.Marshal(&fview)
	assertReportSafe(t, string(b), fmt.Sprintf("%v %#v %s", fview, &fview, fview))
	if r.Findings[0].Code != reportCanary || r.ManifestStatus.Path != reportCanary {
		t.Fatal("source changed")
	}
	v.Kind = reportCanary
	for _, writer := range []func(io.Writer, ReportView) error{WritePublicJSON, WritePublicPretty, WritePublicGitHub} {
		var out bytes.Buffer
		err = writer(&out, v)
		if err == nil || out.Len() != 0 {
			t.Fatal("invalid view written")
		}
		assertReportSafe(t, err.Error())
	}
	assertReportSafe(t, v.String(), fmt.Sprintf("%#v", v))
}
func TestPublicReportViewBoundsAndCodes(t *testing.T) {
	for _, n := range []int{0, 255, 256, 257, 10000} {
		r := ReviewReport{Findings: make([]Finding, n), SuppressedFindings: make([]Finding, n)}
		v, e := r.PublicView()
		if e != nil {
			t.Fatal(e)
		}
		if len(v.Findings) != min(n, 256) || len(v.SuppressedFindings) != min(n, 256) || v.FindingCount != n || v.Truncated != (n > 256) {
			t.Fatal("bound")
		}
		exerciseReportView(t, v)
		var out bytes.Buffer
		WritePublicPretty(&out, v)
		if strings.Contains(out.String(), "No findings") {
			t.Fatal("misleading summary")
		}
	}
	for _, code := range []string{WarningSuppressionUnused, "MANIFEST_UNVERIFIED", "OPERATION_SPEC_PII_SELECTED", "MIGRATION_DROP_TABLE", query.WarningRawSQLUsed} {
		v, e := (ReviewReport{Findings: []Finding{{Code: code, Level: query.RiskHigh, AnalysisPrecision: query.AnalysisPartial, Message: reportCanary}}}).PublicView()
		if e != nil {
			t.Fatal(e)
		}
		if v.Findings[0].Code != code || !strings.Contains(v.Findings[0].Message, code) {
			t.Fatal("allowlist lost")
		}
		exerciseReportView(t, v)
	}
	for _, version := range []int{0, 1, -1, 2} {
		_, e := (ReviewReport{Version: version}).PublicView()
		if (e == nil) != (version == 0 || version == 1) {
			t.Fatal("diagnostic version")
		}
	}
}
func TestPublicReportViewWireFixture(t *testing.T) {
	b, e := os.ReadFile("../../tests/contracts/testdata/public_views_v1.json")
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		Valid   []json.RawMessage `json:"report_valid"`
		Invalid []json.RawMessage `json:"report_invalid"`
	}
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	for _, b := range f.Valid {
		v, e := DecodeReportView(b)
		if e != nil {
			t.Fatal(e)
		}
		exerciseReportView(t, v)
	}
	for _, b := range f.Invalid {
		_, e := DecodeReportView(b)
		if e == nil {
			t.Fatal("invalid accepted")
		}
		assertReportSafe(t, e.Error())
	}
	for _, bad := range []string{`{"kind":"goquent.report_view","version":1.0}`, `{"kind":"goquent.report_view","version":1e0}`, `{"kind":"goquent.report_view","version":1,"version":1}`, `{"kind":"goquent.report_view","Version":1}`, `{"kind":"goquent.report_view","version":1} {}`, `{"kind":"goquent.report_view","version":1,"findings":null}`} {
		if _, e = DecodeReportView([]byte(bad)); e == nil {
			t.Fatal("ambiguous accepted")
		}
	}
	v, _ := (ReviewReport{}).PublicView()
	if e = v.UnmarshalJSON([]byte(reportCanary)); e == nil || v.Kind != "" {
		t.Fatal("receiver not cleared")
	}
	assertReportSafe(t, e.Error())
}
