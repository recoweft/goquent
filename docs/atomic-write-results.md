# Atomic write results (GQ-AI-10 PR2 of 3)

Refs [Issue #86](https://github.com/recoweft/goquent/issues/86). PR1 is
[PR #87](https://github.com/recoweft/goquent/pull/87), merged at
`68f0d7b24ca9a67e4984fd0f51dab5ae47738421`. PR3 audit/outbox integration remains
separate and unimplemented. Whole-Issue acceptance is not claimed.

## API and ownership

`AtomicWrite()` is an explicit generic write option. `Query.AtomicWrite()` is the
DSL equivalent. `Query.ExpectAffected(n)` and `Query.NoRowsAs(err)` mirror the
existing facade options. Expectations refer to observed counts, not business
authorization or a proof of a unique match. Existing expectations alone do not
start a transaction: an autocommitted mismatch can already have changed data;
an external transaction's caller must roll it back. Existing external APIs remain.

The atomic mode requires the actual owned `*sql.DB` from `NewDB` or the Open APIs.
`NewDBWithExecutor`, `NewTxDB`, `WrapTx`, `WrapExecutor`, standalone `query.New`,
and arbitrary custom executors cannot acquire ownership by wrapping a handle.
They reject the new mode before business SQL. There is no external rollback-only
contract. A caller receiving an error inside an ordinary external transaction
must decide its outcome; returning a callback error is not enforced rollback.

Opaque mutable parameter values refuse in the new atomic mode; supported values
are detached without invoking custom methods. The private dispatcher consumes
the original plan, begins a transaction,
re-inspects the retained private SQL/typed values with the transaction executor,
and finishes execution and all result checks before commit. Public diagnostics
cannot supply an executable plan. Strict tenant/schema/policy/risk, owner/seal,
binding expiry, and typed presence checks remain in force. MySQL atomic writes
check the final target tables in `information_schema.tables`, require InnoDB,
and refuse catalog-visible triggers;
unknown, non-base, aliased or unsupported target identities refuse. The application
must supply catalog privileges sufficient to see all target triggers. PostgreSQL
uses its transaction semantics. Only built-in MySQL/PostgreSQL dialects qualify.
Keep schema and engine configuration stable during execution; concurrent
administrative DDL, malicious drivers, relaxed durability, and external effects
are not protected. Sequence/auto-increment allocation need not be rolled back.
CommandStore additionally retains its existing live plain-table/ledger checks.

On SQL, scan, late-row, close, count-acquisition or expectation failure, the owned
transaction rolls back before the error returns. Commit failure or rollback
failure includes `ErrWriteOutcomeUnknown`; CommandStore retains
`ErrCommandOutcomeUnknown`. Do not blindly retry an ambiguous write. Commands
reconcile with the same durable ID and unchanged input. Scanner destination
assignments, scanner callbacks and other application external effects are not
rolled back. No new callback execution API is introduced.

Negative or duplicate expectations and duplicate nonnil `NoRowsAs` values refuse
before business SQL. `NoRowsAs(nil)` supplies no mapping. If `ExpectAffected(0)`
matches zero, it takes precedence over NoRowsAs. Otherwise zero uses NoRowsAs when
provided. Source error identity remains available through `errors.Is/As` and
`RowsAffectedError`; it is not a safe public message. Never publish arbitrary
source errors or invoke their methods to construct public diagnostics.

## Supported paths

| Path | Shared result check | Atomic mode | OptimisticVersion |
| --- | --- | --- | --- |
| Query INSERT/batch/ignore/UPSERT/UPDATE/DELETE and By helpers using Query modifiers | Yes | Owned DB, complete result only | UPDATE only, finite conditions below |
| Generic Insert/Update/Upsert and Many, including `Returning` sql.Result mode | Yes | Owned DB | Update only |
| Typed one-row RETURNING and ManyReturning (PostgreSQL) | Full stream count and errors | Owned DB, before commit | UpdateReturning only |
| UpdateByReturningWithOptions | Returning plus result/atomic/version options now apply | Destination owned DB | Finite UPDATE only |
| UpdateOperationBy / UpdateOperationReturningBy (typed patch) | New trailing `...WriteOpt` accepts result/atomic options | Owned DB | Explicitly refused; typed assignment declaration does not implicitly authorize a version assignment |
| Validated Query insert/batch/update/delete | Same private dispatcher; finite expected/atomic/version settings included in private identity | Owned DB | Validated UPDATE with the same finite restrictions |
| CommandStore ordered literal INSERT/UPDATE steps | Each step, same owned command transaction | Always; no per-step nested transaction | UPDATE steps only |
| InsertGetId / InsertOnceReturning | Result modifiers explicitly refused | Unsupported | Unsupported |
| Nested recipes / hooks / opaque callback recipes | Existing aggregate compatibility checks retained | New atomic/version options refused at preflight | Unsupported |
| SELECT / Raw / unverified expressions / joins / INSERT SELECT / externally streamed rows | Write result modifiers refused | Unsupported | Unsupported |

Typed patch options other than result/atomic options refuse instead of being
ignored. Existing scoped RETURNING table/column/key/assignment/conflict option
nonapplication remains; actual base/scopes/data determine the UPDATE. New result
options no longer disappear. First-row write helpers retain only the first
returned object but scan every row and count the complete stream. They do not
collect an unbounded extra array. Many helpers retain their existing collection
behavior. A missing typed first row remains `sql.ErrNoRows` (or the configured
count error), and atomic mode rolls back even when expected count zero matches.
RowsAffected acquisition is distinct from RETURNING row counting. A one-row
return value alone never proves an expectation of one.

Validated handles cannot bind an arbitrary `NoRowsAs` error value: private
identity is unavailable for that option. Existing handles/default identity
material remain unchanged when no result contract is supplied. New result
settings cannot be changed under an old validated handle.

## Finite optimistic version contract

`OptimisticVersion(column string, expected int64)` and the Query modifier compare
`version = expected` and assign `version = expected + 1` in the **same final
UPDATE**. There is no pre-SELECT version assertion. The version column is explicit;
its name and Go model type are not evidence of a key or authority.

This mode always requires atomic ownership and exactly one affected row, even
without AtomicWrite. Zero returns business `ErrConflict` through
`RowsAffectedError`; multiple rows return a row-expectation error. Neither is a
reservation duplicate or command identity conflict. A failed CAS does not become
a successful same-ID replay unless there is an independently saved matching
committed command result.

The supported form requires the existing Strict trusted application schema and
tenant context, a plain unaliased target, an equality conjunction covering every
column of one schema-declared primary key (including composite keys), and the
trusted tenant condition. Automatic tenant insertion remains available. OR,
raw/non-equality/duplicate/ambiguous conditions, joins, unknown schema and
incomplete keys refuse. Soft-delete non-equality conditions are outside this
initial finite form. Version must be a separate, nonnull signed SMALLINT/INT/
BIGINT (PostgreSQL smallint/integer/bigint) with matching declared bit width.
Expected is nonnegative; both it and its successor must fit. Overflow, unsigned,
nullable, custom types, and updates to PK/tenant/version through ordinary values
or expressions refuse. No arbitrary expected/next types or optimistic
INSERT/DELETE/UPSERT are provided.

An explicit expectation other than 1, duplicate version modifier, or nonnil
NoRowsAs with OptimisticVersion refuses. Schema is the application's trusted
assertion, not live physical attestation. Row counts, precision, low risk,
reasons, and successful result storage do not establish business authorization.

## Dialect observations

| Operation | MySQL default | MySQL clientFoundRows=true | PostgreSQL |
| --- | --- | --- | --- |
| INSERT one row | 1 | 1 | 1 |
| UPDATE unchanged values | 0 changed | 1 matched | 1 updated |
| Version CAS success | 1 (version changes) | 1 | 1 |
| UPDATE/DELETE no match | 0 | 0 | 0 |
| DELETE | Deleted rows | Deleted rows | Deleted rows |
| UPSERT inserted | 1 | 1 | 1 |
| UPSERT changed conflict update | 2 | 2 | 1 |
| UPSERT unchanged conflict update | 0 | 1 | 1 |
| ConflictDoNothing for existing row | No-op UPDATE count, usually 0 | No-op UPDATE count, 1 | 0 |
| RETURNING | Unsupported, rejected | Unsupported, rejected | Returned rows counted after full scan |

These are bounded driver/server observations, not a branch classifier. MySQL
ON DUPLICATE KEY and PostgreSQL ON CONFLICT do not expose identical semantics.
Batch counts aggregate their respective outcomes. Do not infer insert/update
branch, physical rows, trigger effects, or portable ID order from a count.

## Command input v2 and rollout

`CommandStep.ExpectAffected *int64` and `CommandStep.Version *CommandVersion`
(`Column`, `Expected`) are optional application input. Pointers and maps are
snapshotted before planning. Every step's explicit presence, expectation and
version meaning is included in identity. An unspecified expectation, explicit
zero, explicit one, and a version condition are different input.

| Component | Unextended command | Command with any new condition |
| --- | --- | --- |
| Canonical input and input hash domain | Existing v1 bytes and domain unchanged | Canonical v2, `goquent.command.input.v2` domain |
| Scope and namespace identity | v1 | v1 |
| Ledger table / format_version | goquent_commands_v1 / 1 | Same |
| Persisted summary | Fixed result v1 | Fixed result v1 |
| New reader | Reads/replays v1 | Reads/replays v2 |
| PR1 reader | Existing v1 replay | Rejects v2; frozen old-reader fixture observes ErrCommandCorrupt |

No automatic DDL or data migration is needed. Deploy the new reader everywhere
that may reconcile a command before enabling new v2 operations. Existing v1
operation IDs retain their original meaning and input; do not retrofit an
expectation or version under an existing ID. New reader same-ID v1/v2 or condition
changes produce identity conflict. New guarantees apply to new business
operations, not to a new ID used to retry an uncertain old operation.

New readers validate bounded canonical v1/v2 input and the corresponding hash
domain before identity comparison. Unknown/malformed/duplicate/oversized fields,
wrong status/hash/result refuse without reapplying. Downgrading to PR1 after v2
writes can break reconciliation: old readers check the v1 hash domain first, so
v2 is not promised to produce identity conflict. Preserve the record, do not
reinterpret it as v1 or missing, and restore a capable reader. Ordinary and
reservation-race lookups use the same validation. Policy/schema/ownership are
rechecked on replay; a saved successful CAS is not run again against the now
advanced version.

Source compatibility: adding fields to CommandStep breaks external unkeyed
composite literals; use keyed literals. Adding trailing variadic options to typed
patch execution preserves ordinary existing calls but changes exact function
types. The RowsAffectedError public name is an alias to the shared implementation;
source errors.Is/As behavior remains, but reflected package identity changes.
The corrected RETURNING checks and duplicate-option refusals can surface errors
previously ignored; audit those callers before upgrading.
