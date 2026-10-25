package review

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/recoweft/goquent/orm/query"
)

// Counters catch fmt's recovery of panicking formatters as well as direct calls.
type outputAuditOpaque struct{ calls *int }

func (x outputAuditOpaque) hit()                         { *x.calls++; panic("unexpected opaque evaluation") }
func (x outputAuditOpaque) String() string               { x.hit(); return "" }
func (x outputAuditOpaque) Error() string                { x.hit(); return "" }
func (x outputAuditOpaque) Format(fmt.State, rune)       { x.hit() }
func (x outputAuditOpaque) MarshalJSON() ([]byte, error) { x.hit(); return nil, nil }
func (x outputAuditOpaque) Value() (driver.Value, error) { x.hit(); return nil, nil }
func (x outputAuditOpaque) Unwrap() error                { x.hit(); return nil }

type outputAuditSink struct {
	cause error
	short bool
}

func (w outputAuditSink) Write(b []byte) (int, error) {
	if w.short {
		return len(b) - 1, nil
	}
	return 0, w.cause
}

func TestSecurityOutputSARIFOpaqueAndErrors(t *testing.T) {
	calls := 0
	x := outputAuditOpaque{&calls}
	cycle := map[string]any{}
	cycle["self"] = cycle
	r := ReviewReport{Version: 1, Findings: []Finding{{
		Code: query.WarningRawSQLUsed, Level: query.RiskHigh,
		AnalysisPrecision: query.AnalysisPartial, Message: reportCanary, Hint: reportCanary,
		Location: &query.SourceLocation{File: reportCanary, Line: 7, Column: 3},
		Evidence: []query.Evidence{{Key: reportCanary, Value: x}, {Value: cycle},
			{Value: &query.QueryPlan{SQL: reportCanary, Params: []any{int16(7), x}, Metadata: map[string]any{"callback": func() { calls++ }}}}},
	}}}
	var out bytes.Buffer
	if WriteSARIF(&out, r) != nil {
		t.Fatal("SARIF failed")
	}
	assertReportSafe(t, out.String())
	var document struct {
		Runs []struct {
			Results []struct {
				RuleID string `json:"ruleId"`
			} `json:"results"`
		} `json:"runs"`
	}
	if json.Unmarshal(out.Bytes(), &document) != nil || len(document.Runs) != 1 || len(document.Runs[0].Results) != 1 || document.Runs[0].Results[0].RuleID != query.WarningRawSQLUsed {
		t.Fatal("fixed diagnostic lost")
	}
	for _, sink := range []io.Writer{outputAuditSink{cause: x}, outputAuditSink{short: true}} {
		err := WriteSARIF(sink, r)
		if err == nil || errors.Unwrap(err) != nil || errors.Is(err, x) {
			t.Fatal("source error retained or failure ignored")
		}
		assertReportSafe(t, fmt.Sprintf("%v %+v %#v", err, err, err))
	}
	if calls != 0 {
		t.Fatal("publication evaluated an opaque method or callback")
	}
	if r.Findings[0].Location.File != reportCanary || r.Findings[0].Evidence[0].Value != x {
		t.Fatal("source mutated")
	}
}
