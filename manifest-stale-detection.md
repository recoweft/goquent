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

## Content integrity and compatibility

GQ-MAINT-04 requires both inputs to `manifest.Verify` to satisfy the existing
minimal versioned contract. Unknown/missing versions or required metadata/hashes
return `Fresh=false` with `manifest: stale`. CLI `manifest verify`, its `diff`
alias, and `doctor` reject invalid stored contracts with a fixed error and exit 2.
Schema/policy fingerprints must also agree with their own Tables projection on
both inputs. A mismatch returns the corresponding stale check (CLI exit 1),
even when both supplied fingerprint strings are identical. Previously these
malformed inputs could report fresh. Regenerate from authoritative current inputs;
verification neither repairs nor normalizes stored content.

Version 1, wire layout, hash algorithms, projection fields, and ordering rules
remain unchanged. Generated-code/database fingerprints retain their external
input comparisons and optional skip behavior; Tables cannot prove their content.
`--against-db` remains explicit. Equal fingerprints prove only supplied snapshot
correspondence: no signature, live identity, authorization, or complete schema
capability assessment is added. A coordinated rewrite of content and hashes is
not authenticated. Generation time and other fields outside the existing hash
projections are not content-integrity guarantees. Legacy missing type-source
manifests remain stale against current generation.

Review with current inputs and MCP current-input loading use the same Verify
result. Load, attached verification claims, and PublicView are not revalidation
or private execution binding. Review requiring freshness still needs current
inputs. Operation compilation retains its independent tenant and execution gates.
Migration constraint/context/capability unknowns remain separate from manifest
freshness; schema OK and database hash agreement do not certify those axes.
