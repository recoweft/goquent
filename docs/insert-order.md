# INSERT column order

GQ-MAINT-02/PR1 stabilizes generic `Insert` and `InsertReturning` SQL.
Previously `buildInsertStatement` walked either the input map or the struct
metadata `FieldsByName` map. Go map traversal made the column order unstable;
values followed that traversal, but exact SQL expectations could fail.
`RunTransactionWithHooks` -> Apply / `InsertHook` -> generic `Insert` ->
`buildInsertStatement` is the failing path; it does not use struct-to-map or
Query DSL planning. On main a397edb2a74bfed1183448a47c0ec9fb36b38380,
100 repetitions of each hook test produced 23 failing test invocations in total.
The observed users SQL had `(name, id)` instead of `(id, name)`.

Map inputs now select columns with the existing Columns/Omit rules, sort those
names lexically, then retrieve arguments using that same list. Struct inputs
reuse the existing InsertMany field-order helper. They follow declaration order,
including db tag names, with existing readonly/omitempty/ignored-field filtering.
For the hook user the SQL is now `INSERT INTO users (id, name) VALUES (?, ?)`
(with dialect quoting), arguments int64(1), string("alice"). PostgreSQL uses
numbered placeholders. Values are not converted or input maps mutated.

This intentionally changes SQL text/order from the previous unspecified order.
`Columns` is a selection set, not a requested ordering. Struct batch ordering
is preserved. RETURNING projection order and explicit `InsertUsing` destination
order are unchanged. Omitted columns still use database defaults; explicit nil
still binds NULL. Existing ID, RETURNING and affected-row behavior is unchanged.

Query DSL struct-to-map INSERT, map batch INSERT, ignore and DSL upsert already
sort columns in the internal builder. Generic InsertMany already sorts map keys
and uses struct declaration order. These paths are covered by existing tests
and the new single/batch column/value regression. Generic Upsert uses a separate
builder; its ordering and generic Update ordering are outside this correction.
No query policy, ownership, scanning, transaction or driver API is changed.

The hook tests match complete SQL and values, require commit after the audit
insert, and require rollback and errors.Is preservation on hook failure.
The regression also checks distinct batch values, typed nil, omitted/generated
fields, both dialect placeholders, input preservation and real DB round trips.

## Validation

Local toolchain: Go 1.26.4, WSL2 Ubuntu 24.04.4. MySQL 8.4.6 and PostgreSQL
16.10. Both DSNs and TEST_DB_DSN were explicitly supplied. Full suite
`go test ./... -count=1 -json` passed with no test-level skips; 20 packages had
no test files. `make test-integration` passed (custom driver DSN also supplied).
Related `go test ./orm ./orm/query ./orm/internal/querybuilder/... -race -count=1`
passed. Both hooks and the new order regressions passed `-count=100`.
These runs disable test-result caching. No simultaneous full suites used the DBs.

`go run ./cmd/goquent review --fail-on high ./...` exits 1 on both the fetched
main and this change. Both have 45 RAW_SQL_USED, four WEAK_PREDICATE, 11 destructive
and 11 blocked findings. This regression's readback adds one SELECT_STAR_USED
and one LIMIT_MISSING medium finding. No suppression, threshold or baseline was
changed. Partial/unsupported findings remain analysis limitations.

The example manifest verification exits 0: schema and policy match, fresh=true.
Generated-code and database fingerprints are absent and explicitly skipped;
this is not a live database freshness proof. No migration is introduced, so
migration planning is not applicable. PR-head CI status is recorded in the PR
and RelayWeft report separately from these local results.
