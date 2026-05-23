# Review CLI Guide

`goquent review` reviews Go source, raw SQL, `QueryPlan` JSON, and `MigrationPlan` JSON.

```bash
go run ./cmd/goquent review --fail-on high --format pretty ./...
go run ./cmd/goquent review --format json ./...
go run ./cmd/goquent review --format github ./...
```

Useful flags:

- `--fail-on low|medium|high|destructive|blocked`: return exit code `1` at or above threshold.
- `--fail-on-precision partial|unsupported`: return exit code `1` when static review precision is
  at or above the selected limitation.
- `--format pretty|json|github`: select human, machine, or GitHub annotation output.
- `--show-suppressed`: include suppressed findings in the main output.
- `--manifest path`: include manifest freshness status.
- `--schema path`, `--policy path`, `--code path`, `--database-schema path`: current inputs used to
  verify manifest freshness during review.
- `--require-fresh-manifest`: return exit code `3` if the manifest is missing, stale, or
  unverified against current inputs.
- `--config path`: load JSON review defaults, rule overrides, and config suppressions.

Exit codes:

- `0`: no findings at or above the threshold.
- `1`: findings reached the selected threshold.
- `2`: parse, input, or configuration error.
- `3`: review required a fresh manifest and the manifest was missing, stale, or unverified.

Config example:

```json
{
  "manifest": "goquent.manifest.json",
  "schema": "schema.json",
  "policy": "policies.json",
  "code": ["./internal/db"],
  "require_fresh_manifest": true,
  "fail_on": "high",
  "fail_on_precision": "partial",
  "rules": {
    "LIMIT_MISSING": {"severity": "medium", "enabled": true}
  },
  "suppressions": [
    {
      "code": "LIMIT_MISSING",
      "path": "internal/admin/export.go",
      "reason": "admin export is bounded by caller authorization and audit logging",
      "owner": "platform",
      "expires": "2026-08-01"
    }
  ]
}
```

CI example:

```yaml
name: goquent-review
on: [pull_request]
jobs:
  database-review:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go test ./...
      - run: go run ./cmd/goquent manifest verify --manifest goquent.manifest.json --schema schema.json --policy policies.json
      - run: >
          go run ./cmd/goquent review --format github --fail-on high
          --manifest goquent.manifest.json
          --schema schema.json
          --policy policies.json
          --require-fresh-manifest
          ./...
```

Review output should be copied into PR comments for database-affecting changes, especially
findings with `partial` or `unsupported` precision. Those precision levels are review limitations,
not proof that the database operation is safe.
