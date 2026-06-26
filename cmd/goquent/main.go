package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/review"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "review":
		return runReview(args[1:], stdout, stderr)
	case "migrate":
		return runMigrate(args[1:], stdout, stderr)
	case "manifest":
		return runManifest(args[1:], stdout, stderr)
	case "operation":
		return runOperation(args[1:], stdout, stderr)
	case "mcp":
		return runMCP(args[1:], os.Stdin, stdout, stderr)
	case "doctor":
		return runDoctor(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runReview(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("goquent review", flag.ContinueOnError)
	fs.SetOutput(stderr)
	failOn := fs.String("fail-on", "high", "risk threshold that returns exit code 1")
	failOnPrecision := fs.String("fail-on-precision", "", "analysis precision threshold that returns exit code 1: partial or unsupported")
	format := fs.String("format", "pretty", "output format: pretty, json, github")
	showSuppressed := fs.Bool("show-suppressed", false, "include suppressed findings in the primary output")
	configPath := fs.String("config", "", "path to a JSON goquent review config file")
	manifestPath := fs.String("manifest", "", "manifest JSON path for freshness warnings")
	schemaPath := fs.String("schema", "", "current schema JSON path for manifest freshness verification")
	policyPath := fs.String("policy", "", "current table policy JSON path for manifest freshness verification")
	databaseSchemaPath := fs.String("database-schema", "", "current database schema JSON path for manifest freshness verification")
	requireFreshManifest := fs.Bool("require-fresh-manifest", false, "return exit code 3 when the manifest is stale")
	var codePaths stringListFlag
	fs.Var(&codePaths, "code", "current generated code file or directory path for manifest freshness verification; may be repeated")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: goquent review [flags] [path ...]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	explicit := explicitFlagSet(fs)
	var cfg review.Config
	if strings.TrimSpace(*configPath) != "" {
		loaded, err := review.LoadConfig(*configPath)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		cfg = loaded
		if !explicit["manifest"] && cfg.Manifest != "" {
			*manifestPath = cfg.Manifest
		}
		if !explicit["schema"] && cfg.Schema != "" {
			*schemaPath = cfg.Schema
		}
		if !explicit["policy"] && cfg.Policy != "" {
			*policyPath = cfg.Policy
		}
		if !explicit["database-schema"] && cfg.DatabaseSchema != "" {
			*databaseSchemaPath = cfg.DatabaseSchema
		}
		if !explicit["code"] && len(cfg.Code) > 0 {
			codePaths = append(stringListFlag(nil), cfg.Code...)
		}
		if !explicit["require-fresh-manifest"] && cfg.RequireFreshManifest {
			*requireFreshManifest = true
		}
		if !explicit["fail-on"] && cfg.FailOn != "" {
			*failOn = cfg.FailOn
		}
		if !explicit["fail-on-precision"] && cfg.FailOnPrecision != "" {
			*failOnPrecision = cfg.FailOnPrecision
		}
		if !explicit["show-suppressed"] && cfg.ShowSuppressed {
			*showSuppressed = true
		}
	}

	manifestInputs := strings.TrimSpace(*schemaPath) != "" ||
		strings.TrimSpace(*policyPath) != "" ||
		strings.TrimSpace(*databaseSchemaPath) != "" ||
		len(codePaths) > 0
	var currentManifestForReview *manifest.Manifest
	if manifestInputs {
		current, err := buildManifestFromFlags("", *schemaPath, *policyPath, *databaseSchemaPath, "", codePaths)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		currentManifestForReview = current
	}

	threshold, err := review.ParseRiskLevel(*failOn)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	precisionThreshold, err := review.ParseAnalysisPrecision(*failOnPrecision)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	precisionThresholdSet := strings.TrimSpace(*failOnPrecision) != ""
	outputFormat := strings.ToLower(strings.TrimSpace(*format))
	switch outputFormat {
	case "", "pretty", "text", "json", "github":
	default:
		fmt.Fprintf(stderr, "unknown review format %q\n", *format)
		return 2
	}

	report, err := review.Run(review.Options{
		Paths:                fs.Args(),
		ShowSuppressed:       *showSuppressed,
		ConfigPath:           *configPath,
		ManifestPath:         *manifestPath,
		RequireFreshManifest: *requireFreshManifest,
		CurrentManifest:      currentManifestForReview,
		ManifestInputs:       manifestInputs,
		Rules:                cfg.Rules,
		ConfigSuppressions:   cfg.Suppressions,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	switch outputFormat {
	case "", "pretty", "text":
		err = review.WritePretty(stdout, report)
	case "json":
		err = review.WriteJSON(stdout, report)
	case "github":
		err = review.WriteGitHub(stdout, report)
	}
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	if *requireFreshManifest && report.ManifestStatus != nil && !report.ManifestStatus.Fresh {
		return 3
	}
	if review.HasFindingsAtOrAbove(report, threshold) {
		return 1
	}
	if precisionThresholdSet && review.HasFindingsAtOrAbovePrecision(report, precisionThreshold) {
		return 1
	}
	return 0
}

func explicitFlagSet(fs *flag.FlagSet) map[string]bool {
	out := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		out[f.Name] = true
	})
	return out
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: goquent <command>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  review    review Go source, QueryPlan JSON, and raw SQL files")
	fmt.Fprintln(w, "  migrate   plan, dry-run, or apply migration SQL")
	fmt.Fprintln(w, "  manifest  generate and verify AI-readable schema manifests")
	fmt.Fprintln(w, "  operation compile structured read-only OperationSpec")
	fmt.Fprintln(w, "  mcp       run read-only MCP server over stdio")
	fmt.Fprintln(w, "  doctor    run local Goquent health checks")
}
