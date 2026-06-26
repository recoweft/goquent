package main

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm/manifest"
)

func TestMCPCommandInitialize(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	writeJSON(t, manifestPath, manifest.Manifest{
		Version:          manifest.Version,
		GeneratedAt:      time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
		GeneratorVersion: manifest.Generator,
		Tables: []manifest.Table{{
			Name:  "users",
			Model: "User",
		}},
	})

	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize"}` + "\n")
	var stdout, stderr bytes.Buffer
	code := runMCP([]string{"--manifest", manifestPath}, input, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected mcp success, got %d stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"name":"goquent"`)) {
		t.Fatalf("expected initialize response, got %s", stdout.String())
	}
}

func TestMCPCommandToolAllowlist(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "manifest.json")
	writeJSON(t, manifestPath, manifest.Manifest{
		Version:          manifest.Version,
		GeneratedAt:      time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
		GeneratorVersion: manifest.Generator,
		Tables: []manifest.Table{{
			Name:  "users",
			Model: "User",
		}},
	})

	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	var stdout, stderr bytes.Buffer
	code := runMCP([]string{"--manifest", manifestPath, "--tool", "get_manifest"}, input, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected mcp success, got %d stderr=%s", code, stderr.String())
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"name":"get_manifest"`)) {
		t.Fatalf("expected get_manifest tool, got %s", stdout.String())
	}
	if bytes.Contains(stdout.Bytes(), []byte(`"name":"review_query"`)) {
		t.Fatalf("did not expect review_query tool, got %s", stdout.String())
	}
}
