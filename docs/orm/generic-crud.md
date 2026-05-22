# Generic CRUD

The generic API is the small set of helpers around:

- `SelectOne[T]`
- `SelectAll[T]`
- `Insert[T]`
- `Update[T]`
- `Upsert[T]`
- `InsertReturning[T]`
- `UpdateReturning[T]`
- `UpsertReturning[T]`

Use these helpers when you want a compact typed call and the operation is simple enough to fit their model. Use the query-builder API when you want to build SQL fluently with `db.Model(...).Where(...).Get(...)`, `db.Table(...).Where(...).FirstMap(...)`, joins, non-primary-key updates, or other builder features.

The snippets below assume a setup like this:

```go
type User struct {
    ID     int64  `db:"id,pk"`
    Name   string `db:"name"`
    Age    int    `db:"age"`
    Active bool   `db:"active"`
}

ctx := context.Background()
db, err := orm.OpenWithDriver(orm.MySQL, dsn)
```

## Overview

The read side of the generic API executes raw SQL that you provide and scans the result into a concrete Go type:

```go
user, err := orm.SelectOne[User](ctx, db, "SELECT id, name, age FROM users WHERE id = ?", 1)
rows, err := orm.SelectAll[map[string]any](ctx, db, "SELECT id, name FROM users ORDER BY id")
```

The query-builder API does the SQL construction for you:

```go
var user User
err := db.Model(&User{}).Where("id", 1).First(&user)

var row map[string]any
err = db.Table("users").Where("id", 1).FirstMap(&row)
```

The write side of the generic API builds simple `INSERT`, `UPDATE`, and `UPSERT` statements from a struct value or `map[string]any`:

```go
_, err := orm.Insert(ctx, db, User{Name: "sam", Age: 18})
_, err = orm.Update(ctx, db, User{ID: 1, Name: "sam"}, orm.Columns("name"), orm.WherePK())
_, err = orm.Upsert(ctx, db, User{ID: 1, Name: "sam", Age: 18}, orm.WherePK())
created, err := orm.InsertReturning[User](ctx, db, User{Name: "sam", Age: 18})
```

For anything more complex than "write this row to this table", prefer
`db.Table(...).Where(...).Update(...)` so Goquent can still plan and review the operation. Use raw
SQL only when the builder cannot reasonably express the query, and include a `QueryPlan`,
`goquent review` output, or manual review evidence in the PR.

When you want to keep the generic API as the main entry point but still need joins, arbitrary predicates, `DELETE`, or reusable query composition, use `orm.Scope` with the scoped helpers:

```go
func WithProfile() orm.Scope {
    return func(q *query.Query) *query.Query {
        return q.Join("profiles", "users.id", "=", "profiles.user_id")
    }
}

func BioLike(v string) orm.Scope {
    return func(q *query.Query) *query.Query {
        return q.Where("profiles.bio", "like", v)
    }
}

users, err := orm.SelectAllBy[User](
    ctx,
    db,
    db.Model(&User{}),
    orm.ComposeScopes(WithProfile(), BioLike("%go%")),
    func(q *query.Query) *query.Query {
        return q.Select("users.id", "users.name", "users.age").OrderBy("users.id", "asc")
    },
)

_, err = orm.UpdateBy(ctx, db.Table("users"), map[string]any{"age": 55}, WithProfile(), BioLike("%go%"))
updated, err := orm.UpdateByReturning[User](ctx, db, db.Table("users"), map[string]any{"age": 55}, WithProfile(), BioLike("%go%"))
_, err = orm.DeleteBy(ctx, db.Table("users"), WithProfile(), BioLike("%python%"))

tenantUsers, err := orm.SelectAllBy[User](
    ctx,
    db,
    db.Model(&User{}),
    orm.TenantScope(tenantID),
    func(q *query.Query) *query.Query {
        return q.Select("id", "name", "age").Limit(100)
    },
)
```

## Read API

### `SelectOne[T]`

`SelectOne[T]` runs a query and scans the first row into `T`.

```go
u, err := orm.SelectOne[User](ctx, db, "SELECT id, name, age FROM users WHERE id = ?", 1)
```

If the query returns no rows, `SelectOne` returns `sql.ErrNoRows`.

```go
u, err := orm.SelectOne[User](ctx, db, "SELECT id, name, age FROM users WHERE id = ?", id)
if errors.Is(err, sql.ErrNoRows) {
    return
}
if err != nil {
    log.Fatal(err)
}
_ = u
```

### `SelectAll[T]`

`SelectAll[T]` runs a query and scans all rows into `[]T`.

```go
users, err := orm.SelectAll[User](ctx, db, "SELECT id, name, age FROM users ORDER BY id")
```

If the query returns no rows, `SelectAll` returns an empty slice and a `nil` error.

### Supported `T` shapes

The current implementation supports these destination shapes:

- A non-pointer struct type such as `User`
- Exactly `map[string]any`

The current implementation does not support, and this guide does not guarantee, these shapes:

- Pointer destinations such as `*User`
- Scalar destinations such as `int64`, `string`, or `bool`
- Slice types as `T`
- Other map shapes such as `map[string]string`

### Practical column matching

For struct destinations:

- `db:"column_name"` sets the column name explicitly.
- Without a tag, goquent uses the field name converted to `snake_case`.
- Matching first tries the exact column name.
- If that does not match, it tries a normalized match that lowercases the name and removes underscores.

In practice, these columns all match a field tagged or inferred as `schema_name`:

- `schema_name`
- `SchemaName`
- `SCHEMA_NAME`

Columns with no matching field are ignored. Fields with no matching column keep their zero value.

Struct field decoding is reflection-based. A field must either:

- be directly assignable or convertible from the driver value,
- implement `sql.Scanner` via its pointer type, or
- be `bool`, `sql.NullBool`, or `*bool`, which use goquent's bool scan policy.

For map destinations:

- keys are the column names returned by the database,
- values are stored as `any`,
- `[]byte` values are converted to `string`.

### Numeric columns

Drivers do not all return SQL `numeric`/`decimal` values as the same Go type when scanning through `any`. PostgreSQL drivers commonly expose exact numeric values as text-like data. For portable DTOs, prefer one of these shapes:

- Use `string` or `sql.NullString` in persistence rows when you need exact decimal text, then parse or round in your domain layer.
- Use a custom type that implements `sql.Scanner` when the column has business-specific decimal semantics.
- Use `float64` only when precision loss is acceptable and your driver returns a value convertible to `float64`.

Example:

```go
type ScoreRow struct {
    ID         string `db:"id"`
    Confidence string `db:"confidence"` // numeric(4,3)
}
```

## Write API

### Supported input shapes

`Insert`, `Update`, and `Upsert` currently accept:

- A non-pointer struct value
- `map[string]any`

Pointer values such as `*User` are not supported by the current implementation.

### Struct-based writes

Struct-based writes use reflection metadata from the struct:

- The table name comes from `TableName() string` when the struct value implements it, otherwise from the struct type name in `snake_case` plus `s`.
- Column names come from the `db` tag or from the field name in `snake_case`.
- `db:"...,pk"` marks primary-key fields for `WherePK()`.
- `db:"...,readonly"` excludes a field from writes.
- `db:"...,omitempty"` skips zero values on insert, update, and upsert.

Example struct:

```go
type User struct {
    ID     int64  `db:"id,pk"`
    Name   string `db:"name"`
    Age    int    `db:"age"`
    Active bool   `db:"active"`
}
```

### Map-based writes

Map-based writes use the map keys as column names exactly as given. There is no table-name inference and no primary-key inference.

That means:

- `Table("...")` is required for all map writes.
- `PK("...")` is required for map `Update` and `Upsert` when you also use `WherePK()`.
- The map must include values for every column listed in `PK(...)`.

### `Insert[T]`

`Insert` builds a single-row `INSERT`.

Struct example:

```go
_, err := orm.Insert(ctx, db, User{Name: "sam", Age: 18, Active: true})
```

Map example:

```go
_, err := orm.Insert(ctx, db, map[string]any{
    "name":   "sam",
    "age":    18,
    "active": true,
}, orm.Table("users"))
```

### `Update[T]`

`Update` only works with `WherePK()`. There is no generic helper for arbitrary `WHERE` clauses.

For structs, `WherePK()` uses fields tagged with `db:"...,pk"`. If the struct has no primary-key metadata, `Update` returns an error.

For maps, `WherePK()` uses the columns named by `PK(...)`.

```go
_, err := orm.Update(
    ctx,
    db,
    User{ID: 1, Name: "alice", Active: true},
    orm.Columns("name", "active"),
    orm.WherePK(),
)
```

### `Upsert[T]`

`Upsert` requires either `WherePK()` or an explicit conflict target.

- On MySQL it builds `INSERT ... ON DUPLICATE KEY UPDATE ...`.
- On PostgreSQL it builds `INSERT ... ON CONFLICT (...) DO UPDATE ...`.
- Primary-key columns used by `WherePK()` are always kept in the `INSERT` side of the statement, even if `Columns(...)`, `Omit(...)`, or `omitempty` would otherwise exclude them.
- Columns named by `ConflictColumns(...)` are also kept in the `INSERT` side of the statement.

If there are no non-primary-key columns left to update after filtering, the helper falls back to a no-op conflict action:

- MySQL: `INSERT IGNORE`
- PostgreSQL: `ON CONFLICT (...) DO NOTHING`

When the insert payload contains columns that must not be updated on conflict, keep those columns in the insert side and pass `UpdateColumns(...)` for the conflict update side:

```go
_, err := orm.Upsert(
    ctx,
    db,
    map[string]any{
        "id":               fieldID,
        "tenant_id":        tenantID,
        "form_instance_id": formID,
        "field_key":        "weekly_hours",
        "value_text":       "40",
        "needs_update":     false,
    },
    orm.Table("form_fields"),
    orm.ConflictColumns("tenant_id", "form_instance_id", "field_key"),
    orm.UpdateColumns("value_text", "needs_update"),
)
```

For append-only or idempotency tables where the existing row should not be touched, use `ConflictDoNothing()`:

```go
_, err := orm.Upsert(
    ctx,
    db,
    map[string]any{
        "tenant_id":       tenantID,
        "idempotency_key": key,
        "payload_json":    payload,
    },
    orm.Table("submission_attempts"),
    orm.ConflictColumns("tenant_id", "idempotency_key"),
    orm.ConflictDoNothing(),
)
```

```go
_, err := orm.Upsert(
    ctx,
    db,
    User{ID: 1, Name: "alice", Age: 31},
    orm.WherePK(),
)
```

For a PostgreSQL partial unique index, pass the indexed columns plus the index predicate:

```go
_, err := orm.Upsert(
    ctx,
    db,
    map[string]any{
        "tenant_id":       tenantID,
        "idempotency_key": key,
        "payload_json":    payload,
    },
    orm.Table("ai_audit_logs"),
    orm.ConflictColumns("tenant_id", "idempotency_key"),
    orm.ConflictWhere("idempotency_key <> ''"),
)
```

For a PostgreSQL named unique constraint, use `ConflictConstraint(...)`:

```go
_, err := orm.Upsert(
    ctx,
    db,
    map[string]any{"email": email, "name": name},
    orm.Table("users"),
    orm.ConflictConstraint("users_email_key"),
)
```

For PostgreSQL expression indexes, use `ConflictTargetRaw(...)`. This is an
explicit raw escape hatch for the conflict target only; prefer
`ConflictColumns(...)`, `ConflictWhere(...)`, or `ConflictConstraint(...)`
when they can express the index.

```go
_, err := orm.Upsert(
    ctx,
    db,
    map[string]any{
        "tenant_id":      tenantID,
        "target_node_id": targetNodeID,
        "payload_json":   payload,
    },
    orm.Table("citation_links"),
    orm.ConflictTargetRaw(`("tenant_id", COALESCE("target_node_id", '')) WHERE "active"`),
    orm.ConflictDoNothing(),
)
```

### Typed returning helpers

`InsertReturning[T]`, `UpdateReturning[T]`, and `UpsertReturning[T]` execute the same generated write statements as `Insert`, `Update`, and `Upsert`, but scan the PostgreSQL `RETURNING` row into `T`.

If you do not pass `Returning(...)`, goquent infers the returning column list from the destination struct tags.

```go
created, err := orm.InsertReturning[User](
    ctx,
    db,
    User{Name: "sam", Age: 18, Active: true},
)
```

For scoped updates, use `UpdateByReturning[T]`:

```go
updated, err := orm.UpdateByReturning[User](
    ctx,
    db,
    db.Table("users"),
    map[string]any{"active": true},
    func(q *query.Query) *query.Query {
        return q.Where("tenant_id", tenantID).Where("id", id)
    },
)
```

Map return values cannot infer a column list. For `InsertReturning`, `UpdateReturning`, and `UpsertReturning`, pass `Returning(...)` when the destination is `map[string]any`. `UpdateByReturning` currently infers columns from a struct destination.

### Insert-once returning

`InsertOnceReturning[T]` is for append-only/idempotency tables. It attempts
`ON CONFLICT DO NOTHING RETURNING ...`. If the row was inserted, the second
return value is `true`. If the conflict path returns no row, goquent looks up
the existing row by `ConflictColumns(...)` or `WherePK()`.

```go
attempt, inserted, err := orm.InsertOnceReturning[SubmissionAttemptRow](
    ctx,
    db,
    map[string]any{
        "tenant_id":       tenantID,
        "idempotency_key": key,
        "payload_json":    payload,
    },
    orm.Table("submission_attempts"),
    orm.ConflictColumns("tenant_id", "idempotency_key"),
    orm.Returning("id", "tenant_id", "idempotency_key", "payload_json"),
)
_ = inserted
_ = attempt
```

For expression-only raw conflict targets, also provide `ConflictColumns(...)`
or `WherePK()` when you need the existing-row lookup.

## Write options

### `Columns(...)`

`Columns` keeps only the listed columns.

```go
_, err := orm.Update(
    ctx,
    db,
    User{ID: 1, Name: "alice", Active: true},
    orm.Columns("name"),
    orm.WherePK(),
)
```

### `Omit(...)`

`Omit` removes columns from the write set.

```go
_, err := orm.Insert(
    ctx,
    db,
    User{Name: "sam", Age: 18, Active: true},
    orm.Omit("active"),
)
```

If you use both `Columns(...)` and `Omit(...)`, `Omit(...)` still removes the omitted columns.

### `WherePK()`

`WherePK()` is required for `Update`. `Upsert` can use `WherePK()` or an explicit conflict target.

- Struct writes: use fields tagged with `db:"...,pk"`.
- Map writes: use `PK(...)`.
- In practice you will usually combine it with `Columns(...)` or `Omit(...)` on struct updates.

```go
_, err := orm.Update(ctx, db, User{ID: 1, Name: "alice"}, orm.Columns("name"), orm.WherePK())
```

### `Returning(...)`

`Returning` appends a `RETURNING` clause only when the active dialect is PostgreSQL.

```go
_, err := orm.Update(
    ctx,
    db,
    User{ID: 1, Name: "alice"},
    orm.Columns("name"),
    orm.WherePK(),
    orm.Returning("id", "name"),
)
```

`Insert`, `Update`, and `Upsert` still return `sql.Result` when you use `Returning(...)`; they consume the returned rows to count affected rows. Use the typed returning helpers when you need row values.

### `ConflictColumns(...)`

`ConflictColumns` sets the `ON CONFLICT (...)` target for `Upsert`. It is useful for natural unique keys and PostgreSQL partial unique indexes.

```go
_, err := orm.Upsert(
    ctx,
    db,
    map[string]any{"tenant_id": tenantID, "external_key": key, "payload_json": payload},
    orm.Table("events"),
    orm.ConflictColumns("tenant_id", "external_key"),
)
```

### `ConflictWhere(...)`

`ConflictWhere` appends a PostgreSQL partial-index predicate to the conflict target. The predicate is a raw SQL fragment and is validated to reject statement separators, comments, and write/DDL keywords.

```go
_, err := orm.Upsert(
    ctx,
    db,
    event,
    orm.ConflictColumns("tenant_id", "idempotency_key"),
    orm.ConflictWhere("idempotency_key <> ''"),
)
```

### `ConflictConstraint(...)`

`ConflictConstraint` builds `ON CONFLICT ON CONSTRAINT ...` for PostgreSQL named unique constraints. It cannot be combined with `ConflictColumns(...)` or `ConflictWhere(...)`.

### `ConflictTargetRaw(...)`

`ConflictTargetRaw` supplies the PostgreSQL `ON CONFLICT` target literally. Use
it for expression indexes that cannot be represented as plain columns.

```go
_, err := orm.Upsert(
    ctx,
    db,
    row,
    orm.ConflictTargetRaw(`("tenant_id", lower("external_key")) WHERE "active"`),
    orm.ConflictDoNothing(),
)
```

It cannot be combined with `ConflictColumns(...)`, `ConflictWhere(...)`, or
`ConflictConstraint(...)`.

### `UpdateColumns(...)`

`UpdateColumns` limits only the conflict update side of `Upsert` and `UpsertReturning`. The insert side still uses `Columns(...)`, `Omit(...)`, and the required primary-key or conflict columns.

Use it when an insert payload includes application-generated IDs or immutable audit columns that must be written only on the first insert:

```go
_, err := orm.Upsert(
    ctx,
    db,
    formField,
    orm.ConflictColumns("tenant_id", "form_instance_id", "field_key"),
    orm.UpdateColumns("value_text", "value_json", "attachment_required", "needs_update"),
)
```

Every column named in `UpdateColumns` must also be present in the insert column set, because PostgreSQL `EXCLUDED` and MySQL `VALUES(...)` read from the attempted insert row.

### Expression assignments

Use assignment options when the database should compute the updated value.
They apply to `Update` and to the conflict-update side of `Upsert`; `Insert`
rejects them.

```go
_, err := orm.Update(
    ctx,
    db,
    map[string]any{"id": userID},
    orm.TablePath("app", "users"),
    orm.PK("id"),
    orm.WherePK(),
    orm.SetExpr("email_verified_at", "COALESCE(email_verified_at, ?)", verifiedAt),
    orm.Increment("credential_version", 1),
)
```

Available assignment helpers:

- `SetRaw("column", "trusted_sql_expression")`
- `SetExpr("column", "COALESCE(column, ?)", value)`
- `Increment("column", 1)`
- `SetColumn("updated_at", "password_changed_at")`

`SetRaw` and `SetExpr` validate raw fragments to reject statement
separators, comments, and write/DDL keywords. `SetExpr` rewrites `?`
placeholders for the active dialect.

### `ConflictDoNothing()`

`ConflictDoNothing` forces a no-op conflict action even when the insert payload contains non-conflict columns.

```go
_, err := orm.Upsert(
    ctx,
    db,
    auditEvent,
    orm.ConflictColumns("tenant_id", "idempotency_key"),
    orm.ConflictDoNothing(),
)
```

### `Table(...)`

`Table` overrides the inferred table name for struct writes and is required for map writes.

```go
_, err := orm.Insert(
    ctx,
    db,
    map[string]any{"name": "sam"},
    orm.Table("users"),
)
```

For schema-qualified tables, either pass a dotted table name or use
`TablePath(...)`/`SchemaName(...)`; generic writes quote each identifier part:

```go
_, err := orm.Insert(
    ctx,
    db,
    map[string]any{"name": "sam"},
    orm.TablePath("app", "users"),
)

_, err = orm.Update(
    ctx,
    db,
    User{ID: 1, Name: "sam"},
    orm.SchemaName("app"),
    orm.Columns("name"),
    orm.WherePK(),
)
```

### `PK(...)`

`PK` names the primary-key columns for map `Update` and `Upsert` when combined with `WherePK()`.

```go
_, err := orm.Update(
    ctx,
    db,
    map[string]any{"id": 1, "name": "alice"},
    orm.Table("users"),
    orm.PK("id"),
    orm.WherePK(),
)
```

`PK(...)` is for map writes. Struct writes use `db:"...,pk"` tags instead.

## Transactions

The generic helpers take `*orm.DB`. Inside a transaction callback, pass `tx.DB`.

### `db.Transaction(...)`

```go
err := db.Transaction(func(tx orm.Tx) error {
    _, err := orm.Update(
        ctx,
        tx.DB,
        User{ID: 1, Active: true},
        orm.Columns("active"),
        orm.WherePK(),
    )
    return err
})
```

### `db.TransactionContext(...)`

```go
err := db.TransactionContext(ctx, func(tx orm.Tx) error {
    user, err := orm.SelectOne[User](ctx, tx.DB, "SELECT id, name, age FROM users WHERE id = ?", 1)
    if err != nil {
        return err
    }
    _, err = orm.Update(ctx, tx.DB, User{ID: user.ID, Age: user.Age + 1}, orm.Columns("age"), orm.WherePK())
    return err
})
```

### Manual `Begin()` / `BeginTx(...)`

```go
tx, err := db.BeginTx(ctx, nil)
if err != nil {
    log.Fatal(err)
}
defer tx.Rollback()

if _, err := orm.Insert(ctx, tx.DB, User{Name: "sam", Age: 18}); err != nil {
    log.Fatal(err)
}

if err := tx.Commit(); err != nil {
    log.Fatal(err)
}
```

The same pattern works with `db.Begin()`.

When transaction ownership lives outside goquent, wrap the existing executor:

```go
txDB := orm.NewTxDB(sqlTx, driver.PostgresDialect{})
_, err := orm.Update(ctx, txDB, row, orm.Columns("name"), orm.WherePK())
```

If you already have a configured `*orm.DB`, `db.WrapTx(sqlTx)` preserves its
dialect, scan options, and raw-SQL approval state.

## JSON, nullable values, and projections

Use `JSONField[T]` in persistence rows for JSON/JSONB columns when you want
typed scan and value behavior at the repository edge.

```go
type ValidationSummary struct {
    Status string `json:"status"`
}

type FilingRow struct {
    ID                string                           `db:"id"`
    ValidationSummary orm.JSONField[ValidationSummary] `db:"validation_summary"`
}

summary := row.ValidationSummary.OrDefault(ValidationSummary{Status: "unknown"})
```

For insert/update maps, `JSONOf(value)` stores typed JSON and `JSONNull[T]()`
stores SQL NULL. `NullString`, `NullStringPtr`, and `NullStringEmpty` are small
helpers for optional string/UUID fields represented as `sql.NullString`.
Use `EncodeJSON` and `DecodeJSON` when a repository needs a plain text/JSON
column value instead of a `sql.Scanner` field:

```go
payload, err := orm.EncodeJSON(ValidationSummary{Status: "ok"})
summary, err := orm.DecodeJSON(rawPayload, ValidationSummary{Status: "unknown"})
_, _ = payload, summary
```

Wide read projections should use explicit select aliases and dedicated row
structs. For nested JSON aggregate snapshots, keep the raw SQL in a small
repository method, require raw approval, and scan into a typed row containing
`JSONField[T]` fields. That preserves the SQL review boundary without forcing
nested aggregate SQL into the structured builder.

For reviewed raw projections, carry both the approval reason and the touched
table list on the DB copy:

```go
rows, err := orm.SelectAll[WorkItemRow](
    ctx,
    db.RequireRawApproval("reviewed work item union projection").
        TouchedTables("filing_cases", "document_projects", "notices"),
    sqlText,
    tenantID,
)
```

For PostgreSQL JSONB filters, the query builder has narrow predicates for
common key lookups:

```go
err := db.Table("audit_events").
    Select("id", "payload").
    WhereJSONText("payload", "reason", "initial_sync").
    WhereJSONHasKey("payload", "cache_invalidated_at").
    WhereJSONNotHasKey("payload", "ignored_at").
    GetMaps(&rows)
```

## Scope-Based Advanced Path

`Scope` lets you keep reusable query fragments near the generic helpers instead of dropping straight to ad-hoc builder code everywhere.

### `type Scope`

`Scope` is:

```go
type Scope func(*query.Query) *query.Query
```

Each scope receives the current builder and returns the next one. In most cases you mutate and return the same query.

### `ApplyScopes(...)`

`ApplyScopes` runs scopes in order against a base query.

```go
q := orm.ApplyScopes(
    db.Table("users"),
    WithProfile(),
    BioLike("%go%"),
)
```

### `ComposeScopes(...)`

`ComposeScopes` bundles smaller scopes into a reusable larger scope.

```go
activeDevelopers := orm.ComposeScopes(
    WithProfile(),
    BioLike("%developer%"),
)
```

### `TenantScope(...)`

`TenantScope` is a small reusable scope for the common `tenant_id = ?` predicate. Pass a custom column name when the table uses a different tenant boundary column.

```go
tenantDocs := orm.ComposeScopes(
    orm.TenantScope(tenantID),
    func(q *query.Query) *query.Query {
        return q.WhereNull("archived_at")
    },
)

scopeBindings := orm.TenantScope(tenantID, "scope_tenant_id")
```

For joined queries with registered tenant policies on multiple tables, prefer qualified columns so
the `QueryPlan` can prove each table is scoped:

```go
var out []map[string]any
err := db.Table("users").
    Select("users.id", "memberships.role").
    Join("memberships", "users.id", "=", "memberships.user_id").
    Where("users.tenant_id", tenantID).
    Where("memberships.tenant_id", tenantID).
    Limit(50).
    GetMaps(&out)
```

### `CursorAfter(...)` and `CursorBefore(...)`

Cursor scopes add keyset pagination predicates without hand-written raw SQL.
The helper expands ordered columns into a lexicographic predicate, so mixed
ascending and descending cursor columns remain explicit.

```go
rows, err := orm.SelectAllBy[WorkQueueRow](
    ctx,
    db,
    db.Table("filing_cases").
        Select("id", "due_at", "title").
        OrderBy("due_at", "desc").
        OrderBy("id", "desc").
        Limit(50),
    orm.TenantScope(tenantID),
    orm.CursorAfter(
        []orm.CursorColumn{
            orm.CursorDesc("due_at"),
            orm.CursorDesc("id"),
        },
        cursorDueAt,
        cursorID,
    ),
)
```

For computed cursor keys, use the explicit expression helpers:

```go
scope := orm.CursorAfter(
    []orm.CursorColumn{
        orm.CursorDescExpr("(entity_type || ':' || entity_id)"),
    },
    cursorEntityKey,
)
```

### `SelectOneBy[T]` and `SelectAllBy[T]`

These helpers build SQL from a scoped query and still scan through the generic read path.

```go
user, err := orm.SelectOneBy[User](ctx, db, db.Model(&User{}), WithProfile(), BioLike("%go%"))
users, err := orm.SelectAllBy[User](ctx, db, db.Model(&User{}), WithProfile())
```

### `UpdateBy(...)`

`UpdateBy` applies scopes to a base query and then calls the query-builder `Update`.

```go
_, err := orm.UpdateBy(
    ctx,
    db.Table("users"),
    map[string]any{"age": 55},
    WithProfile(),
    BioLike("%go%"),
)
```

Use `UpdateByReturningWithOptions` for optimistic concurrency guards where zero
rows should be treated as a stale-write conflict instead of a generic not-found:

```go
updated, err := orm.UpdateByReturningWithOptions[EditSessionRow](
    ctx,
    db,
    db.Table("document_edit_sessions"),
    map[string]any{"draft_nodes_json": payload},
    []orm.WriteOpt{orm.NoRowsAs(orm.ErrConflict)},
    func(q *query.Query) *query.Query {
        return q.
            Where("tenant_id", tenantID).
            Where("id", id).
            Where("content_hash", previousHash)
    },
)
if orm.IsConflict(err) {
    return ErrEditSessionStale
}
_ = updated
```

### `DeleteBy(...)`

`DeleteBy` does the same for `DELETE`.

```go
_, err := orm.DeleteBy(ctx, db.Table("users"), WithProfile(), BioLike("%python%"))
```

## Dialect notes

- goquent ships with built-in `orm.MySQL` and `orm.Postgres` driver names.
- `SelectOne` and `SelectAll` execute the SQL string you pass in, so placeholder syntax must match your driver.
- `Insert`, `Update`, and `Upsert` use the configured dialect to quote identifiers and build placeholders.
- `ErrNotFound` aliases `sql.ErrNoRows`; use `IsNotFound(err)` for wrapped errors.
- `ExpectAffected(n)` validates write row counts. Combine it with `NoRowsAs(ErrConflict)` for explicit guarded writes.
- `Returning(...)` is PostgreSQL-only in the current implementation.
- Bool scanning follows the same compatibility rules as the rest of goquent. See [Boolean dialect compatibility](../../README.md#boolean-dialect-compatibility).

## Limitations and caveats

- Reads only support struct destinations and `map[string]any`. Pointer destinations are not supported.
- Writes only support non-pointer struct values and `map[string]any`.
- Generic `Update` only supports primary-key-based updates through `WherePK()`. For arbitrary predicates, use `UpdateBy` or `UpdateByReturning`.
- Generic `Upsert` can use `WherePK()`, `ConflictColumns(...)`, `ConflictConstraint(...)`, or `ConflictTargetRaw(...)`.
- Scoped helpers are the recommended bridge when you want arbitrary predicates, joins, or `DELETE` while still keeping generic read/write helpers as the main public API.
- Struct `Update` and `Upsert` depend on `db:"...,pk"` tags. Without them, `WherePK()` has no primary-key columns to use.
- Since generic writes take struct values, a `TableName() string` override must be available on the value type. A pointer-receiver-only `TableName` method is not picked up here.
- Map writes do not use struct metadata, so `readonly`, `omitempty`, and field tags do not apply.
- Mapping is reflection-based. Unmatched columns are ignored, missing columns leave zero values, and scan or type-conversion failures are returned as errors.
- There is no generic helper for `DELETE`.
- Typed returning helpers are PostgreSQL-only because other supported dialects do not expose a compatible `RETURNING` clause here.

## Examples

### Select one struct

```go
user, err := orm.SelectOne[User](ctx, db, "SELECT id, name, age, active FROM users WHERE id = ?", 1)
if err != nil {
    log.Fatal(err)
}
_ = user
```

### Select many structs

```go
users, err := orm.SelectAll[User](ctx, db, "SELECT id, name, age, active FROM users WHERE active = ? ORDER BY id", true)
if err != nil {
    log.Fatal(err)
}
_ = users
```

### Select one `map[string]any`

```go
row, err := orm.SelectOne[map[string]any](ctx, db, "SELECT id, name FROM users WHERE id = ?", 1)
if err != nil {
    log.Fatal(err)
}
_ = row
```

### Insert a struct

```go
_, err := orm.Insert(ctx, db, User{Name: "sam", Age: 18, Active: true})
if err != nil {
    log.Fatal(err)
}
```

### Insert and return a struct

```go
created, err := orm.InsertReturning[User](ctx, db, User{Name: "sam", Age: 18, Active: true})
if err != nil {
    log.Fatal(err)
}
_ = created
```

### Update selected columns on a struct

```go
_, err := orm.Update(
    ctx,
    db,
    User{ID: 1, Name: "alice", Active: true},
    orm.Columns("name", "active"),
    orm.WherePK(),
)
if err != nil {
    log.Fatal(err)
}
```

### Update by scope and return a struct

```go
updated, err := orm.UpdateByReturning[User](
    ctx,
    db,
    db.Table("users"),
    map[string]any{"active": true},
    func(q *query.Query) *query.Query {
        return q.Where("tenant_id", tenantID).Where("id", id)
    },
)
if err != nil {
    log.Fatal(err)
}
_ = updated
```

### Update a map with `Table(...)`, `PK(...)`, and `WherePK()`

```go
_, err := orm.Update(
    ctx,
    db,
    map[string]any{
        "id":     1,
        "name":   "alice",
        "active": true,
    },
    orm.Table("users"),
    orm.PK("id"),
    orm.Columns("name", "active"),
    orm.WherePK(),
)
if err != nil {
    log.Fatal(err)
}
```

### Upsert a struct

```go
_, err := orm.Upsert(
    ctx,
    db,
    User{ID: 1, Name: "alice", Age: 31, Active: true},
    orm.WherePK(),
)
if err != nil {
    log.Fatal(err)
}
```

### Use the generic API inside a transaction

```go
err := db.TransactionContext(ctx, func(tx orm.Tx) error {
    user, err := orm.SelectOne[User](ctx, tx.DB, "SELECT id, name, age, active FROM users WHERE id = ?", 1)
    if err != nil {
        return err
    }
    _, err = orm.Update(
        ctx,
        tx.DB,
        User{ID: user.ID, Active: !user.Active},
        orm.Columns("active"),
        orm.WherePK(),
    )
    return err
})
if err != nil {
    log.Fatal(err)
}
```
