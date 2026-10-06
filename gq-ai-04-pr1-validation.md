# GQ-AI-04 PR1 validation

Work `37fae458-818a-4b63-91da-78b53ba973e2`, revision 1, execution
`35d4ce7d-3b51-484e-8454-ecf6b240ff52`.
[Issue #66](https://github.com/recoweft/goquent/issues/66).
Only PR1 of 3; no merge or Issue closure. See
[the entry inventory and contract](planned-query-execution.md).

## Identity and baseline checks actually performed

The supplied request agrees with the fixed work/revision context. `pwd` was
`/home/murai/github/goquent`; uname and os-release identified WSL2
6.18.40.1-microsoft-standard-WSL2 and Ubuntu 24.04.4 LTS. Go was
`go1.26.4 linux/amd64`. The initial working tree was clean on the previous
GQ-AI-03 branch. Fetch/push origin was `git@github.com:recoweft/goquent.git`.

Sandbox-only DNS/auth checks failed; network-enabled checks then verified the
active recoweft account, repo scope, repository admin/push permissions, SSH
ls-remote/fetch and dry-run push to the specified branch. No invalid credential
conclusion was drawn from the sandbox attempt. PR creation/push results belong
to the actual PR report, not this permission check.

Fetched origin/main was `e81b8b4e93beab7104059b1539980a55c2349b0d`.
GitHub reported PR65 MERGED at that SHA (2026-10-05T17:56:30Z). PR58, PR59 and
PR61 also reported merged; all four merge SHAs were verified ancestors of
origin/main. Issue #66 was OPEN. All-state specified-branch PR search returned
none; the GQ-AI-04 content search found no existing PR1. The specified branch
`feature/gq-ai-04-execution-core` was created from origin/main. No revert or
prior-branch work was carried over. No alternate work was selected or started;
the separate controller work registry was not independently queried.

Read AGENT.MD, checked applicable ancestor/nested AGENTS.md availability, read
the PR template, contracts v3 and tenant-policy documentation, and inspected
Query, OperationSpec, builder snapshots, DB Settings/executor/transaction and
scoped/generic entry code. Local .git and Go cache writes needed the managed
execution permission path; no unrelated files were overwritten.

## Runtime verification

The repository's healthy MySQL 8 and PostgreSQL 16 containers were reused.
Server version queries returned **MySQL 8.4.6** and **PostgreSQL 16.10**.
These are fixture observations, not independent verification of application
schema assertions. DSNs used the repository's local fixture settings; explicit
TEST_MYSQL_DSN and TEST_POSTGRES_DSN made unavailable connections fail. The
final suites additionally set TEST_DB_DSN for the registered-driver test.
Shared-database suites ran sequentially.

| Command / scope | Observed result |
| --- | --- |
| Initial Query/builder unit run | Found RETURNING failure-path lifecycle regression; fixed without weakening its existing test |
| Initial new spy test run | Count SQL fixture and blocked-error expectation corrected; final assertions retained |
| `go test ./... -json` | 1,260 test/subtest passes, 0 failures, 1 existing custom-driver skip (TEST_DB_DSN absent) |
| Initial `make test-integration GOFLAGS=-json` | 1,260 passes, 0 failures, same custom-driver skip |
| Final `TEST_DB_DSN=<fixture> make test-integration GOFLAGS=-json` | 1,264 passes, 0 failures, 0 test skips; underlying `go test ./... -count=1` |
| Final `go test -race ./... -count=1 -json` with all three explicit DSNs | 1,264 passes, 0 failures, 0 test skips, including cleanup of unused snapshot fields/helper |
| `git diff --check` | Passed |

Counts are JSON test pass/fail/skip events including subtests. Final suites had
18 passing packages and 20 packages with no test files (package skip events,
not skipped tests). No failing test was deleted or suppressed. Go build cache
was reused; final `-count=1` runs did not use test-result caching.

New tests cover every Query terminal in both dialects and context/noncontext
modes, Plan zero calls, denied terminal zero calls, exact dispatch method/count,
SQL/argument correspondence, scan values, RowsAffected/LastInsertId/RETURNING,
plan/execution diagnostic parity, private lifecycle consumption, public verdict/
SQL/argument/target mutation, JSON loss of private provenance, RETURNING PII
rejection and reinspection, bool scan compatibility and row closure on errors.
The full suites retain PR65 tenant/all-branch/JOIN/soft-delete/conflict/protected
column/high-risk/context-copy regressions, sql.Tx/custom Executor and registered
driver coverage, hook regressions, generic/scoped/RETURNING and contract fixtures.
Their success does not establish safety for unparsed cases or deferred entries.

## Required review and manifest checks

`go run ./cmd/goquent review --fail-on high ./...` returned exit 1. JSON runs of
the same analyzer against a fresh origin/main archive and this source compared
code, level, precision, file and message, ignoring shifted line numbers:

- Main: 379 findings (11 blocked, 11 destructive, 54 high, 303 medium).
- PR1: 416 findings (11 blocked, 11 destructive, 54 high, 340 medium).
- 37 added, 0 removed: six unsupported dynamic dispatch findings in
  execution.go, six unsupported spy-forwarding findings and 25 partial findings
  in execution_test.go. These are real static-analysis limitations; runtime tests
  do not turn them into static proof.

No suppressions or thresholds changed. An initial comparison reused a preexisting
temporary archive directory and included unrelated files; it was discarded and
replaced with a uniquely created clean archive for the counts above.

`go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json --format json`
returned exit 0: supplied schema/policy fingerprints match. generated_code and
database fingerprint checks were **skipped because fingerprints are absent**.
Aggregate fresh=true is not live schema freshness, authentication or approval.
No example/manifest was regenerated to hide this baseline limitation.

Production migration plan/apply: not applicable (no production schema change).
Benchmarks were not run; no performance improvement claim is made. CI status,
actual PR URL and tested head are reported separately after publication; previous
PR/main CI cannot establish PR1's head status. The existing CI workflow does not
supply TEST_DB_DSN, so its output alone does not establish registered-driver
coverage or a complete test-level skip count.

## Acceptance and remaining work

1. The entry inventory identifies all actual Query terminals, inherited scoped
   writes, nonexecuting plans and separate PR2/PR3/Raw boundaries.
2. INSERT/UPDATE/DELETE target and assignment metadata now comes from the rendered
   builder snapshot. A private execution record preserves the inspected statement
   and gate; public diagnostic fields cannot redirect or unblock dispatch. Final
   PostgreSQL ID RETURNING is re-finalized and recorded before dispatch.
3. Spy tests prove DB-free planning, zero-dispatch rejection, single dispatch and
   expected statement/arguments/results for covered terminals.
4. Existing PR65 policy/risk/tenant and Settings/Tx tests pass unchanged. The
   application trust and supported-domain limits remain documented and enforced.
5. Both DB fixture suites, custom executor/Tx/scanning/bool/hooks and full race
   verification passed as recorded; static review/fingerprint gaps remain open.
6. The requested branch/title, Refs #66 and Issue link are used for the actual PR;
   publication details belong to the execution report. No merge is authorized.

Generic CRUD/nonexecuting generic plans remain PR2. Compound parent/child plans,
nested/hook-wide enforcement, full Raw boundary completion and cross-entry
acceptance remain PR3. GQ-AI-05/06/11/13 artifact versioning, complete output
masking, live freshness and external permits are not implemented here. This is
not completion of Issue #66 or its overall acceptance criteria.
