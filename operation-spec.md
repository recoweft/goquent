# OperationSpec Guide

`OperationSpec` is the narrow structured interface for AI-generated read queries. It compiles to a
`QueryPlan` only after validation against a manifest.

The current OperationSpec scope is intentionally narrow:

- read-only `select` only
- single model only
- explicit selected fields
- filters
- order by
- limit

```json
{
  "operation": "select",
  "model": "User",
  "select": ["id", "name"],
  "filters": [
    {"field": "tenant_id", "op": "=", "value_ref": "current_tenant"}
  ],
  "order_by": [
    {"field": "id", "direction": "asc"}
  ],
  "limit": 100
}
```

Compile from CLI:

```bash
go run ./cmd/goquent operation schema
go run ./cmd/goquent operation compile \
  --manifest goquent.manifest.json \
  --spec operation.json \
  --values values.json \
  --require-fresh-manifest \
  --format json
```

Programmatic compile:

```go
plan, err := operation.Compile(ctx, spec, operation.Options{
    Manifest:             m,
    Values:               map[string]any{"current_tenant": tenantID},
    RequireFreshManifest: true,
})
```

Current constraints:

- Only `select` is supported.
- The model must exist in the manifest.
- Selected fields must be explicit.
- Unknown or forbidden fields are rejected.
- Every `value_ref` must be present in the supplied values map.
- Required filters from manifest policy must be present.
- PII fields require an access reason.
- Joins, aggregates, group by, having, subqueries, raw SQL hints, CTEs, and mutation fields are not current OperationSpec features.

Use OperationSpec when asking AI to propose supported database reads. Use normal Go code and human
review for writes. Mutation OperationSpec would require a separate safety design and must not be
confused with AI autonomous database execution.
