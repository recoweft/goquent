# GQ-AI-04 PR2 validation

Work `02fa0f75-ce5d-48a9-9131-46969d9aa3db`, revision 1, execution
`fc423680-d48e-49a1-aab6-b76b8cfd85d6`.
[Issue #66](https://github.com/recoweft/goquent/issues/66), PR2 of 3.
Implementation/test source commit: `d779c9dac17108adaf16a42d3057cfb93daf9efd`.
This validation document is a subsequent documentation-only commit. Actual PR,
final head and CI run are reported through RelayWeft after publication. No merge,
Issue closure or user completion is performed.

## Checks actually performed

The supplied work/revision match the fixed execution context. `pwd` matched
`/home/murai/github/goquent`; uname identified WSL2
6.18.40.1-microsoft-standard-WSL2, and os-release identified Ubuntu 24.04.4 LTS.
`go version` returned `go1.26.4 linux/amd64`. Initial working tree was clean on
`feature/gq-ai-04-execution-core`, head ded33b22151fc6b7f23835edc87e43d25b3f4565.
Origin fetch/push URL was `git@github.com:recoweft/goquent.git`.

The first sandbox-only network/auth checks failed. Network-enabled checks then
confirmed active worker account recoweft, repo scope, repository admin/push
permissions, SSH ls-remote/fetch and successful dry-run push to the specified
branch. This worker authentication check is separate from the Issue connection.
No invalid-credential conclusion was drawn from the sandbox result.

Remote/main and fetched origin/main were
`8a6360ae93c3c96bc187357d6fada631f7e2410c`. GitHub reported PR67 MERGED at that SHA,
2026-10-06T05:26:39Z. Issue #66 was OPEN. Specified remote branch was absent;
all-state PR search for that branch returned no PR. The specified PR2 branch was
created from origin/main. The separate controller work registry was not queried;
no other work was selected or started. Earlier dependency PRs' statuses were
supplied baseline information, not individually reverified in this execution.

Read AGENT.MD, checked ancestor/nested AGENTS.md availability, read the PR
review template, contracts v3, tenant-policy and planned-query-execution docs,
and inspected generic/scoped/raw/RETURNING, Query/private lifecycle, builders,
settings, scanners and transaction helpers. Initial .git writes and Docker access
needed managed permission escalation. Go cache writes under the home directory
failed once; subsequent tests used writable `GOCACHE=/tmp/gq-pr2-go-cache`.
No unrelated files, dependency inventory, thresholds or suppressions were changed.

## Specification consultation

Question `b42d9d08-2904-4802-a7f5-77040983e7f6` concerned WriteOpt fields that
UpdateByReturningWithOptions historically ignores. The controller delivered the
answer to this execution and it was read using that exact question ID.
Candidate A is implemented: Returning/NoRowsAs remain effective; other fields
retain nonapplication, including ExpectAffected. Options are still evaluated.
Actual base/scopes/data/final projection are inspected using the destination DB;
ignored Columns/Omit/table/assignment options cannot hide actual dangerous data.
See [the option contract and inventory](planned-query-execution.md).

## Runtime verification

Healthy repository containers were reused: MySQL image mysql:8 and PostgreSQL
image postgres:16. Fixture version reads returned **MySQL 8.4.6** and
**PostgreSQL 16.10**. These fixture observations do not verify application schema
assertions, physical connection identity or live freshness.

All final commands below ran at implementation commit d779c9d, with:

- `GOCACHE=/tmp/gq-pr2-go-cache`
- `TEST_MYSQL_DSN=root:password@tcp(127.0.0.1:3306)/testdb?parseTime=true`
- `TEST_POSTGRES_DSN=postgres://postgres:password@127.0.0.1:5432/testdb?sslmode=disable`
- `TEST_DB_DSN=root:password@tcp(127.0.0.1:3306)/testdb?parseTime=true`

Shared-database suites ran sequentially. Explicit MySQL/PostgreSQL DSNs make
connection failures fail; TEST_DB_DSN also enables the registered custom-driver
fixture. Counts below are test/subtest JSON events, not inferred from package
summaries.

| Command | Result |
| --- | --- |
| `go test ./... -count=1 -json` | Exit 0; 1,358 pass, 0 fail, 0 test skips |
| `make test-integration GOFLAGS=-json` | Exit 0; 1,358 pass, 0 fail, 0 test skips; underlying `go test ./... -count=1` |
| `go test -race ./... -count=1 -json` | Exit 0; 1,358 pass, 0 fail, 0 test skips |
| `go test ./tests -run TestGenericPlannedDatabaseSemantics -count=1 -v` with both explicit DB DSNs | Both dialect subtests pass; fixture versions above |
| `git diff --check` | Passed |

Each final full suite had 18 passing packages and 22 packages without test files.
The latter are package skip events, not skipped test cases. No test-result cache
was used in final suites. Earlier all-package smoke testing had 1,263 pass and one
custom-driver skip before the added tests and explicit TEST_DB_DSN. During edits,
compile errors from the mechanical extraction, SQL fixture grouping expectations,
and a missing automatic-tenant conflict-column ordering case were found and fixed.
No failing safety assertion was removed or suppressed.

New unit/integration coverage includes generic Plan zero executor calls; all-six-
method rejection spies; every generic write/typed RETURNING/Many family; final
PII/forbidden projection rejection; all-candidate tenants; protected assignments;
missing tenant/schema; conflict coverage differences; automatic tenant fill;
context/noncontext dispatch; SQL/argument order and literal primary-key quoting;
private one-use lifecycle and public/JSON mutation resistance; opaque driver values
without planning-time Valuer/Stringer/marshaler calls; compatibility driver
conversion; generic/scoped bool scans, row closure and scan errors; NoRowsAs error
identity/option order; scoped ignored-option groups; cross-dialect/destination
settings/tenant rebinding; alias/all-branch conditions; source soft-delete isolation;
parent/external transactions; hook inheritance; and strict raw-shaped read refusal.
Existing Query, tenant, JOIN, expiry, scanning, nested, hook, custom Executor,
registered driver and sql.Tx regressions remain in the full suites.

## Review and manifest limitations

`go run ./cmd/goquent review --format json --fail-on high ./...` returned **exit 1**.
A clean origin/main archive was reviewed with the same command. Comparison uses
code/level/precision/file/message, ignoring shifted line numbers:

| Source | Blocked | Destructive | High | Medium | Total |
| --- | ---: | ---: | ---: | ---: | ---: |
| Main / PR1 baseline | 11 | 11 | 54 | 340 | 416 |
| PR2 | 11 | 11 | 55 | 344 | 421 |

There are 10 added-location findings and 5 removed-location findings (net +5).
The UPDATE_WITHOUT_WHERE finding moved from query.go to generic.go: static
reconstruction sees the internal builder's Update before CopyStateToUpdate and
cannot establish the later copied conditions. This is a static-analysis limitation,
not an omitted runtime gate or a static safety proof. Three old scoped partial
findings and one old write lookup partial finding disappeared with those adapters.

Other additions: one partial bulk-scope fixture, two partial reconstruction
findings (new adapter and test), five unsupported dynamic fixture SQL findings,
and one High RAW_SQL_USED for the integration fixture's explicit SELECT version().
Final precision counts: 214 precise, 138 partial, 69 unsupported. Suppressed: 0.
No threshold, rule, fixture finding or manifest was changed to hide these results.
Runtime tests do not resolve unsupported static analysis.

`go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json --format json`
returned exit 0. Supplied schema/policy fingerprints matched. **generated_code and
database checks were skipped because their fingerprints are absent**. The aggregate
fresh=true is not live freshness, authentication or authorization.

Production migration commands are not applicable (no production schema change).
Benchmarks were not run; no performance claim is made. No new dependencies were
added. Arbitrary custom dialect implementations were not tested; supported MySQL/
PostgreSQL dialects, custom executors and the registered MySQL driver were tested.
The existing CI does not set TEST_DB_DSN; CI success alone does not prove zero skips
or registered-driver coverage. External SonarQubeCloud/vi-push checks are reported
separately and are not substitutes for these test counts.

## Acceptance and exclusions

1. Current inventory covers the requested generic/scoped entries, raw read/scanner
   boundary, constituent helpers and PR3 ownership. Query/generic diagnostic parity
   is tested for supported equivalent operations.
2. Five generic diagnostic Plan APIs use the same private structural preparation
   and finalizer, without DB/Executor/EXPLAIN/schema reads. Public diagnostics/JSON
   never enter the dispatcher as execution inputs.
3. Rejected PR2 statements make zero Executor calls. Both dialects' supported
   execution, driver results, scans, protected columns and tenant/conflict refusals
   are covered. Opaque values retain compatibility driver semantics, not proof.
4. Final RETURNING and separately supplied destination settings/context/dialect
   are inspected and bound before dispatch. Source public SQL/Params/verdicts are
   not trusted, and source-generated soft-delete conditions do not contaminate a
   newly planned destination.
5. Required unit/full/integration/race/custom-driver/review/manifest checks above
   were run, with limitations stated. PR1 lifecycle, expiry and one-use tests pass.
6. PR publication, actual URL/final head and CI runs are recorded in the RelayWeft
   report. This document does not claim merge or Issue-wide completion.

Split bulk parent/child plans, nested/RunIdempotentCommand/hooks-wide orchestration
and full Raw boundaries remain PR3. Existing DB.QueryRow/QueryRowContext rejected-
query transport may still call an executor. Direct SQLDB/driver/Tx/Executor calls
are outside interception. A compound call can execute a valid constituent before
another fails; no whole-operation safety, cardinality, automatic rollback, retry,
external permit, artifact version, authentication or live schema guarantee is added.
