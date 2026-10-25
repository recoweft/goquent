package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/recoweft/goquent/internal/inputjson"
	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/review"
)

func TestSecurityInputSourceFileBoundaries(t *testing.T) {
	const limit = 8 << 20
	for _, tc := range []struct {
		name, source string
		load         func(string) (any, error)
	}{
		{"schema", `{"tables":[{"name":"fixture","columns":[{"name":"id","type":"bigint","nullable":false}]}]}`, func(p string) (any, error) { return loadSchema(p) }},
		{"policy", `[{"version":1,"table":"fixture"}]`, func(p string) (any, error) { return loadPolicies(p) }},
		{"wrapped-policy", `{"policies":[{"version":1,"table":"fixture"}]}`, func(p string) (any, error) { return loadPolicies(p) }},
		{"manifest", `{"version":"1","tables":[{"name":"fixture","columns":[{"name":"id","type":"bigint","nullable":false}]}]}`, func(p string) (any, error) { return manifest.Load(p) }},
		{"config", `{"fail_on":"high","show_suppressed":true}`, func(p string) (any, error) { return review.LoadConfig(p) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "input.json")
			write := func(b []byte) {
				t.Helper()
				if os.WriteFile(p, b, 0600) != nil {
					t.Fatal("fixture write failed")
				}
			}
			write([]byte(tc.source))
			want, err := tc.load(p)
			if err != nil {
				t.Fatal("ordinary fixture rejected")
			}
			for _, n := range []int{limit - 1, limit, limit + 1} {
				b := bytes.Repeat([]byte(" "), n)
				copy(b, tc.source)
				write(b)
				got, err := tc.load(p)
				if n > limit {
					if !errors.Is(err, inputjson.ErrInvalid) {
						t.Fatal("oversized source not refused before decode")
					}
				} else if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatal("within-limit typed value changed")
				}
			}
		})
	}
}

func TestSecurityInputCLIRejectsBeforeOutput(t *testing.T) {
	p := filepath.Join(t.TempDir(), "input.json")
	if os.WriteFile(p, bytes.Repeat([]byte(" "), (8<<20)+1), 0600) != nil {
		t.Fatal("fixture write failed")
	}
	for _, args := range [][]string{
		{"manifest", "--schema", p}, {"manifest", "--policy", p},
		{"operation", "compile", "--manifest", p, "--spec", p},
		{"review", "--config", p, p}, {"review", p},
	} {
		var out, errOut bytes.Buffer
		code := run(args, &out, &errOut)
		if code != 2 || out.Len() != 0 || errOut.String() != "PUBLIC_OUTPUT: operation failed; details omitted\n" {
			t.Fatal("source refusal changed public error/exit/output")
		}
	}
}
