# Persistent command identity (GQ-AI-10 PR1 of 3)

Refs [Issue #86](https://github.com/recoweft/goquent/issues/86).
This is a new application-only, bounded command API. PR2 (optimistic writes and
shared row expectations) and PR3 (audit/outbox integration) are **not implemented**
by this change. Whole-Issue acceptance remains open.

## Deployment and migration

Keep `RunIdempotentCommand` for its existing callback recipe semantics. It does
not acquire persistent input correspondence by upgrading the library. Its
`LookupExisting`, `Apply`, `LookupAfterConflict`, `ErrConflict` behavior and Strict
opaque-recipe refusal remain unchanged. Existing external Executor/Tx APIs also
remain unchanged. There is no automatic migration of recipe results.

For the new path, explicitly install the appropriate
[MySQL DDL](sql/command-store-mysql.sql) or
[PostgreSQL DDL](sql/command-store-postgres.sql) in the selected application
database/schema through your deployment process. The fixed table name is
`goquent_commands_v1`. The library never creates or alters it. Supply an owned
DB from `OpenWithDriver` or `NewDB`, trusted application tenant context, and a
stable namespace to `NewCommandStore(ctx, db, namespace)`.

Authenticate and authorize the application request **before** constructing
`NewApplicationTenantContext`; never promote a request JSON tenant claim into
that context. Set existing application schema, table policies and conditional
Strict settings as described in [tenant policy](tenant-policy.md). Policies and
schema declarations must use the fully qualified `database.table` (MySQL) or
`schema.table` (PostgreSQL) names used by this path. Existing MySQL key declarations
use uppercase `BIGINT`/`INT`/`SMALLINT`; PostgreSQL uses lowercase names.

```go
// requestDB already has the application's trusted tenant and policy settings.
store, err := orm.NewCommandStore(ctx, requestDB, "orders-v1")
if err != nil { return err }
command := orm.Command{
    ID: operationID, // canonical UUID, durably retained before calling Run
    Kind: "set-score",
    Steps: []orm.CommandStep{{
        Operation: "update", Table: "orders",
        Keys: map[string]any{"id": int64(42)},
        Values: map[string]any{"score": int64(7)},
    }},
}
result, err := store.Run(ctx, command)
// Retain command unchanged on response loss; Run(ctx, command) reconciles it.
_ = result
return err
```

Use a fresh, live context when reconciling a cancelled call. Keep operation ID and
complete input durably on the application side **before** the first attempt.
`Run` with the same ID and input is the reconciliation API; it returns the saved
summary if committed and may apply the command if the first transaction never
committed. There is no query-only or arbitrary callback/codec/store plug-in API.
Do not switch to a new ID to recover an ambiguous response.

## Finite input and identity

* `Command` has an ID, kind and 1..64 ordered `CommandStep` values. A step is a
  single-row literal `insert` or an equality-conjunction `update`. Updates require
  nonempty `Keys`; inserts prohibit keys. Values are nonempty. Each map allows
  at most 256 columns. No SQL, expressions, aliases, qualified input table names,
  returning projections, increments, joins, UPSERT, hooks or expected-row options
  are accepted. Multiple rows can match an update; PR1 records its actual count.
* Table, column and selected schema/database names match
  `[a-z][a-z0-9_]{0,62}`. Business input cannot name the ledger. All executed
  business and ledger tables are fully qualified in the selected schema. No
  alternate schema/alias syntax is accepted. Schema selection is captured at
  construction; later session search-path changes do not redirect these writes.
* Namespace and kind match `[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}` (case sensitive,
  1..128 ASCII bytes). IDs are lowercase canonical hyphenated UUIDs with version
  nibble 1..8 and RFC variant nibble 8/9/a/b; nil, uppercase and other forms
  refuse. The caller generates the ID; the library never substitutes one.
* Scope contains format version, built-in dialect, selected schema/database,
  namespace and **typed** tenant. Only private application-provenance nonnull
  string/integer tenants are accepted; the ordinary execution context cannot
  substitute. The SQL primary key is `(scope_hash, operation_id)`. Kind belongs
  to input identity, so changing kind under the same scope/ID conflicts.
* Scalars are nil, bool, valid UTF-8 string, built-in int/int8/int16/int32/int64,
  uint/uint8/uint16/uint32/uint64 within 0..MaxInt64, and signed-64-bit integer
  `json.Number` lexemes matching `-?(0|[1-9][0-9]*)`. Named types, uintptr,
  floats, fractional/exponent decimals, invalid UTF-8, custom methods, pointers,
  structs, maps/lists **as values**, binary slices and cycles refuse without
  evaluating methods. The v1 SQL scalar-only shape has a fixed depth below 32;
  there is no recursive input extension. It is narrower than the read-only
  OperationSpec budget and does not change that API.
* Every integer Go type has its own tag; `int` and `int64` differ. Architecture
  width does not enter the tag: the same representable `int` value is identical
  across architectures, but values must fit the caller's native type. `uint`
  has the analogous rule. Integer `json.Number` preserves its lexeme, including
  the distinction between `0` and `-0`. No numeric type coercion collapses
  identities. SQL binding retains these scalar types and existing driver rules;
  a syntactically accepted scalar may still be refused by the existing planner.
* String bytes are exact, without Unicode normalization. Maps sort keys by byte
  order; step order is significant. Missing, null, empty string and false differ.
  Nil versus empty `Keys` both mean no conditions and are legal only for inserts.
  Null equality keys refuse; null assignments remain subject to SQL constraints.
* The library snapshots the accepted immutable scalars/maps once. The same
  snapshot supplies the canonical representation and all planned/executed
  statements, including transaction rebinding. Do not mutate caller maps
  concurrently with snapshotting (ordinary Go data-race rules apply).
* Raw key/type/value accounting and final canonical JSON are each limited to
  1 MiB; at most 16,384 scalar fields are visited. Canonical scope and fixed
  result are separately limited to 1 MiB. Reads use server-side bounded blob
  substrings and reject overflow. Numeric lexemes are at most 20 bytes.

The library computes domain-separated SHA-256 hashes (`goquent.command.scope.v1`
and `goquent.command.input.v1`, each followed by a NUL separator). Canonical JSON
uses fixed versioned structs, type/value atoms and sorted field arrays; boundaries
are encoded, never ambiguous concatenations. Scope, kind and complete step inputs
are stored alongside their hashes. Replay checks full bytes as well as digest,
version, committed status and strict fixed-result structure. A hash collision
cannot return another scope's result. A caller digest, plan view or result JSON is
never accepted as execution or authorization evidence.

## Transaction, policy and replay contract

`CommandStore` captures a private DB/settings snapshot and cannot be attached to a
different DB/tenant/dialect. Construct a new store to adopt new settings; a store
is not a live authorization service. Its owner must be the actual `*sql.DB` in
both driver and executor, not merely a wrapper with a nonnil driver. `NewTxDB`,
`WrapTx`, `WrapExecutor` and custom executors refuse before SQL effects in this
new path. There is no enforceable external rollback-only mode.

All steps pass the existing private planner and execution checks before a replay
lookup or reservation. The ledger uses only library-owned fixed, parameterized protocol statements,
not caller SQL or public plan execution. The whole set is planned and checked again on the actual
transaction executor, and final dispatch retains each statement gate. Strict
requires existing tenant/schema/policy/risk/binding checks; structural input or
ledger success does not waive any gate. Current configured policy is checked on
replay too. Compatibility settings retain their existing statement gates: ledger
scope isolation is not a new business-row authorization guarantee. Use Strict
and proper application authorization for business tenant enforcement. Logical
namespace/schema declarations are not physical DB attestation or authentication.

For a new ID: begin an owned transaction, INSERT a pending ledger record, execute
business steps, UPDATE exactly one ledger row with the actual result/status, then
COMMIT. Reservation and result-save counts must each equal one. A successful
summary is version 1 plus one actual `RowsAffected` count per ordered step. It
contains no invented generated ID, hydrated row or business-success assertion.
The same saved summary is returned on replay with `Applied=false`; a newly
committed result has `Applied=true`. The library does not infer optimistic success
from row count and does not impose a business row expectation in PR1.

Only an expected reservation unique violation is contention: MySQL 1062 after
validating the sole unique index, or PostgreSQL 23505 identifying this ledger's
schema/table/primary constraint. The losing transaction rolls back, then uses a
fresh outside-transaction lookup and the identical correspondence checks. A
business unique error or `ErrConflict` is not converted to replay. Lock waits,
timeouts, deadlocks and cancellation are errors, not successful duplicates.

| Outcome | Behavior |
| --- | --- |
| Saved matching committed record | Return saved summary without reapplying |
| Same scope/ID, different kind/type/value/order/input | `ErrCommandIdentityConflict`, distinct from `ErrConflict` |
| Corrupt/oversized record, unknown version, pending status, malformed result | `ErrCommandCorrupt`; never treat as missing and reapply |
| Business/result-save/row-count failure before commit | Roll back before returning; no successful result |
| Rollback returns an error (including automatic cancellation completion ambiguity) | Join `ErrCommandOutcomeUnknown` with the cause; do not assert confirmed rollback |
| COMMIT returns an error | Conservatively join `ErrCommandOutcomeUnknown`; reconcile same ID/input |
| Reservation duplicate disappears before reread | `ErrCommandOutcomeUnknown`; no blind retry |
| Process stops before commit | Database recovery rolls back the uncommitted transaction under normal engine guarantees |
| Commit succeeds but response is lost | New pool/store and same ID/input read the saved result |

No automatic retry, TTL, deletion or operation-ID recycling is implemented.
Deleting/replacing ledger data removes the subsequent replay guarantee. Operation
records contain sensitive input and results; restrict access and manage storage
retention in the application. Old recipe records need an explicit application
migration or a new namespace; they cannot be guessed into this ledger format.

## Schema trust and row semantics

Construction and every Run perform read-only checks of ledger column types,
capacity/nullability/generated status, primary-key shape, uniqueness, foreign
keys, base-table kind and triggers/rules. Business targets must be base tables;
MySQL requires InnoDB, PostgreSQL requires permanent nonpartitioned/noninherited
tables without RLS, user triggers or rewrite rules. The ledger also rejects these
features. Incompatible metadata fails before business writes. Tests use only
explicit dedicated test DSNs and never replace an existing ledger automatically.

This is not protection against an administrator concurrently changing schema,
installing triggers or modifying the ledger. Prevent those actions during use;
reconstruct/revalidate stores after controlled deployment changes. Metadata access
must be sufficient to observe the actual schema. Out-of-band mutation, malicious
DB drivers/servers, unstable SQL proxies, physical database replacement, relaxed
durability settings and hidden external effects are outside the guarantee.
The database and its constraint/transaction/durability configuration are trusted.
No result serialization certifies tamper resistance.

MySQL default UPDATE counts changed rows: a value-unchanged update can return 0.
With `clientFoundRows=true` it counts matched rows. PostgreSQL UPDATE counts matched
updated rows even when values are unchanged. Zero and multiple row counts are
stored as observed. MySQL UPSERT generally reports 1 for insert, 2 for a changed
conflict update, and 0 for an unchanged update (the found-rows setting affects
that last case); PostgreSQL INSERT ON CONFLICT counts rows actually inserted or
updated, and DO NOTHING contributes zero. **UPSERT is not accepted by this PR1
command API**; shared write expectations and optimistic version integration belong
to PR2. Existing audit/outbox helpers are not connected here; PR3 remains separate.
There is no DB-external effect or exactly-once delivery guarantee.

## Source and public output boundary

Command input, store internals, IDs, scope/fingerprint, summaries and runtime
errors are sensitive application source data. Driver errors preserve error
identity and may include SQL/input; never publish them directly. Existing
CLI/MCP/public-view consumers do not expose or execute this command API. At a
public application boundary use a fixed error such as `publicoutput.ErrOutput`,
not the source error, and define a separately bounded display projection. Public
views, reason strings, low risk, `precise`, Fresh or suppressed findings cannot
supply authorization or input correspondence.

## PR2 extension

The PR1-only boundaries above are retained as historical context.
[Atomic write results](atomic-write-results.md) defines the added finite version
and expectation fields, shared checks, input-only v2 migration and old-reader
refusal. Unextended commands retain the original v1 bytes and behavior. PR3 hooks
remain unimplemented.
