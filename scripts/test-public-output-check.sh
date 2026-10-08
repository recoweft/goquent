#!/bin/sh
# Exercise the CI publication boundary without a database or sensitive input.
set -eu
check_dir=$(mktemp -d)
trap 'rm -rf "$check_dir"' EXIT HUP INT TERM
cat > "$check_dir/go" <<'GO'
#!/bin/sh
printf '%s\n' 'fixture-ci-canary@example.test' >&1
printf '%s\n' 'fixture-ci-canary@example.test' >&2
if test "$1" = run; then exit "${FIXTURE_CHECK_EXIT:-0}"; fi
exit "${FIXTURE_TEST_EXIT:-0}"
GO
chmod 700 "$check_dir/go"
for scenario in success check_failure check_build_failure test_failure test_build_failure; do
    check_exit=0
    test_exit=0
    case "$scenario" in
        check_failure) check_exit=1;;
        check_build_failure) check_exit=2;;
        test_failure) test_exit=1;;
        test_build_failure) test_exit=2;;
    esac
    expected=$check_exit
    test "$expected" -ne 0 || expected=$test_exit
    actual=0
    PATH="$check_dir:$PATH" FIXTURE_CHECK_EXIT="$check_exit" FIXTURE_TEST_EXIT="$test_exit" sh scripts/test-public-output.sh >"$check_dir/output" 2>&1 || actual=$?
    test "$actual" -eq "$expected"
    if grep -q 'fixture-ci-canary' "$check_dir/output"; then
        printf '%s\n' 'CI_WRAPPER_CHECK_FAILED'
        exit 1
    fi
done
printf '%s\n' 'CI_WRAPPER_CHECK_PASSED'
