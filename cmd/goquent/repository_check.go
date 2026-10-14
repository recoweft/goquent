package main

import (
	"flag"
	"fmt"
	"github.com/recoweft/goquent/orm/manifest"
	"io"
)

func runRepositoryCheck(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("check-repositories", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	config := flags.String("config", "", "Repository registry")
	if flags.Parse(args) != nil || *config == "" || flags.NArg() != 0 {
		fmt.Fprintln(stderr, "REPOSITORY_CHECK_INVALID: details omitted")
		return 2
	}
	r := manifest.CheckRepositories(*config)
	if r.Status != "current" {
		// Even a future source error cannot enter this publication boundary.
		status := "invalid"
		switch r.Status {
		case "invalid_config", "missing_input", "missing_root", "invalid_target", "unregistered_target", "missing_target", "stale_version", "invalid_snapshot", "stale_source":
			status = r.Status
		}
		fmt.Fprintln(stderr, "REPOSITORY_CHECK_FAILED: "+status+"; details omitted")
		return 1
	}
	fmt.Fprintln(stdout, "REPOSITORY_CHECK_PASSED: supplied source comparison only")
	return 0
}
