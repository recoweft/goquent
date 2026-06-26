package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/operation"
)

func runOperation(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printOperationUsage(stderr)
		return 2
	}
	switch args[0] {
	case "compile":
		return runOperationCompile(args[1:], stdout, stderr)
	case "schema":
		return runOperationSchema(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		printOperationUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown operation command %q\n", args[0])
		printOperationUsage(stderr)
		return 2
	}
}

func runOperationCompile(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("goquent operation compile", flag.ContinueOnError)
	fs.SetOutput(stderr)
	manifestPath := fs.String("manifest", "", "manifest JSON path")
	specPath := fs.String("spec", "", "OperationSpec JSON path")
	valuesPath := fs.String("values", "", "JSON map used to resolve value_ref entries")
	format := fs.String("format", "pretty", "output format: pretty, json")
	requireFresh := fs.Bool("require-fresh-manifest", false, "reject stale manifests")
	accessReason := fs.String("access-reason", "", "access reason for PII fields")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: goquent operation compile --manifest goquent.manifest.json --spec operation.json [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	outputFormat := strings.ToLower(strings.TrimSpace(*format))
	switch outputFormat {
	case "", "pretty", "text", "json":
	default:
		fmt.Fprintf(stderr, "unknown operation format %q\n", *format)
		return 2
	}
	if strings.TrimSpace(*manifestPath) == "" || strings.TrimSpace(*specPath) == "" {
		fmt.Fprintln(stderr, "goquent operation compile requires --manifest and --spec")
		return 2
	}
	m, err := manifest.Load(*manifestPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	spec, err := loadOperationSpec(*specPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	values, err := loadOperationValues(*valuesPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	plan, err := operation.Compile(context.Background(), spec, operation.Options{
		Manifest:             m,
		Values:               values,
		RequireFreshManifest: *requireFresh,
		AccessReason:         *accessReason,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if outputFormat == "json" {
		b, err := plan.ToJSON()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if _, err := stdout.Write(append(b, '\n')); err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		return 0
	}
	if _, err := fmt.Fprintln(stdout, plan.String()); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func runOperationSchema(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("goquent operation schema", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	b, err := operation.JSONSchema()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if _, err := stdout.Write(append(b, '\n')); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func loadOperationSpec(path string) (operation.OperationSpec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return operation.OperationSpec{}, err
	}
	var spec operation.OperationSpec
	if err := json.Unmarshal(b, &spec); err != nil {
		return operation.OperationSpec{}, err
	}
	return spec, nil
}

func loadOperationValues(path string) (map[string]any, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var values map[string]any
	if err := json.Unmarshal(b, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func printOperationUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: goquent operation <command>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  compile   compile a read-only OperationSpec to QueryPlan")
	fmt.Fprintln(w, "  schema    print the OperationSpec JSON Schema")
}
