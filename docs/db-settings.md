# DB settings and application execution context

GQ-AI-03/PR1, [Issue #60](https://github.com/recoweft/goquent/issues/60).
This foundation isolates configuration and retains application data. It does not
implement tenant authentication, trusted-value predicate proofs, unified Strict,
or inspection of all public CRUD. Those remain PR2 and GQ-AI-04.

## Immutable snapshots

`orm.NewPolicySet` constructs an immutable set of `TablePolicy` values. An empty
set explicitly means no table policies. `orm.NewSettings` combines a PolicySet,
RiskConfig and ExecutionContext without consulting globals. Settings and PolicySet
have private storage; copying them shares only immutable data. `Policies`,
`PolicyForTable`, `RiskConfig` and execution-context `Input` return detached data.
`NewRiskEngine` now also owns its RiskConfig map and all four pointer-valued rule
fields. Mutating input or returned values does not change existing snapshots.
Callers must not mutate inputs concurrently with the operation copying them.

Use `WithSettings`, `WithPolicySet`, `WithRiskConfig` and `WithExecutionContext`
options with existing constructors. Options apply in order: the last explicit
setting wins. PolicySet replaces the entire set, not individual global entries.
`WithRiskConfig` captures the input when the option is created, so later use of
that option cannot accidentally import a changed map or pointer.

```go
policies, err := orm.NewPolicySet(orm.TablePolicy{
    Table: "orders", TenantColumn: "tenant_id", TenantMode: orm.PolicyModeBlock,
})
if err != nil { return err }
execution, err := orm.NewExecutionContext(orm.ExecutionContextInput{
    Source: "authenticated application request", TenantPresent: true,
    CurrentTenant: tenantFromApplication,
})
if err != nil { return err }
db, err := orm.OpenWithDriverOptions(orm.Postgres, dsn,
    orm.WithPolicySet(policies), orm.WithRiskConfig(orm.RiskConfig{}))
if err != nil { return err }
requestDB := db.WithOptions(orm.WithExecutionContext(execution))
// Queries and transactions started from requestDB retain this context.
// The application still supplies query predicates; PR1 does not bind the tenant.
_ = requestDB
```

Keep the connection DB without request data; derive a request DB after the
application authenticates the request. Deriving a different context never changes
existing DBs, queries or transactions. Reusing the same request DB for another
request intentionally reuses its data, so create a fresh derivation per request.
No session, transaction or process global stores execution context.

## Execution context is data, not proof

`ExecutionContextInput.Source` is the application's provenance label. It is not
validated identity. `TenantPresent` distinguishes missing tenant data from an
explicit nil/empty value. The zero value is missing context; empty strings, nil,
JSON values, source labels and types are never promoted to verified context.
There is no authenticated/authorized/tenant-verified verdict field. The ORM does
not authenticate the caller. A tenant value with `TenantPresent=false` is rejected
as inconsistent. An explicit nil/empty value is retained as unconfirmed data.

`NewExecutionContext` reuses the GQ-AI-02 bounded value copier and never invokes
Valuer, marshaler, Stringer or other user methods. Supported exact types are nil,
bool, string, signed/unsigned integers except uintptr, float32/64, time.Time,
slices of these non-nil scalar types (including []byte), []any and map[string]any
whose descendants are supported. Named custom types, arbitrary structs/pointers,
functions, other map types and unsupported descendants are rejected. Shared DAGs
are copied with memoization; cycles, depth over 64, slot-budget over 65,536, or
copy-budget over 8 MiB are rejected. Budgets are allocation guards, not exact heap
or string-length limits; see `orm/internal/valuecopy`. Failure returns
`ErrUnsupportedExecutionContext` and retains no partial snapshot/reference.

This is storage support, not a promise that every supported value can serve as a
SQL tenant scalar. PR2 must define acceptable tenant comparisons and reject
missing/unconfirmed/unprovable bindings under Strict. Context is available through
`db.Settings().ExecutionContext()` and `q.Settings().ExecutionContext()`; it is
not placed in public plan metadata or OperationSpec JSON as inspection evidence.

## Legacy migration and deliberate compatibility changes

1. Register legacy `orm.Model(...).TenantScoped(...)`, `RegisterTablePolicy`, etc.
   before constructing DBs. New DB construction snapshots the global table
   registry and the built-in `query.DefaultRiskEngine` configuration. For Open,
   this happens after connection/ping, when the DB wrapper is created.
2. Later global registration/reset/engine replacement affects future DBs, not
   existing DBs or their future queries/transactions. Previously, finalization
   consulted the live registry and engine, so late registration could change
   existing queries (including joined-table policy checks). That behavior is
   intentionally removed. Standalone `query.New` snapshots at query creation;
   `query.NewRawPlan` snapshots at its plan call.
3. Prefer explicit per-DB options for multi-DB/multi-request applications. Explicit
   zero `Settings{}`, empty PolicySet or empty RiskConfig overrides defaults;
   there is no fallback to globals when a local policy lookup misses.
4. To deliberately re-import legacy defaults, derive a new DB with
   `db.WithOptions(orm.WithSettings(orm.SnapshotDefaultSettings()))`. This replaces
   all settings, including resetting execution context to missing. Supply a new
   explicit context afterward if needed. Existing DBs/queries/Tx are unchanged.
5. The exported `query.DefaultRiskEngine` variable still supports legacy assignment
   during single-threaded initialization. Caller writes to a public variable
   cannot be synchronized by the library: do not assign it concurrently with
   construction/snapshot imports or direct legacy engine consumers. Registry
   registration itself is mutex-protected. Concurrent application changes should
   use new immutable snapshots and DB derivations instead.

Arbitrary custom RiskEngine implementations cannot be cloned without invoking
user code or sharing unknown mutable state. Default snapshots record
`ErrUnsupportedRiskEngine` for such an engine (including nil). Inspect
`db.Settings().Err()` after non-error-returning construction. Query planning and
DB raw planning return this error; standalone NewRawPlan, whose signature has no
error, returns a non-suppressible blocked plan. No custom CheckQuery method is
called on these paths. Explicit WithRiskConfig or WithSettings replaces the
unsupported default; it is never silently replaced with built-in defaults.
Constructors retain their signatures and connection call timing. This is an
intentional compatibility restriction. Generic paths outside the plan pipeline
remain outside this check as well; this is not universal execution gating.
Custom engines can still be used directly through their existing interface.
`orm.DefaultRiskEngine` is historically a separate interface-variable copy of
`query.DefaultRiskEngine`, not an assignable alias; assigning the facade variable
has never configured Query finalization. Use explicit DB options for configuration.

## Inheritance and ownership

| Entry | Settings/context source | Executor and ownership |
| --- | --- | --- |
| Open/OpenWithDriver/OpenWithDriverOptions, NewDB | Legacy snapshot, then explicit options | Existing connect/ping/pool/Close behavior |
| Clone, value-copy of DB, WithOptions, RequireRawApproval, TouchedTables | Source DB snapshot, then explicit options where accepted | Same executor/ownership; closing an owning copy closes the shared pool |
| Begin/BeginTx, Transaction/TransactionContext | Parent DB snapshot | Existing Begin/commit/rollback/callback timing |
| WrapTx | Parent DB snapshot, then options | Legacy source driver/pool retained; SQLDB/Close semantics unchanged |
| WrapExecutor | Parent DB snapshot, then options | External executor, no pool ownership, Close no-op, no Begin capability |
| NewTxDB/NewDBWithExecutor | Legacy snapshot, then explicit options | No parent inference, external executor remains caller-owned |
| Table/TablePath/Model and query groups/Count's copied builder | Originating DB/Query snapshot | Existing query executor and dialect |
| query.NewWithSettings/NewRawPlanWithSettings | Explicit settings only | No global lookup |

For a standalone external transaction, explicitly supply
`orm.WithSettings(parent.Settings())` or use the parent's WrapTx/WrapExecutor.
Do not infer a parent from *sql.Tx or a custom Executor. Nested calls preserve the
existing driver behavior; no savepoints or nested-transaction semantics are added.
A DB begun from a Tx wrapper may still begin via its retained pool; that is not a
child savepoint. `context.Context` cancellation and its values remain caller-owned
and separate from the immutable ExecutionContext; no values are inferred from it.

## Inspection coverage and deferred work

DB Model/Table/TablePath supply snapshots to the existing Query finalizer:
SELECT/Count and the existing insert/update/delete plans use local risk config
and table policies, including joined tables. Soft-delete predicates use the same
local policy snapshot. Scope helpers retain the base Query snapshot. DB RawPlan
and raw gates use local risk config; TouchedTables remains a post-finalization
annotation, not semantic policy inspection. Existing tenant/required-filter
checks still inspect column presence; they do not compare the retained tenant.

OperationSpec compilation has no DB argument and continues to use standalone
query construction plus supplied manifest checks. Static review/MCP and explicit
engine/registry APIs remain separate consumers; some review paths directly read
the global risk engine. They do not acquire a DB/request context. Direct executor,
SQLDB/driver calls, generic trusted writes, composite operations and incomplete
RETURNING paths retain the exclusions in contracts v3. Supplying settings does
not mean these paths are policy-enforced. A separately supplied scoped Query and
DB are still not bound by identity; GQ-AI-04/05 own that integration.

PR2 owns all-branch trusted tenant equality, OR/NOT/NULL/column-comparison meaning,
outer-AND automatic predicates, alias/join/subquery/CTE proof limits, INSERT/UPDATE/
UPSERT tenant semantics, immutable columns, PII/RETURNING and logical deletion.
GQ-AI-04 owns all public CRUD enforcement. Unified Strict is still unimplemented.
Its required contract remains fail-closed for missing/unverifiable context and
non-bypassable tenant violations; reason strings and suppressions cannot supply
external authorization. Compatibility policy modes keep their existing heuristic
meaning. BoolStrict is a scanning mode and does not implement safety Strict.

Regression evidence is in `orm/settings_test.go`, `orm/query/settings_test.go`
and `tests/settings_test.go`; C26/C27 in the shared case register describe the
bounded guarantees. Consult the PR/report for actual commands, failures, skips,
server versions and CI head; test definitions are not execution evidence.
