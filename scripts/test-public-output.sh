#!/bin/sh
# CI publication boundary: raw test/build/driver diagnostics stay ephemeral.
# Never upload this file or print its contents, including on failure.
set -u
umask 077
log_file=$(mktemp 2>/dev/null) || { printf '%s\n' 'CI_TESTS_ERROR: diagnostic capture unavailable'; exit 2; }
trap 'rm -f "$log_file" 2>/dev/null' EXIT HUP INT TERM
if { go test ./... -count=1 >"$log_file" 2>&1; } 2>/dev/null; then
    printf '%s\n' 'CI_TESTS_PASSED: test command succeeded; detailed diagnostics omitted'
else
    result=$?
    printf '%s\n' 'CI_TESTS_FAILED: test command failed; detailed diagnostics omitted'
    exit "$result"
fi
