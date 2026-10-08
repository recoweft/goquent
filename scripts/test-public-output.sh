#!/bin/sh
# CI publication boundary: raw test/build/driver diagnostics stay ephemeral.
# Never upload this file or print its contents, including on failure.
set -u
umask 077
# Match the explicit MySQL source used by the CI database suite without printing it.
TEST_DB_DSN=${TEST_DB_DSN:-${TEST_MYSQL_DSN:-}}
export TEST_DB_DSN
log_file=$(mktemp 2>/dev/null) || { printf '%s\n' 'CI_TESTS_ERROR: diagnostic capture unavailable'; exit 2; }
trap 'rm -f "$log_file" 2>/dev/null' EXIT HUP INT TERM
if { go run ./cmd/goquent manifest check-repositories --config tests/typedfixture/repositories.json >"$log_file" 2>&1; } 2>/dev/null; then
    printf '%s\n' 'CI_REPOSITORIES_PASSED: supplied source comparison succeeded; details omitted'
else
    result=$?
    printf '%s\n' 'CI_REPOSITORIES_FAILED: source comparison failed; details omitted'
    exit "$result"
fi
if { go test ./... -count=1 >"$log_file" 2>&1; } 2>/dev/null; then
    printf '%s\n' 'CI_TESTS_PASSED: test command succeeded; detailed diagnostics omitted'
else
    result=$?
    printf '%s\n' 'CI_TESTS_FAILED: test command failed; detailed diagnostics omitted'
    exit "$result"
fi
