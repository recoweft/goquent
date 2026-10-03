package review

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm/query"
)

func TestGoSuppressionComments(t *testing.T) {
	tests := []struct {
		name, source string
		line         int
		reason       string
		errorText    string
	}{
		{name: "actual regression", source: "package p\nfunc f(db any) {\n db.Exec(\"SELECT * FROM users\")\n _, _, err = ParseInlineSuppression(\"// goquent:suppress LIMIT_MISSING\")\n}"},
		{name: "raw fixture", source: "package p\nvar fixture = `\n// goquent:suppress RAW_SQL_USED reason=\"fixture\"\n`\n"},
		{name: "interpreted string", source: "package p\nvar s = \"// goquent:suppress RAW_SQL_USED reason=\\\"data\\\"\"\n"},
		{name: "line", source: "package p\n// ordinary comment\n// goquent:suppress RAW_SQL_USED reason=\"reviewed SQL\"\n", line: 3, reason: "reviewed SQL"},
		{name: "trailing", source: "package p\nvar n = 1 // goquent:suppress RAW_SQL_USED reason=\"reviewed SQL\"\n", line: 2, reason: "reviewed SQL"},
		{name: "block", source: "package p\n/* goquent:suppress RAW_SQL_USED reason=\"reviewed SQL\"*/\n", line: 2, reason: "reviewed SQL"},
		{name: "multiline block", source: "package p\n/* ordinary text\n * goquent:suppress RAW_SQL_USED reason=\"reviewed SQL\"\n */\n", line: 3, reason: "reviewed SQL"},
		{name: "escaped", source: "package p\n// goquent:suppress RAW_SQL_USED reason=\"reviewed \\\"SQL\\\" at C:\\\\tmp\"\n", line: 2, reason: `reviewed "SQL" at C:\tmp`},
		{name: "physical line", source: "package p\n//line other.go:100\n// goquent:suppress RAW_SQL_USED reason=\"reviewed SQL\"\n", line: 3, reason: "reviewed SQL"},
		{name: "unterminated", source: "package p\n// goquent:suppress RAW_SQL_USED reason=\"oops\n", line: 2, errorText: "unterminated suppression quote"},
		{name: "missing reason", source: "package p\n/* text\n goquent:suppress RAW_SQL_USED\n*/\n", line: 3, errorText: "suppression reason is required"},
		{name: "invalid Go", source: "package p\nfunc {", errorText: "expected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "source.go")
			if err := os.WriteFile(path, []byte(tt.source), 0o644); err != nil {
				t.Fatal(err)
			}
			got, err := suppressionsForFile(path)
			if tt.errorText != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errorText) {
					t.Fatalf("error=%v", err)
				}
				if tt.line != 0 && !strings.Contains(err.Error(), fmt.Sprintf("%s:%d:", path, tt.line)) {
					t.Fatalf("missing physical location: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.line == 0 {
				if len(got) != 0 {
					t.Fatalf("ordinary data suppressed: %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0].Location.File != path || got[0].Location.Line != tt.line || got[0].Reason != tt.reason {
				t.Fatalf("suppressions=%+v", got)
			}
		})
	}
}

func TestRunGoSuppressionRegression(t *testing.T) {
	for _, tt := range []struct {
		name, source string
		suppressed   bool
		wantError    bool
	}{
		{name: "minimal reproduction", source: "package p\nfunc f(db any) {\n db.Exec(\"SELECT * FROM users\")\n _, _, err = ParseInlineSuppression(\"// goquent:suppress LIMIT_MISSING\")\n}"},
		{name: "raw fixture adjacent", source: "package p\nfunc f(db any) {\n s := `// goquent:suppress RAW_SQL_USED reason=\"fixture\"`\n db.Exec(\"SELECT * FROM users\")\n}"},
		{name: "physical finding line", source: "package p\nfunc f(db any) {\n//line elsewhere.go:100\n// goquent:suppress RAW_SQL_USED reason=\"reviewed SQL\"\n db.Exec(\"SELECT * FROM users\")\n}", suppressed: true},
		{name: "previous line", source: "package p\nfunc f(db any) {\n// goquent:suppress RAW_SQL_USED reason=\"reviewed SQL\"\n db.Exec(\"SELECT * FROM users\")\n}", suppressed: true},
		{name: "same line", source: "package p\nfunc f(db any) {\n db.Exec(\"SELECT * FROM users\") // goquent:suppress RAW_SQL_USED reason=\"reviewed SQL\"\n}", suppressed: true},
		{name: "multiline adjacent", source: "package p\nfunc f(db any) {\n/* comment\n goquent:suppress RAW_SQL_USED reason=\"reviewed SQL\" */\n db.Exec(\"SELECT * FROM users\")\n}", suppressed: true},
		{name: "multiline distant", source: "package p\nfunc f(db any) {\n/* goquent:suppress RAW_SQL_USED reason=\"reviewed SQL\"\n ordinary comment\n */\n db.Exec(\"SELECT * FROM users\")\n}"},
		{name: "invalid directive", source: "package p\nfunc f(db any) {\n// goquent:suppress RAW_SQL_USED reason=\"oops\n db.Exec(\"SELECT * FROM users\")\n}", wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "source.go")
			if err := os.WriteFile(path, []byte(tt.source), 0o644); err != nil {
				t.Fatal(err)
			}
			report, err := Run(Options{Paths: []string{path}})
			if tt.wantError {
				if err == nil || !strings.Contains(err.Error(), path+":3:") || !strings.Contains(err.Error(), "unterminated suppression quote") {
					t.Fatalf("error=%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.suppressed {
				if len(report.SuppressedFindings) != 1 || report.SuppressedFindings[0].Code != query.WarningRawSQLUsed || hasFinding(report.Findings, query.WarningRawSQLUsed) {
					t.Fatalf("report=%+v", report)
				}
			} else if len(report.SuppressedFindings) != 0 || !hasFinding(report.Findings, query.WarningRawSQLUsed) {
				t.Fatalf("report=%+v", report)
			}
		})
	}
}

func TestReviewSuppressionExpiryBoundary(t *testing.T) {
	for _, expiry := range []string{"2040-01-02", "2040-01-02T00:00:00Z", "2040-01-02T09:00:00+09:00"} {
		t.Run(expiry, func(t *testing.T) {
			suppressions, err := configSuppressionsForFile("query.sql", []ConfigSuppression{{Code: query.WarningRawSQLUsed, Reason: "reviewed SQL", Expires: expiry}})
			if err != nil {
				t.Fatal(err)
			}
			midnight := time.Date(2040, 1, 2, 0, 0, 0, 0, time.UTC)
			if len(suppressions) != 1 || !suppressions[0].ExpiresAt.Equal(midnight) {
				t.Fatalf("expiry=%+v", suppressions)
			}
			for _, delta := range []time.Duration{-time.Nanosecond, 0, time.Nanosecond} {
				got := applyReviewSuppressions([]Finding{{Code: query.WarningRawSQLUsed, Level: query.RiskHigh}}, suppressions, midnight.Add(delta))
				if delta < 0 {
					if len(got) != 1 || !got[0].Suppressed {
						t.Fatalf("before expiry: %+v", got)
					}
				} else if len(got) != 2 || got[0].Suppressed || got[0].Code != query.WarningRawSQLUsed || got[1].Code != query.WarningSuppressionExpired {
					t.Fatalf("at/after expiry: %+v", got)
				}
			}
		})
	}
}
