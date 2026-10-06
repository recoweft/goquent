# Conditional tenant and policy inspection

GQ-AI-03/PR2, [Issue #60](https://github.com/recoweft/goquent/issues/60).
This opt-in gate applies to Query and the generic/scoped paths in the current
[GQ-AI-04 entry inventory](planned-query-execution.md). The table below records
the historical PR65 baseline. It is not a unified
Strict profile, authentication service, universal CRUD gate, or ExecutionPermit.

## Application supply and migration

1. Keep `NewExecutionContext` for unconfirmed data retention. Its signature,
   copying budgets and `Input` contract remain unchanged. Source, TenantPresent,
   a plausible value, a type, JSON and manifest verdicts do not establish trust.
2. After authentication **and authorization**, trusted server code calls
   `NewApplicationTenantContext` with the authorized current tenant. Never call
   this merely because a request supplied a tenant. A private bit records this
   supply path; it is not cryptographic proof or protection against malicious Go
   code. Source may be empty: a label is not authentication.
3. Separately construct `NewApplicationSchema(ApplicationSchemaInput{...})` for
   the executor/database being configured. Supply Database, the known dialect,
   exact table/schema paths, column inventory and SQL integer type facts. Set
   `PlainTable` only for a base table with no triggers, rules or generated columns
   rewriting supplied writes or introducing hidden conflict behavior. Views and
   rewrite semantics are unsupported. This is an application assertion, not a
   database read. Do not import operation JSON, old WriteKeyContext, plan verdicts
   or manifest Verification into this supply boundary.
4. Derive the request DB with `WithExecutionContext(context)` and
   `WithTenantPolicy(databaseIdentity, schema, automatic)`. Both the independent
   database identity and schema identity must match, as must the actual configured
   dialect. These string/type checks do not attest the live connection. Reusing a
   correctly labeled assertion against a different physical DB is an application
   error Goquent cannot detect. Use full schema paths where resolution depends on
   search_path/default schema; the application asserts that resolution too.
5. `automatic=false` requires explicit equality predicates and INSERT tenant
   values. `automatic=true` adds the base table's tenant equality outside a group
   containing the entire caller WHERE, and fills only missing INSERT tenant values
   in detached rows. Supplied mismatches are refused, never overwritten. JOIN ON
   predicates remain explicit. Re-plan after changes; do not execute public JSON.

The existing immutable Settings machinery carries schema, strict configuration,
provenance and tenant value through DB derivations, queries, parent transactions,
WrapTx and WrapExecutor. Explicit old/empty context replacement clears provenance
with the value. Input -> NewExecutionContext reconstruction is unconfirmed.
Standalone external Tx/Executor still requires explicit Settings; no parent is
inferred. Schema constructors copy maps/slices and impose limits: 64 tables,
256 columns/table, 4096 total columns, 64 constraints/table, 256 key columns/table,
and bounded identifier/type strings. Context and predicate value budgets remain
those documented in db-settings.md and predicate-tree.md.

## Conditional result and comparison domain

`QueryPlan.TenantPolicy` reports `conditional_pass` only for a private builder
plan and the explicit application supplies. `schema_freshness`, `policy_freshness`
and `executor_identity` remain unknown/unverified. Required facts that are absent,
inconsistent or unsupported cause an error or `unknown_or_rejected` blocked plan.
Neither a low risk level nor precise analysis means authorization. Errors identify
reasons and, for missing bindings, target/alias; they do not interpolate values.
Other legacy plan output can still expose values (GQ-AI-06 remains unfinished).

Tenant columns support nonnullable **signed** SQL SMALLINT/INT/BIGINT on MySQL
and smallint/integer/bigint on PostgreSQL, with exact declared 16/32/64-bit widths.
Exact built-in Go integer types, including unsigned inputs fitting the signed
range, compare numerically through database/sql integer semantics. Named custom
types, pointers, strings (including numeric strings), bools, floats, time, nil,
containers, out-of-range values and unsigned SQL columns do not prove equality.
Zero is a valid integer tenant. No Valuer, Stringer or marshaler is called to
compare, inspect or fill values. String collation and arbitrary implicit casts
are deliberately outside this domain. Ordinary non-key columns can have other
type labels, but strict write payloads are limited to built-in scalar values.

The application asserts these facts are true for its executor and that the
executor preserves database/sql parameter semantics. The ORM does not verify
live schema, uniqueness completeness, freshness, current database identity,
triggers or changes since supply. Integration tests validate only their fixtures.
No schema discovery or EXPLAIN occurs during Plan.

## Predicate and JOIN semantics

Equality leaves to the current tenant prove a binding. AND needs one independently
binding child; OR needs every child to bind. Groups preserve grouping. NOT, !=,
ranges, IN (even singleton), NULL checks and column comparisons never create a
tenant proof. An outer AND equality can restrict a supported NOT/range/other-tenant
predicate without using it as proof. Contradictory predicates need no satisfiability
solver. SQL NULL/unknown cannot add rows outside an independent true equality.
Raw/opaque/subquery conditions remain unknown even beside an outer equality.

Each table instance resolves independently by its alias. Unqualified references
must resolve to exactly one column in the supplied inventories. All joined tables,
including those without a policy, require schema context. Same-table aliases do
not share bindings. INNER and LEFT SELECT joins support simple column equality ON
or an AND-only JoinClause of known column/comparison predicates. Joined tenant
binding can occur in that join's ON or in the caller WHERE. Base binding must be
in WHERE. ON predicates are never moved to WHERE; unmatched LEFT rows remain.
A caller's own WHERE on a nullable side still has its ordinary SQL meaning.

JOIN disjunctions, RIGHT/CROSS/lateral joins, write joins, subqueries, CTE/Raw,
unions, HAVING, raw projection/order, unknown columns/expressions and joined-table
soft-delete policies are conservatively refused. General SQL theorem proving is
not implemented. SELECT aliases/expressions and wildcard projection are refused;
explicit known columns and COUNT(*) are supported. These refusals are coverage
limits, not evidence that the rejected SQL is inherently unsafe.

Base-table soft-delete filters also use an outer AND on a detached builder.
WithDeleted omits the default filter; OnlyDeleted uses IS NOT NULL. Neither
removes tenant binding, changes DELETE into UPDATE, nor proves a single-row bound.
Repeated planning and later OR additions regenerate mandatory conditions without
changing caller inputs or older plan snapshots. RequiredFilterColumns in strict
reads/updates/deletes require all-branch non-NULL equality on that alias.

## Writes, conflicts, projections and authorization

Every INSERT candidate is copied and checked. Mixed tenants, missing/nil/invalid
values and inconsistent batch columns are rejected before execution. UPDATE
rejects every assignment to the tenant column (including a same-value assignment),
ImmutableColumns and ForbiddenColumns. Immutable columns may be initialized on
INSERT. ForbiddenColumns apply to reads and writes. PII reads/writes/RETURNING
are refused in this strict gate: AccessReason is intent, not an external permit.
PolicyMode remains the compatibility severity setting and does not downgrade the
strict check. Suppression, approval strings and risk overrides cannot waive it.
Default and explicitly configured structural high-risk operations are refused without an external permit;
GQ-AI-13's provider API is not implemented here.

Query Upsert supports explicit column-list targets and update-from-candidate
columns. UpdateOrInsert uses the same check on the merged candidate and actual
condition/update column lists. All keys must be valid, all-row, nondeferrable,
nonpartial, nonexpression integer constraints with nonnullable explicit values.
The complete unique set must be explicitly supplied; defaults/NULL/expression
semantics are unsupported. Constraint columns must agree with column schema.

For MySQL, **every** possible PRIMARY/UNIQUE conflict must include the checked
tenant column and values; a global unique key refuses the operation even if the
requested target is tenant scoped. PostgreSQL checks the inferred explicit target
against the declared constraint inventory; unrelated unique violations cause a DB
error, not another conflict update branch. Update columns cannot modify tenant,
immutable, forbidden or PII columns. Constraint/update WHERE expressions and
named/raw targets are outside this Query subset. See the
[MySQL reference](https://dev.mysql.com/doc/refman/8.4/en/insert-on-duplicate.html)
and [PostgreSQL 16 reference](https://www.postgresql.org/docs/16/sql-insert.html).

INSERT IGNORE still checks all candidates; warning/affected-row classification is
not a new guarantee. INSERT SELECT is refused. Query InsertGetId now checks the
final generated PostgreSQL RETURNING column and execution gate; MySQL still uses
LastInsertId. The compatibility PostgreSQL branch also gains its missing gate.
Final SQL/params/trees/targets are checked against private evidence at the existing
query gate. Editing public plan fields cannot change that evidence. JSON roundtrip
loses it and cannot become a strict executable artifact. EnsurePlanExecutable on
an arbitrary public/legacy plan is still not a general artifact validator.

## Entry coverage and deferred integration

| Entry | PR2 status |
| --- | --- |
| Query Plan/First/FirstMap/Get/GetMaps/Count | Conditional strict SELECT gate |
| Query PlanInsert/PlanInsertBatch/Insert/InsertBatch/InsertOrIgnore | All candidate rows checked |
| Query PlanUpdate/PlanDelete/Update/Delete | All-branch tenant and write policy gate |
| Query Upsert/UpdateOrInsert | Supported conflict subset above |
| Query InsertGetId | Final generated projection and gate checked |
| Query InsertUsing | Strict refusal |
| DB RawPlan and ordinary error-returning raw gates | Strict Raw refusal; TouchedTables cannot supply evidence |
| DB QueryRow/QueryRowContext | Legacy canceled empty-query error transport can still call executor; not zero-call rejection |
| PlanSelectBy/PlanUpdateBy/PlanDeleteBy, UpdateBy/DeleteBy | Inherit supplied Query checks; separate DB identity is not bound |
| SelectOneBy/SelectAllBy | Query checks plus separately supplied DB raw gate; strict raw DB refuses |
| UpdateByReturning and generic writes/RETURNING/batches | Final projection/common pipeline not integrated; GQ-AI-04 |
| Generic nested/idempotent/hooks | Delegate to their individual paths; no composite guarantee |
| OperationSpec/review/MCP/manifest | No acquisition of application trust; existing contracts unchanged |
| Build/Dump/RawSQL/direct SQLDB/Tx/driver/Executor | No semantic execution gate |

GQ-AI-04/PR2 now protects the listed generic CRUD/RETURNING and scoped helpers.
Consult the current inventory for exact coverage and migration; compound and
low-level escape paths remain excluded.
05 owns versioned plan/executor correspondence, 06 complete output masking, 07
complex OperationSpec, 09 review reconstruction, 10 result checks, 11 schema
freshness and 13 external permits. None is completed by this conditional gate.

Compatibility inspection also now records raw ORDER BY as unverified; this can
conservatively change a write-scope diagnostic to unknown. Its SQL is unchanged.

A minimal migration, after constructing the schema assertion and authenticating
and authorizing the application tenant, is:

```go
execution, err := orm.NewApplicationTenantContext(orm.ExecutionContextInput{
    CurrentTenant: authorizedTenantID, TenantPresent: true,
})
if err != nil { return err }
requestDB := db.WithOptions(
    orm.WithExecutionContext(execution),
    orm.WithTenantPolicy(applicationDatabaseID, assertedSchema, true),
)
// No user JSON contributes to execution or assertedSchema.
err = requestDB.Table("orders").Select("id").Where("id", orderID).First(&order)
```

Construct a new request derivation for each authorized tenant. Keeping the old
NewExecutionContext call under this option intentionally fails closed.

For GQ-AI-04/PR2 execution integration and the current entry boundaries, see
[planned Query execution](planned-query-execution.md). The historical table above
describes the PR65 baseline; generic integration is in PR2. PR3 adds private compound preflight and Raw
binding, the source-breaking Row wrapper, and Strict unsupported-recipe/scope
refusals. See the current inventory for the supported subset and compatibility
partial-execution limits; arbitrary callbacks and escape hatches are not covered.
