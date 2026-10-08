package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/recoweft/goquent/internal/operationtest"
	"github.com/recoweft/goquent/orm/operation"
)

func TestOperationDiagnosticCLIFixtures(t *testing.T) {
	cases, e := operationtest.Load("../../tests/contracts/testdata/operation_diagnostics_v1.json")
	if e != nil {
		t.Fatal(e)
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			dir := t.TempDir()
			m, _ := json.Marshal(c.Manifest())
			for name, b := range map[string][]byte{"manifest": m, "spec": c.Spec, "values": c.Values} {
				if e := os.WriteFile(filepath.Join(dir, name), b, 0600); e != nil {
					t.Fatal(e)
				}
			}
			var expected string
			for _, format := range []string{"json", "pretty"} {
				var out, errout bytes.Buffer
				exit := run([]string{"operation", "compile", "--manifest", filepath.Join(dir, "manifest"), "--spec", filepath.Join(dir, "spec"), "--values", filepath.Join(dir, "values"), "--format", format}, &out, &errout)
				wantExit := 1
				if c.Compiled {
					wantExit = 0
				}
				if exit != wantExit {
					t.Fatalf("exit %d expected %d", exit, wantExit)
				}
				b := out.String()
				if !c.Compiled {
					if out.Len() != 0 {
						t.Fatal("refusal written to stdout")
					}
					b = errout.String()
				}
				b = strings.TrimPrefix(b, "Operation diagnostics (details omitted): ")
				v, e := operation.DecodeDiagnosticView([]byte(b))
				if e != nil {
					t.Fatal(e)
				}
				if format == "json" {
					expected = v.String()
				} else if expected != v.String() {
					t.Fatal("pretty differs")
				}
				if (v.Outcome == "compiled") != c.Compiled {
					t.Fatal("CLI outcome differs")
				}
				if !c.Compiled && (len(v.Diagnostics) == 0 || v.Diagnostics[0].Code != c.Code) {
					t.Fatal("CLI category differs")
				}
				if strings.Contains(b, "diagnostic_items") || strings.Contains(b, "9007199254740993") {
					t.Fatal("source disclosure")
				}
			}
		})
	}
}
