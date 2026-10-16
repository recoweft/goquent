# Command audit and outbox hooks (GQ-AI-10 PR3 of 3)

Refs [Issue #86](https://github.com/recoweft/goquent/issues/86).
This extends [persistent commands](command-identity.md) and
[atomic results](atomic-write-results.md). It does not adopt the whole Issue's
acceptance checklist or claim external delivery exactly once.

## Migrating existing hook recipes

`TransactionHook{Name, Run}`, `NewTransactionHook`, `InsertHook` and
`InsertManyHook` retain their existing callback semantics. The public `Run` can
be replaced, and a closure cannot prove its inputs or effects. These objects are
**not** accepted by `CommandStore`. `RunTransactionWithHooks` and
`RunIdempotentCommand` still reject opaque recipes in Strict. Compatibility
callbacks do not acquire persistent identity or guarantees about external I/O.

Move an `InsertHook("audit", row)` or `InsertHook("outbox", event)` use case to
an explicit `CommandHook{Name, Table, Values}`. Translate `InsertManyHook` into
ordered individual `CommandHook` entries; repeated names are permitted and each
entry still executes once. No reflection, custom value method, callback, SQL,
expression, UPSERT, INSERT IGNORE, generated-ID dependency or arbitrary `WriteOpt`
is accepted. Construct all event IDs, timestamps (strings in the finite scalar
domain), and configuration-derived values before the first call and durably retain
the complete request. Replays must not regenerate those values.

The following recipe is exercised on both databases by
[`testCommandHooksIntegration`](../orm/command_hooks_integration_test.go), using
Strict settings established by
[`TestCommandIntegration`](../orm/command_integration_test.go). `trusted` is an
owned DB with an application-provenance tenant context, a qualified plain-table
schema with complete primary-key metadata, and tenant policies for **all three**
tables. The schema declares `tenant_id`, `id`, `score` as signed BIGINT columns,
the primary key `(tenant_id,id)`, and `version` for the business table. Use
`NewApplicationTenantContext`, `NewApplicationSchema`, `NewPolicySet` and
`NewSettings(...).WithTenantPolicy("command-fixture", schema, true)` as in the
fixture. A public plan, supplied JSON tenant, or ledger namespace is not a
substitute for that trusted setup. Schema metadata must describe the actual DB.

```go
store, err := orm.NewCommandStore(ctx, trusted, "gq10-fixture")
if err != nil { return err }
one := int64(1)
command := orm.Command{
    ID: retainedOperationID, // lowercase canonical UUID; retain before first Run
    Kind: "audit-outbox",
    Steps: []orm.CommandStep{{
        Operation: "insert", Table: "gq10_items",
        Values: map[string]any{"id": int64(501), "score": int64(2)},
        ExpectAffected: &one,
    }},
    Hooks: []orm.CommandHook{
        {Name: "audit", Table: "gq10_audit",
            Values: map[string]any{"id": int64(501), "score": int64(3)}},
        {Name: "outbox", Table: "gq10_outbox",
            Values: map[string]any{"id": int64(501), "score": int64(4)}},
    },
}
result, err := store.Run(ctx, command)
// result.Summary.RowsAffected is [business, audit, outbox]: [1,1,1].
// A matching saved retry returns that summary with Applied=false.
// On outcome unknown, reconcile this exact ID AND input, never a fresh ID.
```

The demonstration tables are ordinary transactional tables:

```sql
CREATE TABLE gq10_items (
  tenant_id BIGINT NOT NULL, id BIGINT NOT NULL, score BIGINT,
  version BIGINT NOT NULL DEFAULT 0, PRIMARY KEY (tenant_id,id)
);
CREATE TABLE gq10_audit (
  tenant_id BIGINT NOT NULL, id BIGINT NOT NULL, score BIGINT,
  PRIMARY KEY (tenant_id,id)
);
CREATE TABLE gq10_outbox (
  tenant_id BIGINT NOT NULL, id BIGINT NOT NULL, score BIGINT,
  PRIMARY KEY (tenant_id,id)
);
```

Use InnoDB on MySQL; PostgreSQL targets must be ordinary persistent plain tables.
The explicit [MySQL](sql/command-store-mysql.sql) or
[PostgreSQL](sql/command-store-postgres.sql) ledger DDL is also required. The
library never installs, repairs or migrates tables. Deploy schemas through your
normal reviewed process; do not run fixture DDL against an application database.
The integration fixture refuses a pre-existing ledger and removes only tables
it created. On identified dedicated Compose `testdb` instances, run sequentially:

```sh
export TEST_MYSQL_DSN='root:password@tcp(127.0.0.1:3306)/testdb?parseTime=true'
export TEST_POSTGRES_DSN='postgres://postgres:password@127.0.0.1:5432/testdb?sslmode=disable'
TEST_DB_DSN="$TEST_MYSQL_DSN" go test ./orm -run 'TestCommandIntegration/mysql/hooks' -v -count=1
TEST_DB_DSN="$TEST_POSTGRES_DSN" go test ./orm -run 'TestCommandIntegration/postgres/hooks' -v -count=1
```

## Owned execution and visibility

1. Snapshot all business and hook inputs once. Validate every private plan and
   all actual table catalogs before looking up a saved result. Hook targets use
   the same tenant/schema/policy/risk/typed gates and plain-table, engine,
   trigger/rule/inheritance restrictions as business targets.
2. For a missing operation, begin the library-owned transaction and rebind/check
   every retained plan against its actual transaction executor. Reserve the ID.
3. Execute business statements in order, then hooks in order. Each hook is one
   ordinary INSERT and requires exactly one affected row through the existing
   shared checker. It cannot commit, roll back, change executors, or call I/O.
4. Persist the fixed summary in exactly one ledger row, check cancellation, then
   commit. A successful callback is not used as an atomicity proof.

The same owned SQL transaction contains the reservation, business, audit, outbox
and saved result. Under normal committed-read isolation, other transactions see
committed data after commit, not intermediate successful hooks. This does not
promise a cross-query snapshot to an observer or isolation for READ UNCOMMITTED.
Consumers must read committed outbox records. External delivery/acknowledgement
and duplicate delivery handling remain a separate application protocol.

Business, first-hook, later-hook, count, CAS, save and pre-commit failures invoke
rollback before returning. The first failure stops execution: when both hooks
would fail, the second hook is not attempted. A later-hook failure rolls back a
successful earlier audit INSERT. `ErrConflict` remains a business optimistic
conflict; `ErrCommandIdentityConflict` means a changed operation input. A business
unique error is not a reservation replay. The losing reservation transaction
rolls back before a fresh lookup verifies the complete stored correspondence.
Neither ordinary replay nor reservation-race replay runs business or hooks again.

A failed commit or uncertain rollback is `ErrCommandOutcomeUnknown`, potentially
joined with the underlying error. Reconcile with the same retained ID/input on
a capable reader. Do not delete the record or blindly retry under a new ID.
There are no automatic retries. Network COMMIT response loss and power loss are
not established by mock errors or a new pool/store replay test. Durability also
depends on server settings. Sequence/auto-increment allocation, scanner effects,
external I/O, broker delivery and arbitrary callbacks are outside DB rollback.

Only the actual owned `*sql.DB` retained by the driver/executor qualifies.
External Tx, `WrapTx`, `NewTxDB`, `WrapExecutor`, and custom executors refuse the
new guarantee. Existing ordinary external transactions still require their caller
to roll back errors; there is no enforceable external rollback-only mode.

## Finite identity and compatibility

Hook names use nonempty `commandLabel` syntax: 1..128 ASCII characters,
`[A-Za-z0-9][A-Za-z0-9_.:-]*`. Table/column names and literal scalar types use the
existing command rules. Null values remain policy/schema dependent. Ledger and
qualified alternate-schema input names refuse. Hooks cannot be the only work:
at least one business step is required. Business plus hooks share the maximum
64 statements, 1 MiB raw/canonical input budgets and 16,384 scalar fields; each
map has at most 256 columns. Names and table metadata count toward the byte
budget. Snapshotting never evaluates custom methods. Later caller mutation cannot
alter the retained SQL or canonical identity; concurrent map mutation during
snapshotting is still a Go data race.

| Component | No hooks (nil or empty) | One or more hooks |
| --- | --- | --- |
| Input | Original v1, or v2 when business conditions are present | Canonical v3: all business conditions and ordered hook names/tables/typed values |
| Hash domain | Original v1/v2 bytes and domains | `goquent.command.input.v3` followed by NUL |
| Scope / ledger / format_version | Existing v1 | Existing v1; no DDL migration |
| Summary shape | Existing v1 | Existing v1, counts in business-then-hook order |
| New reader | Validates/replays v1/v2 | Validates/replays v3 |
| PR1 / PR2 reader | Existing supported versions | Refuses as corrupt/unsupported, never missing or executable legacy input |

Summary length must equal the total statement count. Hook indices start at
`len(command.Steps)`; no hook names, generated IDs or hydrated rows are added to
the summary. Input and summary readers reject noncanonical, duplicate, unknown,
oversized and unsupported encodings. The frozen PR1 and PR2 reader fixtures test
ordinary and reservation-race rejection of v3 with no business execution. These
are executable frozen source fixtures, not independently built old binaries.

Deploy capable readers to every reconciliation path **before** enabling new v3
operations. Preserve all records and retained inputs through downgrade; restore a
capable reader to reconcile v3. Do not reinterpret v3 as v1/v2, convert unsupported
records into misses, or retrofit hooks to an already used ID. Adding/removing a
hook or changing its name, order, type or value under the same scope/ID conflicts.
This includes fixed configuration changes affecting hook input. A new ID is for
a genuinely new business operation, never an uncertain retry.

DB settings are not silently mixed into canonical input. A store retains its
private settings snapshot; construct a new store to adopt changes. Current gates
apply to both business and hooks even on replay, so a newly forbidden hook can
block reconciliation until a valid policy/schema deployment is restored. A saved
success is not authorization to bypass that gate. The store is not a live
authorization service.

Scope remains dialect/schema/namespace/**typed trusted tenant**. Same UUID under
a legitimate different tenant store is an independent operation; this does not
introduce global UUID uniqueness. Strict rejects business or hook values contrary
to its trusted tenant before reservation or writes, and saved results from another
scope cannot be reused. Scope isolation alone does not add business authorization
to compatibility mode. No global reservation/schema migration is introduced.

Adding `Command.Hooks` breaks external unkeyed `Command` literals; migrate to keyed
literals. Existing hook constructors and keyed commands keep their behavior.
There are no new dependencies or public internal-builder types. Public views,
read-only OperationSpec/CLI/MCP, private owner/seal/binding boundaries and sensitive
source-result handling are unchanged; a source result is not a display artifact.

## Evidence map (not whole-Issue acceptance)

| Acceptance area | Executable evidence | Limits |
| --- | --- | --- |
| Owned atomic hooks, audit/outbox rollback | `TestCommandIntegration/*/hooks`: both success, first/later/both hook failures, result save failure, same-input recovery; direct four-table counts | Both-failure case stops at first; no claim second hook ran |
| Row/CAS/prior-step rollback | `prior-step-rollback-zero/multiple/cas/later-business`, existing atomic integration | Driver row semantics are documented in atomic results, not UPSERT branch proof |
| Replay/identity/races | `success-replay-input-and-tenant`, public concurrent first and forced reservation same/different input | Forced race uses internal apply seam after planning to ensure unique conflict |
| Tenant/settings/ownership | Wrong business/audit/outbox tenant; independent tenant store; current hook policy; missing catalog; external owners | Trusted schema/policy and no concurrent DDL remain deployment assumptions |
| Input/reader compatibility | `TestCommandHookIdentityAndSnapshot`, limits, frozen old-reader ordinary/race tests; v1/v2 regressions | Frozen shared-helper fixtures, not independent binaries |
| Fault classification | `TestCommandHookFaults`, late result-count error, command transaction mocks; real cancellation while reservation waits | Mock commit/rollback/count failures are not real network faults |
| Existing Issue-wide row behavior | Existing atomic integration tests for unchanged updates, UPSERT counts, RETURNING and external Tx | This PR does not re-adopt Issue-wide AC |

Full/race/vet, regeneration, output canary, review, manifest, final-head CI and
required-check observations belong to the PR's fresh verification report. Existing
review risk/precision failures, comparison input failures and stale/skipped manifest
checks are not waived by these integration tests. No threshold relaxation or bulk
baseline is part of this change.
