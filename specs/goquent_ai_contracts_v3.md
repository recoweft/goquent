# Goquent database operation and review contracts v3

Management ID: **GQ-AI-01**, [Issue #52](https://github.com/recoweft/goquent/issues/52).
This is **PR1 of 2**, a documentation-only inventory against main
`8c41e211336c97f9f33c950b3ebc302df05b90e3`. It introduces no runtime API, profile,
test harness, fixture format or safety guarantee. PR2 is a separate task after
PR1 merges into main. Quent/Rust is outside this work.

## 1. Reading this contract

**Current** means observed implementation behavior, with existing tests listed
below. **Required** means a contract for the named follow-up issue, not behavior
available today. Source inspection alone does not prove a runtime guarantee.
The [v2 status audit](goquent_ai_safe_orm_roadmap_v2.md#04-implementation-audit-gq-ai-01pr1)
supersedes the historical unchecked task lists as a current implementation summary.

There are four independent stages: plan generation, policy/risk inspection,
execution gating, and result inspection. A path implementing one does not imply
the others. Scanning is not tenant authorization or an affected-row bound.
Static review is a separate path and does not mediate runtime execution.

## 2. Current public database entry points

Names below are exact public symbols; generic type parameters are omitted.
`Query` means `orm/query.Query`; other helpers are in `orm` unless qualified.
Context variants have the same inspection scope.

### 2.1 Query DSL

`db.Model`, `db.Table`, `db.TablePath` and `query.New` create a `Query`.
`New` accepts a structurally compatible executor even though its interface type
is unexported. `db.Table` passes the underlying executor, not the DB raw-SQL gate.

| Public entry | Plan | Policy/risk | Execution | Result inspection and exclusions |
| --- | --- | --- | --- | --- |
| `Query.Plan` | SELECT SQL, params, snapshot metadata; no DB call | Finalizes risk, registered table policy and required predicates | None | No live schema, EXPLAIN or authorization |
| `First`, `FirstMap`, `Get`, `GetMaps` | Calls `Plan` | Same as above | `ensurePlanExecutable`, then executor query | Struct/map scan; first-row APIs do not establish uniqueness or automatically prove a single-row SQL bound |
| `Count` | Plans a copied aggregate builder | Same finalizer | Gate then QueryRow/Scan | Scans count; aggregate-only plans avoid missing-limit warning |
| `PlanInsert`, `PlanInsertBatch` | INSERT SQL/params, target columns, batch metadata | Finalizer; tenant/required-filter/soft-delete policy rules do **not** apply to INSERT | None | No inserted-value/tenant validation |
| `Insert`, `InsertBatch` | Corresponding plan above | Same | Gate then Exec | Returns `sql.Result`; no automatic expected-row check |
| `InsertGetId` (MySQL) | Via `Insert` | Same | Via gated `Insert` | `LastInsertId`; no authorization inferred from ID |
| `InsertGetId` (PostgreSQL) | `PlanInsert`, then appends RETURNING configured `PrimaryKey` (default `id`) | Plan finalized before RETURNING | Direct QueryRow; this branch omits `ensurePlanExecutable` | Scans ID; complete final-SQL inspection is missing |
| `InsertOrIgnore` | Internal INSERT plan, `insert_mode=ignore` | INSERT finalizer | Gate then Exec | `sql.Result`; conflict outcome not classified |
| `Upsert` | Internal plan is `OperationInsert`, with unique/update column metadata | INSERT finalizer, no update-branch tenant/range proof | Gate then Exec | `sql.Result`; insert vs update not proven |
| `UpdateOrInsert` | Internal INSERT plan with condition/update columns | INSERT finalizer | Gate then Exec | Does not inspect a separate conditional UPDATE plan |
| `InsertUsing` | Internal INSERT SELECT plan records destination | INSERT finalizer | Gate then Exec | Source subquery policies/tables are not recursively covered by this plan |
| `PlanUpdate`, `PlanDelete` | SQL + copied builder condition/join metadata | Risk, table policy, required predicates; soft-delete predicate insertion | None | Flat predicate metadata is not a logical range proof |
| `Update`, `Delete` | Calls corresponding plan | Same | Gate then Exec | `sql.Result`; no affected-row expectation or rollback on cardinality violation |
| `Build`, `Dump`, `RawSQL` | SQL rendering only; `RawSQL` interpolates for debugging | No finalizer/gate; no promise to apply pending soft-delete predicates | None | Not an inspected executable artifact; may expose values |

Evidence: [query execution](../orm/query/query.go), [plans](../orm/query/plan.go),
[risk](../orm/query/risk.go), [policy](../orm/query/policy.go).
No public Query `PlanUpsert`, generic `PlanInsert`, or generic `Delete` helper
exists at this baseline; do not invent these as current APIs.

### 2.2 Generic, scoped, batch and composite operations

| Public entry | Plan | Policy/risk | Execution | Result inspection and exclusions |
| --- | --- | --- | --- | --- |
| `SelectOne`, `SelectAll`; compatibility `SelectStruct`, `SelectStructs`, `DB.SelectMap`, `DB.SelectMaps` | Raw plan through DB query | Raw risk gate, not semantic SELECT policies | DB raw query path | Generic struct/map/scalar scanning, bool options, not-found behavior; raw approval required |
| `PlanSelectBy`, `PlanUpdateBy`, `PlanDeleteBy` | Apply scopes to base Query, then plan | Query finalizer | None | Scopes mutate supplied query; no independent DB identity validation |
| `SelectOneBy`, `SelectAllBy` | Scoped SELECT plan | Query gate, then raw gate with internally supplied reason | Executes on separately supplied DB | Generic scanning; base Query and DB identity/dialect are not bound together |
| `UpdateBy`, `DeleteBy` | Scoped Query UPDATE/DELETE | Query finalizer/gate | Base Query executor | `sql.Result`, no generic write result options |
| `UpdateByReturning`, `UpdateByReturningWithOptions` | Scoped UPDATE plan; RETURNING appended **after** checking | Checks original plan; final RETURNING projection not rechecked | Trusted query on separately supplied DB | One-row scan, `NoRowsAs`; `ExpectAffected` not checked here |
| `Insert`, `Update`, `Upsert` | Private SQL builders, **no QueryPlan** | Shape/option checks only; `Update` requires `WherePK`; Upsert requires conflict target/PK | Trusted Exec, or trusted query with `Returning` option | Optional `ExpectAffected`/`NoRowsAs`; no policy/risk gate |
| `InsertMany`, `UpsertMany` | Private batch SQL builder, no QueryPlan | Shape/option checks, no row-by-row tenant proof | Same trusted execution | Same optional affected-row checks; no general chunking/retry contract |
| `InsertReturning`, `UpdateReturning`, `UpsertReturning` | Private builder with RETURNING, no QueryPlan | Shape/option checks only | Trusted query | First-row scan; `NoRowsAs` supported, `ExpectAffected` not checked |
| `InsertManyReturning`, `UpsertManyReturning` | Private builder with RETURNING, no QueryPlan | Shape/option checks only | Trusted query | All rows scanned; neither `ExpectAffected` nor `NoRowsAs` is applied |
| `InsertOnceReturning` | Private INSERT ON CONFLICT DO NOTHING RETURNING, then scoped SELECT plan if no inserted row | Insert has no plan/policy gate; lookup uses Query finalizer/gate and requires conflict columns or PK | Trusted insert; lookup uses generic SELECT with an internal raw approval reason | Value and inserted flag; not a general concurrency/replay guarantee |
| `ReplaceNestedCollection` | No whole-operation plan | Parent/child writes inherit generic gaps; cleanup uses `DeleteBy` | Caller-owned transaction boundary; can partially write before later error | Child IDs and grandchild input count, not whole-operation postconditions |
| `ReplaceNestedCollectionTx` | Same | Same | Sequence in `TransactionContext` | DB rollback on callback error; external callback effects are not rolled back |
| `RunIdempotentCommand` | No composite plan | Callback-dependent | Lookup, transactional Apply, optional conflict lookup | No automatic payload/key binding, concurrency classification or exactly-once guarantee |
| `RunTransactionWithHooks`, `InsertHook`, `InsertManyHook`, `NewTransactionHook` | No composite plan | Delegate to callback/helper paths | Main callback/hooks in transaction | Error propagation; external side effects/authorization remain caller responsibility |

Evidence: [writes](../orm/write.go), [reads](../orm/select.go),
[compatibility](../orm/compat.go), [scopes](../orm/scope.go),
[nested writes](../orm/nested_write.go), [idempotency](../orm/idempotency.go),
[hooks](../orm/transaction_hooks.go).
Generic RETURNING is PostgreSQL-only and rejects MySQL. Using `Returning` on a
`sql.Result` helper counts returned rows before affected-row checking; typed
RETURNING helpers take a different path. An affected-row error occurs after SQL
executes and does not independently roll back an autocommitted write.
Nested MySQL child IDs are inferred as first ID + index; PostgreSQL obtains
RETURNING rows and checks count/ID conversion. Neither implies portable ID order
or allocation guarantees under every server configuration.

### 2.3 Raw SQL, executors and transaction boundaries

| Public entry | Plan | Policy/risk | Execution | Result inspection and exclusions |
| --- | --- | --- | --- | --- |
| `DB.RawPlan`, `query.NewRawPlan` | OperationRaw with SQL/params | RAW_SQL_USED high, lexical weak/destructive checks; no semantic SQL reconstruction | None | Current precision starts as `precise`; this is **not** full raw analysis |
| `DB.Query`, `QueryContext`, `Exec`, `ExecContext` | Raw plan each call | Raw gate; `TouchedTables` annotates after finalization and is not policy proof | Requires current nonempty approval reason for high risk | Rows/result left to caller; no affected-row/tenant postcondition |
| `DB.QueryRowE` | Same | Same | Returns gating error before QueryRow | Caller scans; use to retain exact safety error |
| `DB.QueryRow`, `QueryRowContext` | Same | Same | Deprecated; rejection invokes executor with canceled context and harmless empty SELECT, not caller SQL | Cannot return exact gating error through `*sql.Row`; custom executor must honor context |
| `RequireRawApproval`, `TouchedTables` | Configure a shallow DB copy | Reason/table annotation, not external authorization | None themselves | Do not verify SQL table set or caller authority |
| `NewDB`, `NewDBWithExecutor`, `NewTxDB`, `WrapTx`, `Open`, `OpenWithDriver`, `OpenWithDriverOptions` | Construction only | No universal wrapper policy; subsequent path determines checks | Existing DB/Tx/custom executor or driver; Open connects/pings | Preserve dialect/scan behavior; wrapping does not inspect direct executor calls |
| `DB.Transaction`, `TransactionContext`, `Begin`, `BeginTx`; `Tx.Commit`, `Rollback` | No operation plan | Individual helper path decides checks | Owned DB starts transaction; external executor has no generic Begin capability | Commit/rollback errors; transaction alone is not inspection |
| `DB.SQLDB`, underlying `sql.Tx`/embedded driver Tx, `driver.Driver.DB`, direct `sql.DB`/`sql.Tx`/custom Executor calls | No Goquent plan | Outside Goquent interception | Direct database/sql | Caller-owned checks; cannot claim Goquent guarantees |
| `RegisterDriver`, `RegisterDriverWithDialect`, `RegisterDialect`, `GetDriver`; `driver.Open` and transaction methods | No plan | Infrastructure, not policy | Connection/transaction support | Driver compatibility does not certify query meaning |

Evidence: [DB/Executor](../orm/orm.go), [driver](../orm/driver/driver.go),
[driver registry](../orm/driver_registry.go). `NewDBWithExecutor`/`NewTxDB` do not
own/close the supplied executor; `NewDB` wraps an owned `*sql.DB`. `WrapTx` retains
the source driver pointer; its `SQLDB`/`Close` do not have the ownership semantics
of `NewTxDB`.

### 2.4 Construction, review and other public surfaces

All chain modifiers (selection, aggregates, joins/lateral/subqueries/unions,
Where/OrWhere groups/NOT/IN/NULL/ranges/column comparisons, date/time, JSON/text
search, cursor, order/group/having, limit/offset and locks) inherit the terminal
path above; none independently executes or proves scope. `SelectRaw`, raw
predicates, `SafeWhereRaw`/`SafeOrWhereRaw`, raw ordering/having, expression cursor
columns, `ProjectionSQL`/`ApplyProjection`, `SetRaw`/`SetExpr`/`SetColumn`/`Increment`
and raw conflict targets are SQL expression surfaces. Syntax validation and
parameter binding do not establish logical safety. The `Safe` prefix is not a
tenant or row-range proof. Write options (`Columns`, `Omit`, `PK`, `WherePK`, table
and schema options, conflict options, RETURNING and row expectations) change only
the documented consuming helper path.

Scope combinators, `RequireTenantScope`/`RequirePredicates`, policy registration,
`AccessReason`, `WithDeleted`/`OnlyDeleted`, `RequireApproval`, suppressions and
`NewRiskEngine` configure inspection; they do not authorize business operations.
`EnsurePlanExecutable` consumes already-populated plan fields, accepts nil, and
does not re-derive SQL semantics. `PlanHasPredicateColumn` and
`MissingRequiredPredicates` inspect column presence, not logical implication.
In-memory projection/hydration/grouping, model conversion, scanner APIs, JSON,
nullable/numeric/bool helpers do not execute SQL or impose policy on supplied rows.

| Surface | Plan and policy | Execution/result boundary |
| --- | --- | --- |
| `CompileOperationSpec`, `ValidateOperationSpec`, `operation.Compile/Validate` | Single-model read-only SELECT; manifest fields/forbidden fields, required filters, PII reason, value refs, ordering and limit checks | No DB execution. Filter presence is not trusted tenant equality; absent verification is not rejected merely by `RequireFreshManifest` |
| `review.Run`, CLI `review` | Go AST patterns, SQL, QueryPlan/MigrationPlan JSON; optional manifest/config | No query execution; name-based reconstruction has omissions/false positives; partial/unsupported must stay visible |
| `GenerateManifest`, `LoadManifest`, `ValidateManifest`, `VerifyManifest` and manifest equivalents | Version 1, schema/policy/code/database fingerprints and supplied evidence | Not authorization. Verification compares snapshots, not live DB unless explicitly obtained |
| `NewMigrator`, `PlanMigrationSQL`, `PlanMigrationSteps`, `DiffSchemas`; migration plan/diff APIs and CLI plan/dry-run | Separate migration risk/precision/approval path; parser/schema limits | Plan/dry-run do not execute. `Migrator.Apply` is explicit human-controlled execution, not CRUD policy enforcement |
| `migration.ReadSchema`, `ReadStatus` and CLI live-schema operations | Explicit DB reads through supplied StatusExecutor, separate from pure planning | Scan catalog/status data; no CRUD policy gate. `CompareSchemaDrift` compares supplied schemas without DB execution |
| MCP resources, tools, prompts | Manifest, raw/migration review, OperationSpec planning | No DB writes. `explain_query` produces a raw plan, not live EXPLAIN; fixture/skeleton tools return text |

## 3. Responsibilities required of later work

### 3.1 Compatibility and Strict profiles

Current Goquent has policy modes `warn`, `enforce`, `block` and a global risk
engine/registry. It has **no unified compatibility/Strict profile API**. These
terms describe a required migration boundary, not new options introduced here.

Compatibility must preserve existing MySQL/PostgreSQL, custom Executor/sql.Tx,
scanning and BoolCompat behavior unless a change is explicitly documented.
`BoolStrict` is a scanning choice, unrelated to the proposed safety Strict profile.
Strict must refuse to treat unverified, unsupported or context-missing operations
as executable/verified. Missing current tenant, necessary schema or a trusted
high-risk authorization provider must not be bypassed by a reason string.
Strict is not weakened to match existing compatibility behavior.

Each intentional change needs the case ID below, old/new determination, owning
issue, compatibility impact, migration instructions and regression coverage.
Introduce/version the new contract explicitly; do not silently reinterpret old
JSON as authorized. Keep legacy behavior and limitations visible during staged
adoption. Unsupported dialect semantics report limits, not successful inspection.
Equal operation meaning should satisfy equal safety requirements across dialects;
identical SQL strings are not required.

### 3.2 Condition representation — GQ-AI-02

One internal representation must generate the executed SQL/values and inspected
conditions. Preserve AND/OR/NOT nesting, aliases/table identity, operators, bound
values, NULL semantics, key metadata, subqueries and raw nodes. Do not infer
single-row scope from an `id` name, occurrence of key columns, an OR branch, a
join equality or column-to-column comparison. Prove scope for every possible
affected row, or report unknown/broad. SQL three-valued logic, nullable unique
keys and dialect-specific expressions constrain proofs. No new tree type or
proof algorithm is specified as an implemented API here.

### 3.3 DB settings and trusted context — GQ-AI-03

DB-scoped policy/risk/profile settings must be isolated and propagate explicitly
to queries, copies and transactions, without cross-DB global mutation. Current
registry and `query.DefaultRiskEngine` are process-wide. Trusted tenant context
must originate outside user-controlled operation JSON; mere presence of a tenant
column/value_ref is insufficient. Unknown schema/policy freshness must remain
unknown. Legacy global configuration requires a documented migration route;
concurrency and transaction inheritance tests belong to 03.

### 3.4 Plans, execution and results — GQ-AI-04/05/10/12

All public CRUD paths, including generic, batch, RETURNING, conflict branches and
nested operations, must expose/consume consistent inspected operation data (04).
Capture final projection/conditions/params before approval; appended SQL needs
reinspection. Preserve executor/context/dialect identity. Composite plans must
state atomicity and callback limitations.

Plan version, dialect, configuration/schema context and correspondence to the
executed operation belong to 05. Any change to values, predicates, projection,
target or execution context invalidates inspection. Input JSON verdicts,
warnings, suppressions and approval fields are untrusted claims. Recompute from
trusted context; `EnsurePlanExecutable` alone is not artifact validation.
Plan generation stays DB-free; live schema and EXPLAIN are separate explicit
reads (11/12). Estimated rows and index fields are not measured evidence merely
because fields exist in QueryPlan.

Result inspection (10) must distinguish execution failure, conflict/replay,
unknown affected rows, too few/many rows and decoding errors. Record which checks
occur before/after SQL and whether a transaction actually rolled back. MySQL
changed/matched-row and upsert counts differ from PostgreSQL; do not equate all
counts or assume a returned first row proves exactly one affected row.
Idempotency requires operation/key/payload context and conflict semantics, not
just a uniqueness exception or pre-read. External hook effects are outside DB
rollback. Observation (12) must not leak execution values.

### 3.5 Diagnostics, public data and authorization — GQ-AI-06/07/09/13

Diagnostics must distinguish code, severity, precision, source, safe evidence,
suppression and why analysis was incomplete. `precise` describes supported
reconstruction, never authorization. Static type identity and unsupported coverage
need improvement (09); typed OperationSpec validation and structured errors
belong to 07.

Public plan/diagnostic/manifest/MCP data must be a separate non-executable view
(06). Current `ToJSON`, `String`, interpolated SQL, free-form metadata, reasons,
evidence and errors may contain sensitive values. Redact secrets and literals
across outputs, not only `Params`; masking must never flow back into execution.
Retain useful operation shape and analysis limits without claiming complete
inspection or revealing confidential values.

Reasons record intent; external permission proves authorized execution (13).
Current approval is a nonempty reason plus optional expiry, not a verified
principal/token. Suppression hides eligible diagnostics; it does not grant
permission. Until external high-risk authorization is implemented and configured,
Strict must fail closed. Token/provider API design is left to 13.

## 4. Shared cases (documentary specification, not new tests)

PR2 adds the [shared case register and validation guide](../tests/contracts/README.md).
It mirrors this table and distinguishes executable evidence from documentary gaps.

Use stable IDs when follow-up implementations intentionally change a verdict.
Unless stated otherwise, inputs are Query DSL operations on `users`, default risk
engine, no suppression/approval, and no policy. For tenant cases use `tenant_id`
required in block mode, current trusted tenant `T1`; schema assertions in the
required column refer to trusted schema. “Allowed” means the current gate does
not reject, never business authorization. Current statements are source-derived;
only existing cases named in section 5 have executable evidence.

| ID / input | Current determination | Required determination / owner and reason for change | Analysis limit |
| --- | --- | --- | --- |
| C01 SELECT id,name WHERE id=10 LIMIT 1 | Low plan; no execution during Plan | Preserve pure planning (01/04) | LIMIT bounds output, not authorization |
| C02 SELECT * with no limit | SELECT_STAR_USED and LIMIT_MISSING, medium | Preserve diagnostics (04/09) | No DB cost estimate |
| C03 UPDATE/DELETE with no predicate, no soft-delete policy | Blocked, non-suppressible missing-WHERE warning | Preserve block (02/04) | Default soft-delete predicate changes input; not a narrow-write proof |
| C04 UPDATE WHERE tenant_id=T1, no key | Medium bulk warning, allowed absent other policy violations | Tenant-wide, never prove one row (02/03) | Correct tenant still permits many rows |
| C05 UPDATE WHERE id > 0 or id IN (1,2) | `id` occurrence suppresses bulk warning | Broad/unknown, not single-row (02) | Column-name/operator-insensitive heuristic is insufficient |
| C06 UPDATE WHERE id=1 OR status='active' | `id` occurrence can suppress bulk warning | Every OR branch must satisfy scope; broad/unknown otherwise (02) | Flattened predicates lose implication |
| C07 UPDATE WHERE (tenant_id=T1 AND id=1) OR id=2 | Tenant presence can satisfy policy; ID heuristic narrows risk | Reject unbound branch in Strict (02/03) | OR must not escape tenant restriction |
| C08 tenant_id=T2, tenant_id!=T1, NOT(tenant_id=T1), or tenant_id=other_column | Column presence can satisfy required/tenant checks | Reject incorrect/unknown binding in Strict (02/03) | No proof of equality to trusted value |
| C09 Tenant-scoped read without tenant context but literal tenant filter | Presence check can pass | Unverified/rejected in Strict until trusted tenant supplied (03) | Application JSON is not trusted context |
| C10 Composite unique key (tenant_id,external_key), range predicates mentioning both | Attached key metadata can suppress bulk warning on presence alone | Require conjunction/equality/full unique key (02) | Nullability, partial/expression indexes, dialect semantics matter |
| C11 NOT(id=1), id IS NOT NULL, raw expression or EXISTS child query | SQL/snapshot produced; heuristics may allow; precision need not degrade | Preserve condition meaning or mark unknown; no verified scope in Strict (02/09) | General SQL theorem proving is not promised |
| C12 Raw DELETE FROM users via DB.Exec | RAW_SQL_USED high; no approval rejects; reason permits raw gate | Unknown semantics not verified by reason; trusted high-risk authorization required in Strict (04/13) | No full SQL parser; TouchedTables does not fix this |
| C13 Generic Update with WherePK or generic Upsert with conflict target | Shape checks/trusted execution, no common plan/policy | Common inspection for final operations/conflict branches (04) | PK option is not tenant binding |
| C14 Batch containing T1 and T2 rows | Generic batch/DSL INSERT lack per-row tenant-value inspection | Inspect all rows/branches with trusted context (03/04) | Chunking/partial execution must be explicit |
| C15 PostgreSQL InsertGetId / UpdateByReturning | Plan exists but gate omitted in first path; RETURNING appended after checking in both | Inspect/gate final SQL and projection (04) | MySQL uses LastInsertId; generic RETURNING rejected |
| C16 Typed RETURNING with ExpectAffected(1), DB returns multiple rows | First-row typed helper ignores expected count; ManyReturning scans all | Inspect cardinality across paths; surface rollback status (04/10) | RETURNING rows and driver RowsAffected differ |
| C17 Generic write ExpectAffected(1), actual=2 | RowsAffectedError after execution | Preserve error, define transactional rollback contract (10) | Autocommit may already have changed data |
| C18 Nested replacement, later callback fails | Non-Tx can leave prior writes; Tx rolls DB back | Composite plan and explicit atomicity/result contract (04/10) | ID assignments/external effects persist |
| C19 Mutate plan params/target or supply JSON low/precise/approval/warnings | Mutable plan; EnsurePlanExecutable trusts fields; review can consume supplied warnings | Revalidate final operation and untrusted artifact (05/09) | Serialized verdict is not evidence |
| C20 Serialize secret in params, SQL literal, reason or metadata | Values may appear in output | Non-executable redacted public view (06) | Removing Params alone is incomplete |
| C21 OperationSpec unresolved value_ref / unsupported write | Missing ref and non-select operation rejected | Preserve narrow read-only scope; reject invalid values (07) | Present ref values still need type/trust validation |
| C22 OperationSpec manifest has no Verification, RequireFreshManifest=true | Compile only checks attached stale verification; absence can pass | Missing/unverified required context rejected in Strict (03/07) | CLI review fresh gate is a different, stronger path |
| C23 review --require-fresh-manifest missing/stale/unverified | Gate failure | Preserve explicit verification requirement (05/11) | Supplied fingerprints do not prove live DB equality |
| C24 Dynamic helper/raw SQL or unrelated method named Update in Go source | Partial/unsupported, omissions or false positives possible | Type-aware identification; preserve uncertainty (09) | No whole-program/control-flow proof |
| C25 InsertOnceReturning/RunIdempotentCommand concurrent replay | Existing-row/conflict recipes | Define operation/key/payload and retry/result semantics (10) | Existing tests do not prove exactly-once effects |
| C26 Two DBs register different policy for same table | Shared process-wide registry | DB isolation and transaction inheritance (03) | Mutex protection is not tenant isolation |
| C27 sql.Tx/custom Executor wrapped by NewTxDB/NewDBWithExecutor | Delegated path works; no new-transaction capability implied | Preserve interface/ownership; bind context for inspected paths (03/04/05) | Direct executor/SQLDB access is outside interception |
| C28 MySQL/PostgreSQL equivalent filtered operation | Distinct placeholders/conflict/RETURNING behavior | Same semantic safety requirement, dialect-specific limits (02/04/10) | Do not demand identical SQL/counts |

New executable guarantees belong in their owning issue. PR2 may share documentary
cases and baseline fixtures after PR1 merges; it must not freeze gaps as desired
safety or add deliberately failing tests. Every changed verdict should retain its
case ID and explanation above.

## 5. Existing evidence and fixture compatibility

| API/review area | Existing evidence to run | What it does not establish |
| --- | --- | --- |
| DSL plans/SQL/params/no execution | `orm/query/plan_test.go`, `risk_test.go`, `policy_test.go`; internal builder tests | Full condition proofs, terminal-path parity |
| Raw DB and Executor | `orm/orm_raw_test.go`; `tests/raw_sql_test.go`, `custom_driver_test.go`, `transaction_context_test.go`, `manual_transaction_test.go` | Universal interception or external approval |
| Generic/scoped CRUD, RETURNING and bool | `orm/write_test.go`, `scope_test.go`, `select_test.go`, bool tests; `tests/write_generic_test.go`, `select_generic_test.go`, `orm_postgres_test.go` | Shared plan/policy or uniform cardinality checks |
| Nested/idempotency/hooks | `tests/nested_write_test.go`; `orm/idempotency_test.go`, `transaction_hooks_test.go` | Arbitrary replay safety, callback atomicity or all MySQL ID allocation modes |
| Plan JSON and diagnostics | `TestSelectPlanSnapshot`, `TestRunReviewsRawSQLAndQueryPlanJSON`, `TestRunReviewsMigrationPlanJSON`, `TestRunReviewsSuppressedWarningsFromQueryPlanJSON`, `TestWriteJSONAndPretty`; `cmd/goquent/main_test.go` | Versioned executable artifacts or redaction |
| Manifest/OperationSpec/MCP | `orm/manifest/manifest_test.go`, `orm/operation/spec_test.go`, `orm/mcp/server_test.go`, CLI tests; `examples/ai-safe-orm/*.json` | Trusted tenant context or freshness without current evidence |

The [PR2 compatibility harness](../tests/contracts/compatibility_test.go) reuses the
example files and connects selected plan/diagnostic/load/verify paths. Its
[coverage register](../tests/contracts/testdata/cases.json) maps the public API
families and case IDs to existing tests; mappings alone are not execution evidence.

Plan/diagnostic fixtures are largely inline Go literals/assertions or temporary
JSON produced by existing tests; there is no separate shared golden-file harness
to add in PR1. Preserve those sources and the checked-in example manifest,
schema, policies, operation and values byte-for-byte. Compare to the audit
baseline and run existing serialization/review/load tests. Do not claim complete
wire compatibility solely from passing assertions: they cover selected fields,
not every byte/consumer. Record actual validation outcomes in the PR/report.

## 6. Follow-up ownership (plans, not implementation in PR1)

These are management IDs, not inferred GitHub issue numbers.

| ID | Priority / PRs | Depends on | Required work |
| --- | --- | --- | --- |
| GQ-AI-01 | P0 / 2 | None | PR1 inventory/contracts; PR2 shared fixtures after PR1 main merge |
| GQ-AI-02 | P0 / 2 | 01 | Condition tree and write-scope proofs |
| GQ-AI-03 | P0 / 2 | 01,02 | DB-scoped settings and trusted tenant binding |
| GQ-AI-04 | P0 / 3 | 02,03 | Public CRUD plan/inspection/execution parity |
| GQ-AI-05 | P0 / 2 | 01,04 | Plan versions and execution correspondence |
| GQ-AI-06 | P0 / 2 | 01,04 | Redacted public plans, diagnostics and manifests |
| GQ-AI-07 | P1 / 2 | 02,03,05,06 | Typed OperationSpec validation and diagnostics |
| GQ-AI-08 | P1 / 2 | 04,07 | Typed columns and update code generation |
| GQ-AI-09 | P1 / 3 | 02,04,05,07 | Static type identity and precision |
| GQ-AI-10 | P1 / 3 | 04,05,06,07 | Replay, concurrency and affected-row contracts |
| GQ-AI-11 | P1 / 3 | 01,05,06 | Schema diff and migration evidence |
| GQ-AI-12 | P2 / 2 | 04,06 | Performance plans and execution observation |
| GQ-AI-13 | P1 / 2 | 03,05,06 | Reasons separated from external permission |
| GQ-AI-14 | P1 / 2 | 01–13 | Safe examples and staged release validation |

No complex OperationSpec language, natural-language DB execution, MCP writes,
custom DB driver implementation or Rust port is introduced. Track documented/
implemented, tested, PR-created and merged states separately. PR creation is not
main integration and does not close Issue #52.
