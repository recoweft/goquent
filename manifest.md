# Manifest Guide

The manifest is an AI-readable export of schema and policy metadata. It gives review tools and AI
editors the context they need without granting database access.

Generate a manifest:

```bash
go run ./cmd/goquent manifest --format json \
  --dialect mysql \
  --schema schema.json \
  --policy policies.json \
  --code ./orm \
  > goquent.manifest.json
```

The manifest contains:

- version and generator metadata.
- schema fingerprint.
- policy fingerprint.
- generated-code fingerprint.
- optional database fingerprint.
- tables, columns, indexes, relations, policies, and query examples.
- optional verification status.

Policy JSON may be an array:

```json
[
  {
    "table": "users",
    "tenant_column": "tenant_id",
    "soft_delete_column": "deleted_at",
    "pii_columns": ["email"],
    "required_filter_columns": ["tenant_id"]
  }
]
```

Print the manifest JSON Schema:

```bash
go run ./cmd/goquent manifest schema
```

Verify freshness before using a stored manifest as AI or review context:

```bash
go run ./cmd/goquent manifest verify \
  --manifest goquent.manifest.json \
  --schema schema.json \
  --policy policies.json
```

Database fingerprint comparison is opt-in during verify:

```bash
go run ./cmd/goquent manifest verify \
  --manifest goquent.manifest.json \
  --database-schema database-schema.json \
  --against-db
```

`manifest verify` returns `0` when the stored manifest matches current inputs and `1` when it is
stale. A stale manifest must not be used to justify schema, policy, tenant, PII, relation, or
migration decisions.

Use the manifest with:

- `goquent review --manifest goquent.manifest.json`
- `goquent operation compile --manifest goquent.manifest.json --spec operation.json`
- `goquent mcp --manifest goquent.manifest.json`

Generate a repository skeleton for one manifest table:

```bash
go run ./cmd/goquent manifest repository \
  --manifest goquent.manifest.json \
  --table users \
  --package infra
```

The generated skeleton includes:

- a row struct with `db` tags derived from manifest columns.
- `BaseQuery` and `PlanBaseQuery` methods for repository-level `QueryPlan` tests.
- required predicate guards from tenant and required-filter manifest metadata.
- small scope helpers such as `UserTenantIDScope(...)`.
- basic `SelectAll`, `Insert`, and primary-key `FindByID` / `UpdateByID` / `DeleteByID` methods when a single primary column is known.

Regenerate the manifest whenever schema, policy, generated ORM code, or database state changes.
