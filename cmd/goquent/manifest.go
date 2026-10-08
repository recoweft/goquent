package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/migration"
	"github.com/recoweft/goquent/orm/publicoutput"
	"github.com/recoweft/goquent/orm/query"
)

type stringListFlag []string

func (s *stringListFlag) String() string {
	return strings.Join(*s, ",")
}

func (s *stringListFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	*s = append(*s, value)
	return nil
}

func runManifest(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "schema":
			return runManifestSchema(args[1:], stdout, stderr)
		case "verify":
			return runManifestVerify(args[1:], stdout, stderr)
		case "diff":
			return runManifestVerify(args[1:], stdout, stderr)
		case "repository", "repo", "skeleton":
			return runManifestRepository(args[1:], stdout, stderr)
		case "-h", "--help", "help":
			printManifestUsage(stdout)
			return 0
		}
	}
	return runManifestGenerate(args, stdout, stderr)
}

func runManifestGenerate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("goquent manifest", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	localOutput := fs.String("unsafe-local-output", "", "save sensitive internal artifact to a new local file; forbidden in CI")
	format := fs.String("format", "pretty", "output format: pretty, json")
	dialect := fs.String("dialect", "", "SQL dialect name")
	schemaPath := fs.String("schema", "", "schema JSON path")
	policyPath := fs.String("policy", "", "table policy JSON path")
	databaseSchemaPath := fs.String("database-schema", "", "database schema JSON path for database fingerprint")
	generatorVersion := fs.String("generator-version", "", "manifest generator version")
	var codePaths stringListFlag
	fs.Var(&codePaths, "code", "generated code file or directory path; may be repeated")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: goquent manifest [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, "PUBLIC_INPUT: invalid command arguments")
		return 2
	}
	outputFormat, ok := normalizeManifestFormat(*format)
	if !ok {
		fmt.Fprintln(stderr, "PUBLIC_INPUT: unknown command or format")
		return 2
	}
	m, err := buildManifestFromFlags(*dialect, *schemaPath, *policyPath, *databaseSchemaPath, *generatorVersion, codePaths)
	if err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	if *localOutput != "" {
		data, err := m.ToJSON()
		if err == nil {
			err = saveLocalArtifact(*localOutput, data)
		}
		if err != nil {
			fmt.Fprintln(stderr, "PUBLIC_OUTPUT: local export failed")
			return 2
		}
	}
	if err := writeManifestOutput(stdout, outputFormat, m); err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	return 0
}

func runManifestSchema(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("goquent manifest schema", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, "PUBLIC_INPUT: invalid command arguments")
		return 2
	}
	b, err := manifest.JSONSchema()
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

func runManifestRepository(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("goquent manifest repository", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	localOutput := fs.String("unsafe-local-output", "", "save sensitive internal artifact to a new local file; forbidden in CI")
	manifestPath := fs.String("manifest", "", "manifest JSON path")
	tableName := fs.String("table", "", "table name to generate")
	packageName := fs.String("package", "repository", "Go package name for generated code")
	rowType := fs.String("row-type", "", "generated row struct name")
	repositoryType := fs.String("repository-type", "", "generated repository type name")
	typed := fs.Bool("typed", false, "generate typed repository reads")
	var projections stringListFlag
	fs.Var(&projections, "projection", "named projection Name=column1,column2; may be repeated")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: goquent manifest repository --manifest goquent.manifest.json --table users [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, "PUBLIC_INPUT: invalid command arguments")
		return 2
	}
	if strings.TrimSpace(*manifestPath) == "" {
		fmt.Fprintln(stderr, "goquent manifest repository requires --manifest")
		return 2
	}
	m, err := manifest.Load(*manifestPath)
	if err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	var projectionOptions []manifest.RepositoryProjection
	for _, raw := range projections {
		name, cols, ok := strings.Cut(raw, "=")
		if !ok {
			fmt.Fprintln(stderr, "PUBLIC_INPUT: invalid command arguments")
			return 2
		}
		projectionOptions = append(projectionOptions, manifest.RepositoryProjection{Name: name, Columns: strings.Split(cols, ",")})
	}
	if !*typed && len(projectionOptions) > 0 {
		fmt.Fprintln(stderr, "PUBLIC_INPUT: invalid command arguments")
		return 2
	}
	src, err := manifest.GenerateRepositorySkeleton(m, manifest.RepositorySkeletonOptions{
		PackageName:        *packageName,
		TableName:          *tableName,
		RowTypeName:        *rowType,
		RepositoryTypeName: *repositoryType,
		Typed:              *typed, Projections: projectionOptions,
	})
	if err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	if *localOutput != "" {
		if err := saveLocalArtifact(*localOutput, src); err != nil {
			fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
			return 2
		}
	}
	if err := publicoutput.Write(stdout, []byte("PUBLIC_OUTPUT: repository artifact details omitted\n")); err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: output failed")
		return 2
	}
	return 0
}

func runManifestVerify(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("goquent manifest verify", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	format := fs.String("format", "pretty", "output format: pretty, json")
	manifestPath := fs.String("manifest", "", "stored manifest JSON path")
	dialect := fs.String("dialect", "", "SQL dialect name")
	schemaPath := fs.String("schema", "", "current schema JSON path")
	policyPath := fs.String("policy", "", "current table policy JSON path")
	databaseSchemaPath := fs.String("database-schema", "", "current database schema JSON path")
	generatorVersion := fs.String("generator-version", "", "manifest generator version")
	againstDB := fs.Bool("against-db", false, "compare database fingerprint when --database-schema is provided")
	var codePaths stringListFlag
	fs.Var(&codePaths, "code", "current generated code file or directory path; may be repeated")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: goquent manifest verify --manifest goquent.manifest.json [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, "PUBLIC_INPUT: invalid command arguments")
		return 2
	}
	outputFormat, ok := normalizeManifestFormat(*format)
	if !ok {
		fmt.Fprintln(stderr, "PUBLIC_INPUT: unknown command or format")
		return 2
	}
	if strings.TrimSpace(*manifestPath) == "" {
		fmt.Fprintln(stderr, "goquent manifest verify requires --manifest")
		return 2
	}
	databaseSchemaForVerify := ""
	if *againstDB {
		if strings.TrimSpace(*databaseSchemaPath) == "" {
			fmt.Fprintln(stderr, "goquent manifest verify --against-db requires --database-schema")
			return 2
		}
		databaseSchemaForVerify = *databaseSchemaPath
	}
	stored, err := manifest.Load(*manifestPath)
	if err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	current, err := buildManifestFromFlags(*dialect, *schemaPath, *policyPath, databaseSchemaForVerify, *generatorVersion, codePaths)
	if err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	storedForVerify := *stored
	if !*againstDB {
		storedForVerify.DatabaseFingerprint = ""
	}
	verification := manifest.Verify(&storedForVerify, current, time.Time{})
	if outputFormat == "json" {
		err = publicoutput.WriteSummary(stdout, verification.PublicView())
	} else {
		err = manifest.WriteVerificationPretty(stdout, verification)
	}
	if err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	if !verification.Fresh {
		return 1
	}
	return 0
}

func runDoctor(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("goquent doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	manifestPath := fs.String("manifest", "", "stored manifest JSON path")
	schemaPath := fs.String("schema", "", "current schema JSON path")
	policyPath := fs.String("policy", "", "current table policy JSON path")
	databaseSchemaPath := fs.String("database-schema", "", "current database schema JSON path")
	var codePaths stringListFlag
	fs.Var(&codePaths, "code", "current generated code file or directory path; may be repeated")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintln(stderr, "PUBLIC_INPUT: invalid command arguments")
		return 2
	}
	if strings.TrimSpace(*manifestPath) == "" {
		fmt.Fprintln(stdout, "Doctor\n\nmanifest: skipped")
		return 0
	}
	stored, err := manifest.Load(*manifestPath)
	if err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	current, err := buildManifestFromFlags("", *schemaPath, *policyPath, *databaseSchemaPath, "", codePaths)
	if err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	verification := manifest.Verify(stored, current, time.Time{})
	if _, err := fmt.Fprintln(stdout, "Doctor"); err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	if _, err := fmt.Fprintln(stdout); err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	if err := manifest.WriteVerificationPretty(stdout, verification); err != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	if !verification.Fresh {
		return 1
	}
	return 0
}

func buildManifestFromFlags(dialect, schemaPath, policyPath, databaseSchemaPath, generatorVersion string, codePaths []string) (*manifest.Manifest, error) {
	var schema *migration.Schema
	if strings.TrimSpace(schemaPath) != "" {
		loaded, err := loadSchema(schemaPath)
		if err != nil {
			return nil, err
		}
		schema = loaded
	}
	var databaseSchema *migration.Schema
	if strings.TrimSpace(databaseSchemaPath) != "" {
		loaded, err := loadSchema(databaseSchemaPath)
		if err != nil {
			return nil, err
		}
		databaseSchema = loaded
	}
	var policies []query.TablePolicy
	if strings.TrimSpace(policyPath) != "" {
		loaded, err := loadPolicies(policyPath)
		if err != nil {
			return nil, err
		}
		policies = loaded
	}
	return manifest.Generate(manifest.Options{
		Dialect:            dialect,
		Schema:             schema,
		Policies:           policies,
		GeneratedCodePaths: codePaths,
		DatabaseSchema:     databaseSchema,
		GeneratorVersion:   generatorVersion,
	})
}

func loadSchema(path string) (*migration.Schema, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var schema migration.Schema
	if err := json.Unmarshal(b, &schema); err != nil {
		return nil, err
	}
	return &schema, nil
}

func loadPolicies(path string) ([]query.TablePolicy, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var policies []query.TablePolicy
	if err := json.Unmarshal(b, &policies); err == nil {
		return policies, nil
	}
	var wrapped struct {
		Policies []query.TablePolicy `json:"policies"`
	}
	if err := json.Unmarshal(b, &wrapped); err != nil {
		return nil, err
	}
	return wrapped.Policies, nil
}

func writeManifestOutput(w io.Writer, format string, m *manifest.Manifest) error {
	switch format {
	case "", "pretty", "text":
		return manifest.WritePretty(w, m)
	case "json":
		return publicoutput.WriteSummary(w, m.PublicView())
	default:
		return fmt.Errorf("unknown manifest format %q", format)
	}
}

func normalizeManifestFormat(format string) (string, bool) {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "", "pretty", "text":
		return "pretty", true
	case "json":
		return "json", true
	default:
		return "", false
	}
}

func printManifestUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: goquent manifest [command]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  manifest          generate a manifest")
	fmt.Fprintln(w, "  manifest schema   print the manifest JSON Schema")
	fmt.Fprintln(w, "  manifest verify   verify a stored manifest against current inputs")
	fmt.Fprintln(w, "  manifest diff     alias of verify for fingerprint diffs")
	fmt.Fprintln(w, "  manifest repository generate a repository skeleton from a manifest table")
}
