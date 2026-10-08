package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/recoweft/goquent/internal/inputjson"
	"io"
	"os"
	"strings"

	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/operation"
	"github.com/recoweft/goquent/orm/publicoutput"
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
		fmt.Fprintln(stderr, "PUBLIC_INPUT: unknown command or format")
		printOperationUsage(stderr)
		return 2
	}
}

func runOperationCompile(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("goquent operation compile", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
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
		fmt.Fprintln(stderr, "PUBLIC_INPUT: invalid command arguments")
		return 2
	}
	outputFormat := strings.ToLower(strings.TrimSpace(*format))
	switch outputFormat {
	case "", "pretty", "text", "json":
	default:
		fmt.Fprintln(stderr, "PUBLIC_INPUT: unknown command or format")
		return 2
	}
	if strings.TrimSpace(*manifestPath) == "" || strings.TrimSpace(*specPath) == "" {
		fmt.Fprintln(stderr, "goquent operation compile requires --manifest and --spec")
		return 2
	}
	m, err := manifest.Load(*manifestPath)
	if err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	spec, specBytes, err := loadOperationSpecSized(*specPath)
	if err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	values, valuesBytes, err := loadOperationValuesSized(*valuesPath)
	if err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	// Raw files share the same wrapper budget, including whitespace, using the
	// bytes actually decoded rather than a later filesystem stat.
	if specBytes+valuesBytes+19 > inputjson.MaxBytes {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 1
	}

	_, view, err := operation.CompileWithDiagnostics(context.Background(), spec, operation.Options{
		Manifest:             m,
		Values:               values,
		RequireFreshManifest: *requireFresh,
		AccessReason:         *accessReason,
	})
	resultCode := 0
	sink := stdout
	if err != nil {
		resultCode = 1
		sink = stderr
	}
	var writeErr error
	if outputFormat == "json" {
		writeErr = operation.WriteDiagnosticJSON(sink, view)
	} else {
		writeErr = operation.WriteDiagnosticPretty(sink, view)
	}
	if writeErr != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	return resultCode
}

func runOperationSchema(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("goquent operation schema", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, "PUBLIC_INPUT: invalid command arguments")
		return 2
	}
	b, err := operation.JSONSchema()
	if err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	if err := publicoutput.Write(stdout, append(b, '\n')); err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	return 0
}

func loadOperationSpec(path string) (operation.OperationSpec, error) {
	v, _, e := loadOperationSpecSized(path)
	return v, e
}
func loadOperationSpecSized(path string) (operation.OperationSpec, int, error) {
	b, err := readOperationInput(path)
	if err != nil {
		return operation.OperationSpec{}, 0, err
	}
	var spec operation.OperationSpec
	if err := json.Unmarshal(b, &spec); err != nil {
		return operation.OperationSpec{}, 0, err
	}
	return spec, len(b), nil
}
func loadOperationValues(path string) (map[string]any, error) {
	v, _, e := loadOperationValuesSized(path)
	return v, e
}
func loadOperationValuesSized(path string) (map[string]any, int, error) {
	if strings.TrimSpace(path) == "" {
		return nil, 4, nil
	}
	b, err := readOperationInput(path)
	if err != nil {
		return nil, 0, err
	}
	if err := inputjson.CheckJSON(b); err != nil {
		return nil, 0, err
	}
	var values map[string]any
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if err := decoder.Decode(&values); err != nil {
		return nil, 0, err
	}
	return values, len(b), nil
}

func printOperationUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: goquent operation <command>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  compile   compile a read-only OperationSpec to QueryPlan")
	fmt.Fprintln(w, "  schema    print the OperationSpec JSON Schema")
}

func readOperationInput(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, inputjson.MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > inputjson.MaxBytes {
		return nil, inputjson.ErrInvalid
	}
	return b, nil
}
