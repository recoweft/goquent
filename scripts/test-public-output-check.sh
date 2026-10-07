#!/bin/sh
# Exercise the CI publication boundary without a database or sensitive input.
set -eu
check_dir=$(mktemp -d)
trap 'rm -rf "$check_dir"' EXIT HUP INT TERM
cat > "$check_dir/go" <<'GO'
#!/bin/sh
printf '%s\n' 'fixture-ci-canary@example.test' >&1
printf '%s\n' 'fixture-ci-canary@example.test' >&2
exit "${FIXTURE_EXIT:-0}"
GO
chmod 700 "$check_dir/go"
for expected in 0 1 2; do
    actual=0
    PATH="$check_dir:$PATH" FIXTURE_EXIT="$expected" sh scripts/test-public-output.sh >"$check_dir/output" 2>&1 || actual=$?
    test "$actual" -eq "$expected"
    if grep -q 'fixture-ci-canary' "$check_dir/output"; then
        printf '%s\n' 'CI_WRAPPER_CHECK_FAILED'
        exit 1
    fi
done
printf '%s\n' 'CI_WRAPPER_CHECK_PASSED'
