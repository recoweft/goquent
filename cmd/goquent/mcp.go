package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/recoweft/goquent/orm/manifest"
	mcpserver "github.com/recoweft/goquent/orm/mcp"
)

func runMCP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("goquent mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	manifestPath := fs.String("manifest", "", "stored manifest JSON path")
	schemaPath := fs.String("schema", "", "schema JSON path used to generate or verify manifest")
	policyPath := fs.String("policy", "", "table policy JSON path used to generate or verify manifest")
	databaseSchemaPath := fs.String("database-schema", "", "database schema JSON path used for database fingerprint")
	var codePaths stringListFlag
	var resources stringListFlag
	var tools stringListFlag
	var prompts stringListFlag
	fs.Var(&codePaths, "code", "generated code file or directory path; may be repeated")
	fs.Var(&resources, "resource", "resource URI or name to expose; may be repeated")
	fs.Var(&tools, "tool", "tool name to expose; may be repeated")
	fs.Var(&prompts, "prompt", "prompt name to expose; may be repeated")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: goquent mcp [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	m, err := loadMCPManifest(*manifestPath, *schemaPath, *policyPath, *databaseSchemaPath, codePaths)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	server := mcpserver.NewServer(mcpserver.Options{
		Manifest:  m,
		Resources: resources,
		Tools:     tools,
		Prompts:   prompts,
	})
	if err := server.Serve(context.Background(), stdin, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func loadMCPManifest(manifestPath, schemaPath, policyPath, databaseSchemaPath string, codePaths []string) (*manifest.Manifest, error) {
	if strings.TrimSpace(manifestPath) == "" {
		return buildManifestFromFlags("", schemaPath, policyPath, databaseSchemaPath, "", codePaths)
	}
	stored, err := manifest.Load(manifestPath)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(schemaPath) == "" && strings.TrimSpace(policyPath) == "" && strings.TrimSpace(databaseSchemaPath) == "" && len(codePaths) == 0 {
		return stored, nil
	}
	current, err := buildManifestFromFlags(stored.Dialect, schemaPath, policyPath, databaseSchemaPath, stored.GeneratorVersion, codePaths)
	if err != nil {
		return nil, err
	}
	verification := manifest.Verify(stored, current, time.Time{})
	return manifest.AttachVerification(stored, verification), nil
}
