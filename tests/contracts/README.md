# Shared database operation contracts

GQ-AI-01/PR2, [Issue #52](https://github.com/recoweft/goquent/issues/52).
Baseline: merged PR1 `778ce9f1f97a37b0fb55bfe0edbe66ab9888ff4f` (includes
maintenance PR #55). This package adds test data and tests only.

## Case register and evidence

[`testdata/cases.json`](testdata/cases.json) mirrors C01–C28 in
[contracts v3, section 4](../../specs/goquent_ai_contracts_v3.md#4-shared-cases-documentary-specification-not-new-tests).
It is documentary input, **not** a new OperationSpec, executable plan format,
Strict option or permission source. `defaults` applies to every case; `input`
is a human-readable operation. Tenant context in a required determination is a
future requirement, not a currently implemented API. Values are fictional.

Each case has input, current source observation, required future determination,
owners (management IDs, not inferred GitHub issue numbers), analysis limit and
executable evidence references. `coverage` means:

- `documentary`: no executable evidence registered for this exact case. Current
  observation and future requirement must not be presented as tested.
- `partial`: named tests exercise only the stated `assertion_scope`, often a
  related recipe or synthetic plan. The complete case remains unverified.
- `executable`: named tests exercise the bounded current behavior described in
  `assertion_scope`. This does not certify the future requirement or authorization.

A reference is `repository/path_test.go#TestName`. Registration does not mean the
test ran or passed: consult the PR's command results, skips and CI head.
`TestSharedCaseRegistry` checks all 28 IDs exactly once, required fields, exact
four-column correspondence to the specification, owner correspondence, coverage
states and actual Go test declarations. Negative registration tests exercise
missing/duplicate IDs, verdict drift, wrong owners, absent tests, invalid coverage
and API inventory drift. No test asserts that a known safety gap must remain.

`api_coverage` retains every tabular public entry from sections 2.1–2.4 and maps
each API family to existing test files. The validator catches omitted/changed
entries and nonexistent files. All API-family mappings are explicitly partial:
file membership does not prove every symbol/variant is exercised. Non-tabular
constructors, chain modifiers, options, conversion and projection helpers inherit
the documented consuming path, or remain outside DB inspection. Section 2 of the
specification remains the authoritative per-entry inspection/exclusion inventory.

## Reused fixtures and bounded compatibility checks

Existing inline and temporary plan, diagnostic, migration and manifest fixtures
remain in their original tests. In particular, run:

| Path / test | What it checks | What it does not prove |
| --- | --- | --- |
| `orm/query`: `TestSelectPlanSnapshot`, `TestWritePlanSnapshots`, `TestRawPlanSnapshot`, risk/policy tests | SQL, params, selected snapshot metadata, no DB calls, warnings, gate errors and non-suppression | Complete tree proofs or terminal parity |
| `orm/review`: `TestRunReviewsRawSQLAndQueryPlanJSON`, `TestRunReviewsMigrationPlanJSON`, `TestRunReviewsSuppressedWarningsFromQueryPlanJSON`, `TestWriteJSONAndPretty` | Selected diagnostic codes, locations, raw evidence, suppression counts and output markers | Authenticity of input warnings/verdicts or all wire bytes |
| `orm/manifest`, `orm/operation`, `orm/mcp`, `cmd/goquent` tests | Load/version/fingerprint checks, read-only validation, missing/stale/unverified review gate, CLI errors | Trusted tenant context, complete value typing or live DB equality |
| `orm/write_test.go`, `scope_test.go`, select/bool tests; `tests/write_generic_test.go`, `select_generic_test.go`, `orm_postgres_test.go` | Generic/scoped CRUD, dialect SQL, results and scanning/bool recipes | Common policy or uniform affected-row checks |
| `orm/orm_raw_test.go`; `tests/raw_sql_test.go`, `custom_driver_test.go`, `manual_transaction_test.go`, `transaction_context_test.go` | Raw rejection, executor/driver compatibility, transaction delegation | Universal interception or external authorization |
| `orm/idempotency_test.go`, `transaction_hooks_test.go`; `tests/nested_write_test.go` | Existing/conflict/nested/hook recipes and selected rollback behavior | Concurrent exactly-once effects or callback atomicity |

The added `TestSharedPlanSerializationAndReview` connects C01/C02/C28 to both
MySQL and PostgreSQL planners using an executor that panics on every DB method.
It checks distinct dialect SQL, bound fictional values, limit/risk, selected
operation/column/predicate fields after JSON decoding, repeated serialization of
the same plan, diagnostic codes/severity/location/non-suppression through
`review.Run`, and summary/count/pretty output. JSON number conversion is explicit.
These are selected semantic assertions, not full historical wire compatibility,
public-data redaction, live execution or business permission. C20 stays documentary;
no real secret is serialized and no test requires continued secret exposure.

`TestExampleFixtureCompatibility` reuses the five checked-in files under
[`examples/ai-safe-orm`](../../examples/ai-safe-orm). Their SHA-256 values in
[`testdata/examples.json`](testdata/examples.json) preserve the exact PR1 baseline
bytes without copying or regenerating the originals. It exercises Load/Validate,
WriteJSON/Load of known manifest fields, schema/policy fingerprint comparison,
changed-schema rejection and example OperationSpec compilation (params, limit and
soft-delete predicate). Missing generated-code/database fingerprints remain
**unverified**; a `fresh` aggregate does not turn skipped evidence into verification.
No live schema read occurs. This is not a universal JSON consumer compatibility test.

`TestExternalTransactionOwnership` connects C27 to both dialect wrappers with
sqlmock: NewTxDB.Close leaves the external transaction usable, raw execution is
delegated, and the caller rolls back. Existing custom Executor and live transaction
tests remain in place; wrapping does not imply Begin capability or context binding.

## Run and evolve

Run from the repository root, **serially** against shared databases:

```sh
go test ./tests/contracts -count=1
go test ./...
make test-integration
go test ./orm/query ./orm/review ./orm/manifest ./orm/operation ./orm/mcp ./cmd/goquent -count=1
go run ./cmd/goquent review --fail-on high ./...
go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json
```

`make test-integration` starts MySQL 8/PostgreSQL 16 and sets explicit DSNs so
connection failures cannot silently skip the standard DB helpers. The separate
custom-driver test requires `TEST_DB_DSN`; set it to the same local MySQL DSN
when running the integration target to exercise that test too. `GOFLAGS=-json`
records test-level pass/fail/skip events. Report actual skip/failure results. `TestRunTransactionWithHooksCommitsAuditHook` has a known independent
column-order flake; report occurrences without changing production code, skipping
it or relaxing expectations. Repository review is expected to expose existing
findings; record its exit status and precise/partial/unsupported counts separately
from test success. No passing suite certifies unsupported operations.

Follow-up implementations retain the case ID and record owner, reason, old/new
verdict, compatibility/migration impact and new executable regression reference in
the specification, register and PR. Update `coverage` only to the extent demonstrated
by those assertions. Keep unknowns explicit. Never blindly refresh hashes or
convert a known defect into a correct golden result. New guarantees belong to their
GQ-AI owner; this package does not implement GQ-AI-02 onward, Strict, redaction,
DB-scoped policies, condition trees or production write changes.
