package main

import (
	"fmt"
	"io"

	"github.com/recoweft/goquent/orm/query"
	"github.com/recoweft/goquent/orm/review"
)

func runReviewCI(opts review.Options, diff, base, baseline, format string, risk query.RiskLevel, precision query.AnalysisPrecision, stdout, stderr io.Writer) int {
	ci := review.CIOptions{BaseRoot: base, FailOn: risk, FailOnPrecision: precision}
	fail := func() int { fmt.Fprintln(stderr, "PUBLIC_INPUT: invalid review comparison input"); return 2 }
	if diff != "" || base != "" || baseline != "" {
		if diff == "" || base == "" || len(opts.Paths) > 1 {
			return fail()
		}
		ci.Root = "."
		if len(opts.Paths) == 1 {
			ci.Root = opts.Paths[0]
		}
		var e error
		ci.Diff, e = review.ReadCIInput(diff, review.MaxDiffBytes)
		if e != nil {
			return fail()
		}
		if baseline != "" {
			ci.Baseline, e = review.ReadCIInput(baseline, review.MaxBaselineBytes)
			if e != nil {
				return fail()
			}
		}
	}
	result, e := review.RunCI(opts, ci)
	if e != nil {
		return fail()
	}
	if result.WriteDisplay(stdout, format) != nil {
		fmt.Fprintln(stderr, "PUBLIC_OUTPUT: operation failed; details omitted")
		return 2
	}
	return result.ExitCode()
}
