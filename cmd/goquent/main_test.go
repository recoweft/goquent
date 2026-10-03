package main

import (
	"bytes"
	"encoding/json"
	"github.com/recoweft/goquent/orm/review"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReviewCommandExitCodesAndJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "query.sql")
	if err := os.WriteFile(path, []byte("SELECT * FROM users"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"review", "--format", "json", "--fail-on", "destructive", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0 below destructive threshold, got %d stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"findings"`)) {
		t.Fatalf("expected JSON review output, got %s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{"review", "--fail-on", "high", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected exit code 1 at high threshold, got %d stderr=%s", code, stderr.String())
	}
}

func TestReviewCommandRejectsBadFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"review", "--format", "sarif"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected config error exit code, got %d", code)
	}
}

func TestReviewCommandConfigSuppressionAndPrecisionGate(t *testing.T) {
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "query.sql")
	if err := os.WriteFile(sqlPath, []byte("SELECT * FROM users"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := map[string]any{
		"fail_on": "high",
		"suppressions": []map[string]any{{
			"code":    "RAW_SQL_USED",
			"path":    sqlPath,
			"reason":  "reviewed SQL fixture",
			"expires": time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339),
		}},
	}
	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "goquent.review.json")
	if err := os.WriteFile(cfgPath, cfgBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"review", "--format", "json", "--config", cfgPath, sqlPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected config suppression to avoid high failure, got %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}

	var report review.ReviewReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.SuppressedFindings) != 1 || report.SuppressedFindings[0].Code != "RAW_SQL_USED" || !report.SuppressedFindings[0].Suppressed {
		t.Fatalf("expected applied raw SQL suppression, got %+v", report)
	}

	goPath := filepath.Join(dir, "dynamic.go")
	if err := os.WriteFile(goPath, []byte(`package sample

func run(q any) {
	q.Update(map[string]any{"name": "x"})
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg = map[string]any{"fail_on_precision": "partial", "fail_on": "blocked"}
	cfgBytes, err = json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, cfgBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	code = run([]string{"review", "--config", cfgPath, goPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected precision gate failure, got %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
}

func TestReviewCommandRequireFreshManifestWithoutManifestFails(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := run([]string{"review", "--require-fresh-manifest", dir}, &stdout, &stderr)
	if code != 3 {
		t.Fatalf("expected manifest freshness exit code 3, got %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
}

func TestReviewCommandExpiredConfigSuppression(t *testing.T) {
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "query.sql")
	if err := os.WriteFile(sqlPath, []byte("SELECT * FROM users"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := review.Config{FailOn: "high", Suppressions: []review.ConfigSuppression{{Code: "RAW_SQL_USED", Path: sqlPath, Reason: "expired SQL fixture exception", Expires: time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC3339)}}}
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "goquent.review.json")
	if err := os.WriteFile(cfgPath, b, 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"review", "--format", "json", "--config", cfgPath, sqlPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit=%d stderr=%s", code, &stderr)
	}
	var report review.ReviewReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, f := range report.Findings {
		codes[f.Code] = true
	}
	if len(report.SuppressedFindings) != 0 || !codes["RAW_SQL_USED"] || !codes["SUPPRESSION_EXPIRED"] {
		t.Fatalf("expected unsuppressed expired finding: %+v", report)
	}
}
