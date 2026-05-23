# Manifest Stale Detection

AI tools should not rely on stale schema or policy context. Goquent tracks freshness with stable
fingerprints for schema, policy, generated code, and optional database schema.

Verify a stored manifest against current inputs:

```bash
go run ./cmd/goquent manifest verify \
  --manifest goquent.manifest.json \
  --schema schema.json \
  --policy policies.json \
  --code ./orm
```

JSON output is available for automation:

```bash
go run ./cmd/goquent manifest verify --format json \
  --manifest goquent.manifest.json \
  --schema schema.json \
  --policy policies.json
```

`goquent review` can surface stale manifests:

```bash
go run ./cmd/goquent review \
  --manifest goquent.manifest.json \
  --schema schema.json \
  --policy policies.json \
  --require-fresh-manifest \
  ./...
```

Exit behavior:

- `manifest verify` returns `0` when fresh and `1` when stale.
- `review --require-fresh-manifest` returns `3` when the manifest is missing, stale, or cannot be
  verified against current inputs.

MCP exposes freshness through `goquent://manifest` and `goquent://manifest-status`.

Recommended CI gate:

```bash
go run ./cmd/goquent manifest verify \
  --manifest goquent.manifest.json \
  --schema schema.json \
  --policy policies.json \
  --code ./orm
go run ./cmd/goquent review \
  --manifest goquent.manifest.json \
  --schema schema.json \
  --policy policies.json \
  --code ./orm \
  --require-fresh-manifest \
  ./...
```

When `--require-fresh-manifest` is used, pass at least one current input such as `--schema`,
`--policy`, `--code`, or `--database-schema`. Without current inputs, review reports
`MANIFEST_UNVERIFIED` because the manifest cannot be trusted as fresh.

If verification fails, regenerate the manifest or update the schema/policy inputs before asking AI
tools to generate database code.
