#!/bin/sh
# Run from the repository root. Build/driver diagnostics remain local and ephemeral.
set -eu
umask 077
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT HUP INT TERM
fail() { printf '%s\n' 'EXAMPLE_CHECK_FAILED: details omitted'; exit 1; }
go build -o "$work/example" ./examples/ai-safe-orm >"$work/build" 2>&1 || fail
go build -o "$work/goquent" ./cmd/goquent >"$work/build" 2>&1 || fail
chmod 700 "$work/example" "$work/goquent"
"$work/example" >"$work/output" 2>&1 || fail
for mode in refuse-tenant refuse-authorization; do
    expected=3
    marker=EXAMPLE_TENANT_REFUSED
    if test "$mode" = refuse-authorization; then
        expected=4
        marker=EXAMPLE_AUTHORIZATION_DENIED_CONSUMED
    fi
    actual=0
    "$work/example" "$mode" >"$work/output" 2>&1 || actual=$?
    test "$actual" -eq "$expected" || fail
    grep -qx "$marker" "$work/output" || fail
done
"$work/goquent" manifest verify \
    --manifest examples/ai-safe-orm/goquent.manifest.json \
    --schema examples/ai-safe-orm/schema.json \
    --policy examples/ai-safe-orm/policies.json \
    --code examples/ai-safe-orm/storedmodel/users_generated.go \
    --against-db --database-schema examples/ai-safe-orm/storedmodel/mysql-database.json \
    --format json >"$work/output" 2>&1 || fail
python3 - "$work/output" <<'PY_CHECK' || fail
import json, sys
v = json.load(open(sys.argv[1]))
axes = {'schema', 'policy', 'generated_code', 'database'}
assert v['kind'] == 'goquent.verification_view' and v['version'] == 1
assert v['count'] == 4 and v['present'] is True and v['known'] is True
assert v['fresh'] is True and not v.get('truncated', False)
assert len(v['items']) == 4 and {c['check'] for c in v['items']} == axes
assert all(c['status'] == 'ok' for c in v['items'])
PY_CHECK
"$work/goquent" manifest check-repositories \
    --config examples/ai-safe-orm/storedmodel/repositories.json >"$work/output" 2>&1 || fail
actual=0
"$work/goquent" operation compile \
    --manifest examples/ai-safe-orm/goquent.manifest.json \
    --spec examples/ai-safe-orm/operation.json \
    --values examples/ai-safe-orm/values.json \
    --format json >"$work/output" 2>&1 || actual=$?
test "$actual" -eq 1 || fail
grep -q '"outcome":"rejected"' "$work/output" || fail
grep -q '"code":"OPERATION_RESERVED_BINDING"' "$work/output" || fail
printf '%s\n' 'EXAMPLE_CHECK_PASSED: offline planning and expected refusals; saved model supplied four-axis evidence'
