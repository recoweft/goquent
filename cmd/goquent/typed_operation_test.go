package main

import (
	"bytes"
	"github.com/recoweft/goquent/orm/manifest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTypedCLIRefusalsAreFixed(t *testing.T) {
	const secret = "gq07_cli_type_secret"
	dir := t.TempDir()
	mf := filepath.Join(dir, "manifest.json")
	sf := filepath.Join(dir, "spec.json")
	vf := filepath.Join(dir, "values.json")
	writeJSON(t, mf, &manifest.Manifest{Version: manifest.Version, Dialect: "postgres", Tables: []manifest.Table{{Name: secret, Columns: []manifest.Column{{Name: "v", Type: "bigint", TypeSource: "sql", NullableKnown: true}}}}})
	for _, value := range []string{`1.0`, `"` + secret + `"`, `9223372036854775808`, `null`} {
		raw := `{"operation":"select","model":"` + secret + `","select":["v"],"filters":[{"field":"v","op":"=","value":` + value + `}]}`
		if e := os.WriteFile(sf, []byte(raw), 0600); e != nil {
			t.Fatal(e)
		}
		var out, stderr bytes.Buffer
		if code := run([]string{"operation", "compile", "--manifest", mf, "--spec", sf}, &out, &stderr); code != 1 || out.Len() != 0 || strings.Contains(stderr.String(), secret) || !strings.Contains(stderr.String(), "details omitted") {
			t.Fatal("nonfixed rejection")
		}
	}
	for _, raw := range []string{`{"v":1,"V":2}`, `{"v":1,"v":2}`, `{"v":1} {}`, `{"unused":"` + strings.Repeat("x", 1048576) + `"}`} {
		if e := os.WriteFile(vf, []byte(raw), 0600); e != nil {
			t.Fatal(e)
		}
		if _, e := loadOperationValues(vf); e == nil {
			t.Fatal("ambiguous or excessive values accepted")
		}
	}
}
