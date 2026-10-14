package operation

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/manifest"
)

func TestUpdateReturningDiagnosticPositions(t *testing.T) {
	for _, tc := range []struct {
		name, code, section, coverage string
		want                          error
	}{
		{"unknown", "FIELD_UNKNOWN", "returning", "unavailable", ErrUnknownField},
		{"forbidden", "FIELD_FORBIDDEN", "returning", "unavailable", ErrForbiddenField},
		{"pii", "ACCESS_REASON_REQUIRED", "returning", "unavailable", ErrPIIAccessReasonRequired},
		{"unsupported", "TYPE_UNVERIFIED", "returning", "partial", ErrTypeUnverified},
		{"filter", "TYPE_MISMATCH", "filters", "partial", ErrTypeMismatch},
		{"assignment", "ASSIGNMENT_INVALID", "assignments", "partial", ErrInvalidAssignment},
		{"success", "", "", "checked_subset", nil},
	} {
		for _, origin := range []string{"go", "json"} {
			t.Run(tc.name+"/"+origin, func(t *testing.T) {
				s, o := updateFixture("text", "postgres")
				s.Assignments[0].Value = diagnosticCanary
				target := diagnosticCanary + "_column"
				o.Manifest.Tables[0].Columns = append(o.Manifest.Tables[0].Columns, manifest.Column{Name: target, Type: "text", TypeSource: "sql", NullableKnown: true})
				s.Returning = []string{"id", target}
				switch tc.name {
				case "unknown":
					s.Returning[1] += "_missing"
				case "forbidden":
					o.Manifest.Tables[0].Columns[2].Forbidden = true
				case "pii":
					o.Manifest.Tables[0].Columns[2].PII = true
				case "unsupported":
					o = strictFixture(t, o, false, false)
					o.Manifest.Tables[0].Columns[2].Type = "unsupported_type"
				case "filter":
					s.Filters = append(s.Filters, FilterSpec{Field: "id", Op: "=", Value: diagnosticCanary})
				case "assignment":
					s.Assignments = append(s.Assignments, UpdateAssignment{Column: target, State: UpdateState(diagnosticCanary)})
				}
				if origin == "json" {
					b, err := json.Marshal(s)
					if err != nil {
						t.Fatal("source encode failed")
					}
					var decoded UpdateSpec
					if json.Unmarshal(b, &decoded) != nil {
						t.Fatal("source decode failed")
					}
					s = decoded
				}
				p, v, err := CompileUpdateWithDiagnostics(t.Context(), s, o)
				if !errors.Is(err, tc.want) || v.Coverage != tc.coverage {
					t.Fatal("identity or coverage changed")
				}
				recorder := newRecorder(OperationSpec{sourceBytes: s.sourceBytes})
				_, internalErr := prepareUpdate(t.Context(), s, o, recorder, nil, nil)
				if !errors.Is(internalErr, tc.want) {
					t.Fatal("private entry diverged")
				}
				if tc.want != nil {
					if p != nil || v.Outcome != "rejected" {
						t.Fatal("refusal lost")
					}
					first := v.Diagnostics[0]
					loc := first.Location
					if first.Ordinal != 1 || first.Code != "OPERATION_"+tc.code || first.Status != "refused" || loc.Source != "spec" || loc.Section != tc.section || !loc.IndexKnown || loc.Index != 1 || loc.Origin != origin {
						t.Fatal("wrong public refusal location")
					}
					failure, ok := err.(*validationFailure)
					if !ok || !reflect.DeepEqual(failure.diagnostics, recorder.records) {
						t.Fatal("private history diverged")
					}
					last := recorder.records[len(recorder.records)-1]
					if last.Section != tc.section || last.Source != "spec" || last.Index != 1 || !last.IndexKnown || last.Origin != origin || recorder.current.Section != last.Section || recorder.current.Index != last.Index || recorder.current.Code != last.Code {
						t.Fatal("current refusal and history diverged")
					}
				} else if p == nil || v.Outcome != "compiled" {
					t.Fatal("success refused")
				}
				returningChecked := false
				for _, d := range recorder.records {
					if d.Section == "select" {
						t.Fatal("internal SELECT location leaked into update")
					}
					if d.Section == "returning" && d.Code == "OPERATION_CHECK_TYPE" && d.Status == "checked" && d.Index == 0 {
						returningChecked = true
					}
				}
				if tc.coverage != "unavailable" && !returningChecked {
					t.Fatal("completed returning check lost")
				}
				for i, d := range v.Diagnostics {
					if d.Ordinal != i+1 {
						t.Fatal("ordinal changed")
					}
				}
				b, e := v.ToJSON()
				if e != nil {
					t.Fatal("public encode failed")
				}
				next, e := DecodeDiagnosticView(b)
				if e != nil || !reflect.DeepEqual(next, v) {
					t.Fatal("public roundtrip changed")
				}
				output := string(b) + v.String() + fmt.Sprintf("%+v %#v %+v %#v", v, &v, err, err)
				if err != nil {
					output += err.Error()
				}
				if strings.Contains(output, diagnosticCanary) || strings.Contains(output, "unsupported_type") {
					t.Fatal("source disclosed")
				}
			})
		}
	}
}

func TestUpdateDiagnosticVocabulary(t *testing.T) {
	for _, tc := range []struct{ section, member, code string }{
		{"assignments", "column", "OPERATION_ASSIGNMENT_INVALID"},
		{"assignments", "state", "OPERATION_ASSIGNMENT_INVALID"},
		{"assignments", "", "OPERATION_PATCH_EMPTY"},
		{"returning", "", "OPERATION_FIELD_UNKNOWN"},
	} {
		v := DiagnosticView{Kind: DiagnosticViewKind, Version: 1, Outcome: "rejected", Coverage: "unavailable", DiagnosticCount: 1, DetailsOmitted: true, Diagnostics: []DiagnosticEntry{{Code: tc.code, Status: "refused", Location: DiagnosticLocation{Source: "spec", Section: tc.section, Member: tc.member, IndexKnown: true, Index: 1, Origin: "json"}}}}
		b, e := v.ToJSON()
		if e != nil {
			t.Fatal("encode failed")
		}
		next, e := DecodeDiagnosticView(b)
		if e != nil || next.Diagnostics[0].Code != tc.code || next.Diagnostics[0].Location.Section != tc.section {
			t.Fatal("closed update vocabulary lost")
		}
		if tc.member != "" && next.Diagnostics[0].Location.Member != tc.member {
			t.Fatal("member lost")
		}
		next.Diagnostics[0].Code = diagnosticCanary
		next.Diagnostics[0].Location.Section = diagnosticCanary
		next.Diagnostics[0].Location.Member = diagnosticCanary
		b, e = next.ToJSON()
		if e != nil || strings.Contains(string(b), diagnosticCanary) {
			t.Fatal("unknown vocabulary disclosed")
		}
		normalized, e := DecodeDiagnosticView(b)
		if e != nil || normalized.Diagnostics[0].Status != "unknown" || normalized.Diagnostics[0].Location.Section != "unknown" || normalized.Diagnostics[0].Location.Member != "unknown" {
			t.Fatal("unknown normalization changed")
		}
	}
}
