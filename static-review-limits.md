# Static Review Limits

`goquent review` can inspect Go source, raw SQL files, `QueryPlan` JSON, and `MigrationPlan` JSON.
Static analysis is useful in CI, but it is not omniscient.

Every finding has `analysis_precision`:

- `precise`: Goquent reconstructed the query or plan shape directly.
- `partial`: Goquent found a relevant Goquent call but could not reconstruct all dynamic pieces.
- `unsupported`: Goquent found a database pattern it cannot analyze precisely.

Examples that are usually precise:

```go
err := db.Table("users").
    Select("id", "name").
    Where("tenant_id", tenantID).
    Limit(100).
    Get(&rows)
```

Examples that can become partial or unsupported:

```go
q := buildUserQuery(db, tenantID)
err := q.Get(&rows)

sql := "SELECT * FROM " + table
rows, err := db.Query(sql)
```

Interpretation rule: if review says `partial` or `unsupported`, do not claim the query is safe.
Treat it as a prompt to add a runtime `QueryPlan`, reduce dynamic SQL, or add focused tests.

Static review is strongest when:

- Goquent builder calls are inline and explicit.
- `goquent review` receives a fresh manifest and current schema/policy inputs so tenant, soft
  delete, PII, required-filter, and unique-key metadata can be checked.
- Query plans are checked into artifacts or generated in tests.
- Raw SQL is isolated in `.sql` files for review.
- Suppressions include reasons and expiration dates.
