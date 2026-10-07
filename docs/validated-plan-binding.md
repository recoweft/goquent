# Validated plan binding

GQ-AI-05 PR2 (2/2), [Issue #70](https://github.com/recoweft/goquent/issues/70).
PR1 (#71) supplies the version and private canonical/HMAC contract. This opt-in
API adds current-input comparison for six Query operations. Ordinary CRUD keeps
its existing gates and does not require a binding key. This bounded implementation
does not complete every Issue #70 path or GQ-AI-06 output redaction.

## Supply and migration

`query.NewBindingContext(query.BindingContextInput{Key, Scope, Generation,
Target, Dialect})` validates and detaches input. Key needs at least 32 bytes;
scope, generation and target must be nonempty; dialect is mysql or postgres.
The existing canonical type/size budgets apply. Invalid input returns a zero
context and ErrBindingContext. There is no secret getter, public digest, keyring,
automatic key generation, storage service or persistent bearer permit.

Pass `query.BindingCurrent{Settings: currentSettings, BindingContext: context}`
on **both** generation and execution. The application must obtain actual current,
authenticated/authorized settings and key context immediately before each call.
Do not treat `q.Settings()` as an automatic current-state provider. Query's
construction settings and the supplied current settings must agree semantically,
including their base PolicySet even when a Query-specific policy overrides it.
After updating settings, construct a new Query and new handle. Query-specific
policy, required predicates, write keys, deletion visibility and effective risk
configuration also participate. Existing gate failures and q.err are not ignored.

Target must equal the current Settings database and application schema database;
the declared dialect must equal the Query dialect and schema dialect. These are
application assertions. Re-submitting stale current input cannot detect a lie,
a physical database replacement, external policy/schema changes or authentication
failure. Neither manifest Fresh nor startup/deployment comparison supplies current
execution authority. No database discovery or EXPLAIN occurs while planning.

The following illustrates the call sequence with application-owned inputs:

```go
binding, err := query.NewBindingContext(keyAndTargetFromApplication)
if err != nil { return err }
current := query.BindingCurrent{Settings: settingsFromApplication, BindingContext: binding}
handle, diagnostic, err := q.ValidateSelect(ctx, current, time.Now().Add(time.Minute))
if err != nil { return err }
// Display diagnostic; do not turn its fields into SQL or permission.
_ = diagnostic
// Obtain current settings/key context again from the application before execution.
current = currentFromApplication
return q.ExecuteValidatedSelect(ctx, current, handle, &rows)
```

Do not persist the handle or use a decoded QueryPlan instead. BindingContext and
ValidatedPlan explicitly reject JSON marshal/unmarshal and format as opaque text.
Nil pointers retain encoding/json's ordinary null behavior; no handle is restored.
A method-level failed decode clears the visited handle/context receiver, not its
aliases. Standard-library pre-method syntax failures may leave a receiver intact;
use a new zero receiver and discard errors. Logging raw BindingContextInput.Key
is the caller's responsibility. Existing diagnostic SQL/Params/reasons can still
contain sensitive data; complete redaction remains GQ-AI-06.

## Operations and meaning

All Validate methods take `(ctx, current, expiresAt, operationInputs...)` and
return `(*ValidatedPlan, *QueryPlan, error)`. All ExecuteValidated methods take
`(ctx, current, handle, operationInputs...)`. No signature exposes an internal
builder or accepts public plan JSON.

| Family | Validate inputs after expiry | Execute inputs after handle / result | Bound Query clauses |
| --- | --- | --- | --- |
| Select | none | dest / error (Get scanner) | Current SELECT projection, conditions, joins, grouping, ordering, limit/offset, locks and other emitted supported structure |
| Count | columns ...string | columns ...string / (int64, error) | CopyStateToSelect followed by existing Count transformation; retained projection/conditions/order etc. remain bound |
| Insert | data any (existing struct/map form) | data any / (sql.Result, error) | Table and current mapped data, including automatic tenant fill; unused WHERE/projection/order are not copied |
| InsertBatch | []map[string]any | []map[string]any / (sql.Result, error) | Table and every candidate in ordered single-statement batch |
| Update | data any | data any / (sql.Result, error) | Table, assignments and policyBuilder/CopyStateToUpdate conditions, joins and ordering actually emitted/inspected |
| Delete | none | none / (sql.Result, error) | Table and policyBuilder/CopyStateToDelete conditions, joins and ordering actually emitted/inspected |

Names are `ValidateSelect` / `ExecuteValidatedSelect`, and similarly for each
family above. Insert and InsertBatch are distinct even for one candidate. Count
and Select are distinct. Count columns are explicitly re-supplied and independently
bound in order; no columns and an empty slice use the existing `*` default.
Existing Count does not simply discard all previous projections: it marks matching
columns or appends an aggregate while retaining other columns. The DB/scanner can
reject unsuitable combinations just as with ordinary Count.

Only PR1's conservative generated/tenant-evidence domain intersected with existing
gates is supported. Raw/opaque material actually used by the operation, unsupported
typed values, unknown versions and insufficient context refuse. No Valuer,
Stringer or arbitrary marshaler converts opaque values into supported input.
Integer JSON lexemes do not become native integer widths or expand Strict's SQL
integer domain. Unknown/legacy/forged low-risk JSON remains diagnostic only;
rebuild from trusted typed source input and current settings to migrate.

An INSERT can ignore a valid WHERE, projection or ORDER clause, including an
unused WhereRaw with opaque values. Changing only such an unused clause does not
invalidate the INSERT handle, and does not evaluate or authorize that fragment.
The same clause used by SELECT/UPDATE/DELETE is inspected under those planners'
existing boundaries. Invalid builder state, q.err or a gate failure still refuses.
There is no public exclusion flag or promise to detect every Query mutation.

| Entry path | New validated binding |
| --- | --- |
| Query from DB.Table/TablePath/Model or NewWithSettings | Six families above, only within the supported domain |
| Same Query over custom Executor or externally owned sql.Tx | Same path; wrapper retains executor identity without comparing arbitrary executor values |
| Generic/scoped existing terminals | Existing private planned pipeline; no handle-consuming adapter added |
| Raw, compound, nested, hooks, idempotent recipes | No new validated terminal; existing Strict refusal/compatibility boundaries remain |
| Upsert, InsertOrIgnore, UpdateOrInsert, InsertGetId, RETURNING | No new validated terminal; existing behavior retained |
| SQLDB, direct sql.DB/sql.Tx/driver/Executor calls | Outside interception |
| OperationSpec, MCP | Read-only planning; no new database write endpoint |

No fallback from unavailable binding to ordinary CRUD is performed. No new
transaction ownership, callback safety, atomic external effects or full collection
replacement guarantee is introduced.

## Lifecycle and final dispatch

A handle holds detached private canonical material, typed structural inspection,
operation family/Count columns, key context, original owner/executor/context and
expiry. Full private material and keyed identity are compared, not shape or digest
alone. Creation clocks and diagnostic warning order are not execution identity.
Diagnostics retain no private execution/evidence reference and cannot mutate the
sealed material or confer another execution permission.

Each execution reserves the handle by CAS **before** validation. A successful
reservation is never returned to fresh state: mismatch, missing context, wrong
owner, cancellation, expiry, inspection failure, DB error and success all require
a new handle for another attempt. Pointer aliases and value copies share this
state. Concurrent losers get ErrBindingConsumed and never dispatch or invalidate
the winner. This synchronization does not make Query/data/current concurrent
mutation safe. A Query value copy has a different owner and cannot consume the
original handle successfully.

Effective context is explicit ctx when non-nil, otherwise Query's stored context.
Both the effective context and Query's stored context must match generation.
Context identity is separate from SQL/value identity. Non-comparable or typed-nil
contexts are unsupported without unsafe interface equality/panic. Context nil on
both sides uses non-context Executor methods. A context present on either path
uses the matching context method. Cancellation is checked before planning, after
fresh inspection and by the private dispatch gate. Existing ordinary API context
behavior is unchanged.

ExpiresAt is mandatory and future at generation. The handle is capped by any
private approval expiry; a fresh plan can shorten but never extend it. The
current Strict domain refuses high-risk operations requiring external permits
before a handle is created, even with a reason. The private approval minimum
calculation is unit-tested separately; it does not enable that unsupported path. Cancellation
and expiry are checked again at final private bind. After matching current input,
the exact fresh planner record supplies SQL and typed ordered arguments to one
Executor call; there is no second SQL render, public-JSON reconstruction or callback
provider between comparison and dispatch. The original private execution CAS is
also consumed. Driver errors/scan errors remain ordinary result errors after an
attempt; successful inspection does not predict DB success or Tx commit.

## Refusal identities and precedence

| Identity | Meaning |
| --- | --- |
| ErrBindingContext | Missing/invalid key context or inconsistent target/schema assertion |
| ErrBindingUnavailable | Nil/zero handle, unavailable identity/typed structure, unsafe context identity or serialization attempt |
| ErrBindingMismatch | Comparable semantic current input, operation family/columns, key/scope/generation or private material differs |
| ErrBindingExpired | Explicit handle expiry reached, including the final bind check |
| ErrBindingOwner | Different Query, executor reference or context lifecycle |
| ErrBindingConsumed | Any later or concurrent attempt after reservation |

Nil/zero handles refuse first; otherwise reservation wins before owner/context,
expiry/cancellation, current-context/settings, family, fresh planner/gate and
complete material checks. Simultaneous faults may therefore surface the earlier
check. Original gate/context/version errors preserve `errors.Is`; binding inspection
wrappers omit source text and do not expose source errors via Unwrap. They do not
promise errors.As for arbitrary original errors. Errors generated here contain no
SQL, argument values, keys, canonical bytes or guessable digest. Existing executor
and scanner errors are still returned by their existing result path.

Tests in `orm/query/binding_test.go` exercise refusal with all six counters zero,
all six dispatch methods, both dialects, typed values/current changes, ignored
INSERT clauses, JSON, copy/one-attempt/concurrency and final cancellation/expiry.
`tests/plan_binding_test.go` exercises six operations, bool scanning and external
Tx rollback against both databases. The language-neutral fixture is
`tests/contracts/testdata/validated_binding_v1.json`; it is not an executable permit.
