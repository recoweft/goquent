package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/review"
)

// Exercise process exits as well as the public adapters; no database is opened.
func TestDynamicRawCLIProcess(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "goquent")
	build := exec.Command("go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		_ = out
		t.Fatal("CLI build failed")
	}
	if os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture.example/private\ngo 1.25.0\n"), 0600) != nil {
		t.Fatal("module write failed")
	}
	for _, tc := range []struct {
		name, operand string
		high          int
	}{
		{"dynamic", "privateOperand", 0}, {"literal", `"SELECT PRIVATE_SQL_CANARY"`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, "source.go")
			source := "package fixture\nimport \"database/sql\"\nfunc example(db *sql.DB, privateOperand string){ db.Exec(" + tc.operand + ") }\n"
			if os.WriteFile(path, []byte(source), 0600) != nil {
				t.Fatal("source write failed")
			}
			for _, format := range []string{"json", "json-v2", "sarif"} {
				for _, gate := range []struct {
					args []string
					exit int
				}{
					{[]string{"--fail-on", "high"}, tc.high},
					{[]string{"--fail-on", "medium"}, 1},
					{[]string{"--fail-on-precision", "partial"}, 1},
				} {
					args := append([]string{"review", "--format", format}, gate.args...)
					args = append(args, path)
					command := exec.Command(binary, args...)
					var out, errs bytes.Buffer
					command.Stdout = &out
					command.Stderr = &errs
					err := command.Run()
					code := 0
					if err != nil {
						if e, ok := err.(*exec.ExitError); ok {
							code = e.ExitCode()
						} else {
							t.Fatal("CLI unavailable")
						}
					}
					if code != gate.exit || errs.Len() != 0 {
						t.Fatal("CLI gate exit changed")
					}
					for _, secret := range []string{path, "privateOperand", "PRIVATE_SQL_CANARY"} {
						if strings.Contains(out.String(), secret) {
							t.Fatal("CLI private data escaped")
						}
					}
					if tc.name == "dynamic" {
						if !strings.Contains(out.String(), "STATIC_REVIEW_UNSUPPORTED") || !strings.Contains(out.String(), "unsupported") || strings.Contains(out.String(), "type_unavailable") {
							t.Fatal("unsupported finding lost")
						}
						if format != "json" && !strings.Contains(out.String(), "dynamic_value") {
							t.Fatal("operation reason lost")
						}
						if format == "json-v2" {
							v, err := review.DecodeReportViewV2(out.Bytes())
							if err != nil {
								t.Fatal("invalid public output")
							}
							if v.FindingCount != 1 || v.OperationCount != 1 || len(v.Operations) != 1 || v.Operations[0].Precision != "unsupported" || v.Operations[0].Detail.Policy != "unknown" {
								t.Fatal("unsupported count or evidence changed")
							}
						}
					} else if !strings.Contains(out.String(), "RAW_SQL_USED") {
						t.Fatal("literal raw warning lost")
					}
				}
			}
		})
	}
}
