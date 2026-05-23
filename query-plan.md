# QueryPlan Guide

`QueryPlan` is the review artifact for database operations. It is generated before execution and
contains SQL, params, operation type, tables, selected or written columns, predicates, risk,
warnings, approval state, and analysis precision.

```go
plan, err := db.Table("users").
    Select("id", "name").
    Where("tenant_id", tenantID).
    OrderBy("id", "asc").
    Limit(100).
    Plan(ctx)
if err != nil {
    return err
}
```

Write operations can also be planned:

```go
plan, err := db.Table("users").
    Where("id", userID).
    PlanUpdate(ctx, map[string]any{"name": "Alice"})
```

Planning does not call the database. Execution methods such as `Get`, `Update`, and `Delete`
generate a plan internally and refuse blocked operations or operations that require approval but
have no approval reason.

Use `plan.ToJSON()` when passing a plan to CI, logs, PR comments, or AI tools.

Important fields:

- `operation`: `select`, `insert`, `update`, `delete`, or `raw`.
- `sql` and `params`: the statement shape and parameter values.
- `tables`, `columns`, `predicates`: structural metadata used for review.
- `risk_level`: structural database risk.
- `warnings`: active review findings.
- `suppressed_warnings`: findings hidden by an accepted suppression.
- `required_approval`: whether execution needs an explicit reason.
- `analysis_precision`: `precise`, `partial`, or `unsupported`.

Raw SQL can be wrapped with `query.NewRawPlan(sql, args...)`. Raw plans are useful for review, but
they are high risk because Goquent cannot fully inspect arbitrary SQL. When executing raw
projections through `orm.SelectOne` or `orm.SelectAll`, use a DB copy with an approval reason and
touched-table metadata so the review artifact explains the escape hatch:

```go
plan, err := db.RequireRawApproval("reviewed audit snapshot aggregation").
    TouchedTables("audit_snapshots", "snapshot_items", "document_versions").
    RawPlan(ctx, sqlText, tenantID)
```
