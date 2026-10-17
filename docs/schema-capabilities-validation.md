# GQ-AI-11 PR2 validation

Management ID: GQ-AI-11, planned PR2 (2/3),
[Issue #90](https://github.com/recoweft/goquent/issues/90).
The implementation contract and official DB references are in
[schema capabilities](schema-capabilities.md). Actual PR/head and final-head CI
URLs are recorded in the PR and Issue reference, not inferred here.

## Identity and start gates

Worker directly checked `/home/murai/github/goquent`, WSL2, Ubuntu 24.04.5,
`origin=git@github.com:recoweft/goquent.git`, and an initially clean working tree.
The request is work `65be037f-81ca-430f-aef8-b3e1245294b0`, revision 1,
execution `3e3d6181-1808-493e-83ca-11d9233bcbf0`.

Before edits/tests, PR91 was directly confirmed MERGED and its merge
`9e6896182e2726a0a0f35abf5d564556e3f41663` was the fetched latest origin/main.
Exact [CI 39009561792](https://github.com/recoweft/goquent/actions/runs/39009561792)
and [Docs 39009561772](https://github.com/recoweft/goquent/actions/runs/39009561772)
were completed/success on that SHA. PR53/56/71/72/74/75 were directly confirmed
merged, with every merge SHA passing `git merge-base --is-ancestor` against that
main. A case-insensitive revert-subject search returned no entries; this is not
an exhaustive proof against arbitrary semantic reversal.

AGENT.MD, applicable AGENTS.md locations, PR template, current contracts, model,
reader, comparison, source JSON, public views and facade were inspected. Worker
`gh` authentication and repository push/admin permission and SSH authentication
were checked independently of Issue connector credentials. The sandbox's initial
network/auth failure was resolved by the permitted network-enabled check.
Main protection returned `Branch not protected`, and applicable branch rules
were empty. Exact-head all-state PR search and remote branch lookup were empty
before creation. No work-search capability was supplied; duplicate-work state
was not independently queried, and no other task was selected.

Specification question `20662e92-2fd9-476b-9a4b-a77ac3c2d6e8` was answered and
adopted in this revision: additive model, exact tuple identity, per-item coverage,
conservative absence, source/public separation, no schema-wide v2 or PR3 evidence.
Implementation waited for answer delivery.

## Measured local environment and commands

Go: `go version go1.26.4 linux/amd64`.
Real DBs: MySQL **8.4.6**, PostgreSQL **16.10 (Debian 16.10-1.pgdg13+1)**.
Both project Compose services were healthy. DB suites ran sequentially with all
three variables explicitly exported: `TEST_DB_DSN` and `TEST_MYSQL_DSN` selected
the project MySQL fixture; `TEST_POSTGRES_DSN` selected its PostgreSQL URI fixture.
The values matched the repository Compose/Makefile defaults. No GOFLAGS test
flags were passed through the static loader. Raw logs stayed outside the repo.

| Command | Result |
| --- | --- |
| `go test ./... -count=1 -json` | Exit 0; **3,244 pass / 0 fail / 0 test skip**, 22 passing packages / 28 no-test packages |
| `go test -race ./... -count=1 -json` | Exit 0; **3,244 pass / 0 fail / 0 test skip**, same package counts |
| `make test-integration` with explicit three-DSN environment | Exit 0; 22 passing packages / 28 no-test packages; individual test events not counted |
| `go vet ./...` | Exit 0 |
| `go run ./cmd/goquent manifest check-repositories --config tests/typedfixture/repositories.json` | Exit 0; actual supplied-source regeneration comparison |
| `sh scripts/test-public-output-check.sh` | Exit 0; CI_WRAPPER_CHECK_PASSED |
| Built CLI `review --fail-on high --fail-on-precision partial --format review-ci-json ./...` | Exit 1; risk and precision gates fail |
| Source-API `review.Run` aggregate counter on `./...` | 1,359 findings; 0 suppressed; precise 0 / partial 1,186 / unsupported 173; no Run error |
| Built CLI review with actual staged main diff and archived main source root | Exit 2; input_failed; all 1,359 head findings unknown; no baseline or suppression |
| Example `manifest verify` with checked-in manifest/schema/policies | Exit 1; schema stale, policy OK, generated_code/database skipped |
| Built CLI `migrate plan docs/sql/command-store-mysql.sql` and PostgreSQL counterpart | Exit 0 each; disconnected structural planning only |
| `git diff --cached --check` | Exit 0 |

Counts are Go test/subtest events, not independent proof counts. Package `skip`
events with no tests are reported separately from test skips. The integration
Make target emits package results rather than individual JSON events; its per-test
counts are not inferred. Local source measurements use the final Go source;
this results document is appended afterward. Final-head CI is checked separately.
The local source inventory hash (sorted tracked .go/.mod/.sum paths, NUL, bytes,
NUL; SHA-256) is
`e91219a6a6eed0ad13b1898848642e95af64f4d108e014d2fe6a606994c3d365`.
This is local source correspondence, not a schema fingerprint, DB identity,
freshness, authorization, or a manifest gate bypass.

## Test coverage and limits

Real fixtures use dedicated namespaces `gq_ai11_pr2_a`, `gq_ai11_pr2_b`, and
`gq_ai11_pr2_visibility`, the existing PR1 fixture table, and a dedicated temporary
reader role. All DDL is fixture setup/cleanup. The reader wrapper accepts SELECT
only, and repeated export comparison confirms schema data remains unchanged.

- Both DBs: ordered/cross-schema FK, action/reference changes, CHECK expression,
  expression and compound index definitions, same-name tables/constraints across
  namespaces, raw retention, public canaries and export-to-desired comparison.
- PostgreSQL: partial predicates, NOT VALID state, hash indexes retained as
  unsupported/unknown, normal index DDL rollback and CONCURRENTLY-in-transaction
  refusal. MySQL: FULLTEXT retained as unsupported/unknown, partial-index syntax
  rejected, and actual DDL implicit commit. No lock duration/rebuild measurement.
- Both DBs: restricted role exposes one column but not a requested hidden table;
  inventory stays partial and cannot prove absence. Query/Scan/rows.Err failures
  at every catalog stage are injected with sqlmock, separately from real DB cases.
- Pure/API/CLI: identity/parent contradictions, scope mismatch, partial inventories,
  known-empty versus zero, legacy local differences plus unknown, old-consumer
  precision loss, enum/null/duplicate JSON, definition contradictions, detached
  snapshots, table/index/constraint additions/deletions with unknown, closed public
  vocabulary, bounded views, static nonconnection and structural execution refusal.
- Capabilities: missing/unknown version, declaration versus reader observation,
  JSON loss of observation, forged source labels, per-operation concurrency
  conditions, and equal plan/drift public axes. Supplied public views are display
  claims and never execution or approval inputs.

MySQL 8.0 and other untested releases/editions/engines, arbitrary quoting/case
semantics, invisible/zero-column inventory completeness, physical rebuild,
production concurrency, permission/freshness, and preflight/phase acceptance remain
unverified. PostgreSQL nondefault opclasses/collations/options/tablespaces,
exclusion/partitioned/invalid indexes and other unrepresented forms remain unknown.
No independent CI test-event counts or CI DB/toolchain versions are inferred from
its fixed-output success wrapper.

## Failure history and retained gates

Earlier attempts exposed sandbox cache/network restrictions, legacy test
expectations that now require explicit identity/coverage, and old Columns-only
mutations contradictory to exported ordered Keys. These were corrected without
promoting legacy unknowns. An initial MySQL fixture combined CHECK with an FK
column used by a referential action; MySQL rejected that setup. Separate fixture
columns now test those features, without changing library enforcement. A later
vet run rejected unkeyed literals after introducing public-view type aliases;
those literals were changed to keyed fields and validation was repeated.

Review gates still fail. Actual-main comparison still reports input_failed/all
unknown. The example manifest remains stale, with database and generated code
checks skipped. No thresholds, bulk baselines, suppressions, or cosmetic saved
fingerprints were changed. Earlier PR1 counts are historical evidence only.
Test/CI success does not adopt whole-Issue acceptance. PR3 evidence and phase work,
merge, and Issue closure are outside this revision.
