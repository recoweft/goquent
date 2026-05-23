# MigrationPlan Guide

`MigrationPlan` reviews schema SQL before it is applied. It is a review artifact for migration
shape, not proof that the migration is business-approved or operationally safe.

```bash
go run ./cmd/goquent migrate plan --format pretty migrations/001.sql
go run ./cmd/goquent migrate plan --format json --fail-on high migrations/001.sql
go run ./cmd/goquent migrate dry-run migrations/001.sql
go run ./cmd/goquent migrate dry-run --approve "legacy column retired" migrations/001.sql
```

Supported step types include:

- `add_table`
- `drop_table`
- `add_column`
- `drop_column`
- `rename_column`
- `alter_column_type`
- `alter_nullability`
- `add_index`
- `drop_index`
- `unsupported`

Risk examples:

- Dropping a table or column is destructive.
- Adding a not-null column without a default is risky.
- Renaming or altering a column type is high risk.
- Enforcing `NOT NULL` on an existing nullable column is high risk.
- Adding a PostgreSQL index without `CONCURRENTLY` is medium risk.
- Unsupported SQL is reported as partial or unsupported.

`migrate dry-run` validates the plan and approval gates without executing SQL. High and
destructive migrations require an explicit approval reason before dry-run or human-controlled
apply can pass.
Destructive steps also include suggested preflight checks, such as code search, rollout order,
backup or rollback preparation, and evidence that dropped objects are no longer used.

Programmatic use:

```go
plan, err := migration.PlanSQL(`ALTER TABLE users DROP COLUMN legacy_email;`)
if err != nil {
    return err
}
if plan.RequiresApproval() {
    // require a human reason before human-controlled apply
}
```

## Lightweight Status Reader

`ReadStatus` is a small readiness helper for reading the migration table. It
answers whether the table exists, which versions are applied, what the latest
applied version is, whether desired versions are pending, and whether a
configured dirty column contains a dirty row.

It is not a drift detector. It does not compare live database schema,
manifests, generated code, migration file fingerprints, or application
expectations. If the database has applied versions that are not present in the
caller-provided desired list, the status includes a warning and marks the
result as unknown instead of claiming a complete drift verdict.

```go
desired := []string{
    "202605220001_create_users",
    "202605220002_add_document_nodes",
}

status, err := migration.ReadStatus(
    ctx,
    sqlDB,
    driver.PostgresDialect{},
    desired,
    migration.WithStatusTable("schema_migrations"),
    migration.WithStatusAppliedAtColumn("applied_at"),
    migration.WithStatusDirtyColumn("dirty"),
)
if err != nil {
    return err
}
if !status.Exists || status.Dirty || len(status.Pending) > 0 {
    // not ready
}
```

Readiness endpoint sketch:

```go
func readyz(w http.ResponseWriter, r *http.Request) {
    status, err := migration.ReadStatus(
        r.Context(),
        sqlDB,
        driver.PostgresDialect{},
        desiredMigrationVersions,
        migration.WithStatusDirtyColumn("dirty"),
    )
    if err != nil {
        http.Error(w, err.Error(), http.StatusServiceUnavailable)
        return
    }
    if !status.Exists || status.Dirty || len(status.Pending) > 0 {
        http.Error(w, "migrations pending", http.StatusServiceUnavailable)
        return
    }
    w.WriteHeader(http.StatusNoContent)
}
```

The first implementation supports PostgreSQL and MySQL migration tables. The
default table is `schema_migrations` and the default version column is
`version`. Desired versions are supplied by the caller; file discovery and
manifest/schema/generated-code comparison are explicit non-goals for this
helper.

## Human-Controlled Apply

`migrate apply` validates the same approval gates and requires `--driver` and `--dsn`, but it is a
human-controlled deployment command, not an AI-agent workflow and not an MCP tool.

`--approve` records an explicit reason for a high or destructive migration. It is audit context;
it is not business approval by itself.

AI agents may prepare or review migration artifacts, but must not run migration apply. The MCP
server intentionally exposes migration review only, not migration apply.
