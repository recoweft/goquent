package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewUnverifiedJSONGateAndPrivacy(t *testing.T) {
	for _, body := range []string{
		`{"operation":"insert","sql":"INSERT INTO CANARY_CLI_924 VALUES (1)","analysis_precision":"precise","risk_level":"low","warnings":[]}`,
		`{"operation":"update","sql":42,"metadata":{"error":"CANARY_CLI_924"}}`,
	} {
		path := filepath.Join(t.TempDir(), "CANARY_CLI_924.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		var out, errout bytes.Buffer
		code := run([]string{"review", "--format", "json", "--fail-on", "blocked", "--fail-on-precision", "partial", path}, &out, &errout)
		if code != 1 {
			t.Fatalf("expected precision failure, got %d", code)
		}
		if strings.Contains(out.String()+errout.String(), "CANARY") {
			t.Fatal("private content escaped")
		}
		if !strings.Contains(out.String(), "STATIC_REVIEW_") {
			t.Fatal("missing public marker")
		}
	}
}
