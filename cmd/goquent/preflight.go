package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/recoweft/goquent/internal/inputjson"
	"github.com/recoweft/goquent/orm/migration"
)

// The preflight commands have no apply path. Source files are explicitly local.
func runPreflight(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return preflightCLIError(stderr)
	}
	mode := args[0]
	if mode == "--help" || mode == "help" {
		fmt.Fprintln(stdout, "migrate preflight plan --plan-source FILE --phases FILE [--reference-schema FILE] [--unsafe-local-output NEWFILE]")
		fmt.Fprintln(stdout, "migrate preflight collect --source FILE --driver mysql|postgres --dsn DSN [--unsafe-local-output NEWFILE]")
		fmt.Fprintln(stdout, "migrate preflight verify --source FILE (offline consistency only; external origin remains unknown)")
		fmt.Fprintln(stdout, "Exit: plan 0 means generation only; collect 0 means observed necessary conditions only; unknown/unsupported/nonzero 1; input/read errors 2. UTC system clock, maximum age five minutes.")
		return 0
	}
	if mode != "plan" && mode != "collect" && mode != "verify" {
		return preflightCLIError(stderr)
	}
	fs := flag.NewFlagSet("preflight", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	source := fs.String("source", "", "sensitive preflight source file")
	planFile := fs.String("plan-source", "", "sensitive MigrationPlan source file")
	phasesFile := fs.String("phases", "", "phase declaration JSON array")
	var refs stringListFlag
	fs.Var(&refs, "reference-schema", "explicit additional source schema scope")
	output := fs.String("unsafe-local-output", "", "sensitive artifact to new local file; forbidden in CI")
	driver := fs.String("driver", "", "explicit collection driver")
	dsn := fs.String("dsn", "", "explicit collection DSN")
	format := fs.String("format", "json", "json or pretty")
	if fs.Parse(args[1:]) != nil || fs.NArg() != 0 || (*format != "json" && *format != "pretty") {
		return preflightCLIError(stderr)
	}
	read := func(path string, v any) error {
		f, e := os.Open(path)
		if e != nil {
			return e
		}
		defer f.Close()
		b, e := io.ReadAll(io.LimitReader(f, 8<<20+1))
		if e != nil || len(b) > 8<<20 {
			return migration.ErrPreflightInput
		}
		if inputjson.Document(b) != nil {
			return migration.ErrPreflightInput
		}
		return json.Unmarshal(b, v)
	}
	var p *migration.Preflight
	var err error
	if mode == "plan" {
		if *source != "" || *driver != "" || *dsn != "" {
			return preflightCLIError(stderr)
		}
		var plan migration.MigrationPlan
		var phases []migration.PhaseContract
		var schemas []migration.Schema
		if read(*planFile, &plan) != nil {
			return preflightCLIError(stderr)
		}
		if *phasesFile != "" && read(*phasesFile, &phases) != nil {
			return preflightCLIError(stderr)
		}
		for _, path := range refs {
			var s migration.Schema
			if read(path, &s) != nil {
				return preflightCLIError(stderr)
			}
			schemas = append(schemas, s)
		}
		p, err = migration.PlanPreflight(&plan, phases, schemas)
	} else {
		if *planFile != "" || *phasesFile != "" || len(refs) > 0 {
			return preflightCLIError(stderr)
		}
		p = &migration.Preflight{}
		err = read(*source, p)
	}
	if err != nil || migration.ValidatePreflightSource(p) != nil {
		return preflightCLIError(stderr)
	}
	opts := migration.PreflightOptions{Clock: time.Now, MaxAge: migration.PreflightMaxAge}
	var view migration.PreflightView
	code := 0
	if mode == "collect" {
		dialect, e := migrationDialect(*driver)
		if e != nil || *dsn == "" {
			return preflightCLIError(stderr)
		}
		db, e := sql.Open(*driver, *dsn)
		if e != nil {
			return preflightCLIError(stderr)
		}
		defer db.Close()
		collector, e := migration.NewPreflightCollector(context.Background(), db, dialect, migration.CollectorOptions{Clock: time.Now})
		if e != nil {
			return preflightCLIError(stderr)
		}
		defer collector.Close()
		p, err = collector.Collect(context.Background(), p)
		if p == nil {
			return preflightCLIError(stderr)
		}
		var evalErr error
		view, evalErr = collector.Evaluate(p, opts)
		if err != nil || evalErr != nil {
			code = 2
		} else if view.Status != "satisfied" {
			code = 1
		}
	} else {
		if *driver != "" || *dsn != "" {
			return preflightCLIError(stderr)
		}
		view, err = migration.EvaluatePreflight(p, opts)
		if err != nil {
			code = 2
		} else if mode == "verify" {
			code = 1
		}
	}
	if *output != "" {
		b, e := json.Marshal(p)
		if e != nil || saveLocalArtifact(*output, b) != nil {
			return preflightCLIError(stderr)
		}
	}
	if _, e := fmt.Fprintln(stdout, view.String()); e != nil {
		return 2
	}
	if code == 2 {
		fmt.Fprintln(stderr, "PREFLIGHT_FAILED: preflight failed; source details omitted")
	}
	return code
}
func preflightCLIError(w io.Writer) int {
	fmt.Fprintln(w, "PREFLIGHT_INPUT: preflight failed; source details omitted")
	return 2
}
