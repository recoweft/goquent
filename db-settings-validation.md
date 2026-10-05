# GQ-AI-03/PR1 revision 2 validation

Issue: https://github.com/recoweft/goquent/issues/60. Existing PR:
https://github.com/recoweft/goquent/pull/61. Refs #60; Refs #62.
Related maintenance: https://github.com/recoweft/goquent/pull/63.

## Identity, integration and final source

Request/context matched work `fa426a13-bb21-4ecf-b023-2494fb86e3fa`, revision **2**,
execution `2b2af33d-7da8-4ced-851d-06258dc9a718`. Measured cwd was
`/home/murai/github/goquent`, WSL2 Ubuntu 24.04.4, Go 1.26.4 linux/amd64.
Origin fetch/push: `git@github.com:recoweft/goquent.git`. Initial worktree was clean
on the maintenance branch. Authentication succeeded as recoweft outside the
sandbox; repository API grants push/admin and repo scope was present. Fetch and
push dry-run succeeded. Initial sandbox network/git-write failures were environment
restrictions, not test failures or evidence of invalid credentials.

All-state management-ID Issue search reused open Issue 60. Head-branch PR API
search and git confirmed existing open PR61, branch
`feature/gq-ai-03-policy-context`, base `main`, required title
`feat: scope policies and execution context to each DB`, old head
`330d5b2bc1ea1b04c6a0a33baea661d91009563f`. PR53/56/58/59 merges were independently
confirmed. PR63 API reported merged into main at 2026-10-05T13:37:15Z, merge SHA
`25fbda52b6ea19a9d691c21d52668d7b3d438c2c`; fetched origin/main matched.
AGENT.MD, ancestor/repository AGENTS.md search (none found), PR template,
workflows, contracts and settings/inspection entry points were checked.

`git merge --no-edit origin/main` on the existing feature branch produced
`46f593f2aff463bed696a8fc87036f1253405688`, without conflicts. The required main
SHA is an ancestor. No cherry-pick, duplicate implementation, rebase, force push,
new Issue or new PR. The merge brings the five maintenance files; this revision's
only additional edit is this validation document. All test runs below use the
final production/test source tree; subsequent documentation changes do not alter
it. Final publication SHA and matching CI run/job are recorded in PR61 and the
RelayWeft report to avoid a self-referential commit hash in this file.

## Current local validation (2026-10-05 UTC)

Both healthy compose services were queried: MySQL **8.4.6**, PostgreSQL **16.10**
(Debian 16.10-1.pgdg13+1). Explicit TEST_MYSQL_DSN and TEST_POSTGRES_DSN were used;
TEST_DB_DSN was explicitly set for registered custom-driver coverage. Values and
credentials are omitted. Full database suites ran sequentially.

| Command | Actual result |
| --- | --- |
| `go test ./orm -run 'TestRunTransactionWithHooks(CommitsAuditHook\|RollsBackOnHookError)$' -count=100 -json` | Exit 0; each hook passed 100 times (200 total), one passing package. |
| `go test ./... -count=1 -json` | Exit 0; 1,113 test/subtest passes, 18 passing test packages; 20 packages with no tests. |
| `GOFLAGS=-json make test-integration` with TEST_DB_DSN also supplied | Exit 0; 1,113 test/subtest passes, 18 passing test packages; 20 packages with no tests. |
| `go test -race ./orm/... ./tests ./tests/contracts -count=1 -json` | Exit 0; 1,089 test/subtest passes, 17 passing test packages; 18 packages with no tests. No race reports. |
| `go run ./cmd/goquent review --fail-on high --format json ./...` | Exit 1; 363 findings, zero suppressed. See limits below. |
| Same analyzer reviewing an archive of fetched origin/main | Exit 1; 340 findings. |
| `go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json` | Exit 0; schema/policy match. generated_code/database fingerprints missing: skipped/unverified. |
| `git diff --check` | Clean. No Go changes beyond the merge; no formatting edit needed. No migrations, so migration-plan check is not applicable. |

All four test commands: zero test-level failures/skips, zero cached results.
Go JSON package skip events correspond only to the explicitly listed packages
without test files. Both live settings/mysql and settings/postgres tests passed
in full and race runs, including Query/Tx/two-DB isolation. DB/external Tx/custom
Executor/registered driver/scan/BoolCompat and argument ownership boundaries
65,535/65,536/65,537, budgets/cycles/DAGs, condition tree and private write scope
remain covered by the executed suites.

PR63 permanently fixes the old randomized INSERT construction: map columns are
sorted after filtering; structs use the existing declaration-order helper; args
and placeholders follow the same columns. Full SQL/all-argument hook matching,
commit/rollback and errors.Is assertions remain intact. The old revision-1 local
failure and CI37293786684/job111710136153 at old head 330d5b2 are historical
failures, not the current result. No retry-until-success, weakened matcher,
AnyArg, skip/deletion, suppression or threshold/baseline change was used.

## Current review and manifest limits

Current findings: blocked 11, destructive 11, high 53, medium 288;
precise 208, partial 107, unsupported 48. Corrected main: blocked 11,
destructive 11, high 49, medium 269; precise 195, partial 100, unsupported 45.
Comparison by file/code/level/message/precision ignoring line shifts found
23 additions and no removals: one in orm/query/settings_test.go, 16 in
orm/settings_test.go, six in tests/settings_test.go. All are PR1 test fixtures;
no additional production-source finding was observed. The two maintenance
medium findings occur on both sides. These are analyzer observations, not a
safety guarantee. Manifest aggregate fresh=true does not establish missing
generated-code/database fingerprints or live freshness.

## Acceptance and remaining states

AC1/2: immutable settings/ownership, parallel two-DB isolation and parent
Tx/clone/wrapper inheritance pass local/full/race and both live DB tests.
AC3: construction-time legacy snapshots, explicit precedence and migration
remain implemented/documented and tested. AC4: missing/unconfirmed context stays
data, never authentication or trusted tenant proof. AC5/8: current command results
above replace the historical hook failure; head-matching CI is separately recorded
in PR61/report. AC6/9: existing Issue60/PR61, branch/base/title reused; PR2 plan
retained. AC7: corrected main is integrated by normal merge, no conflict.

PR1 implemented and locally validated; PR exists but remains unmerged. Issue60
remains open/incomplete; user acceptance has not been requested or recorded.
PR2 is not started and can be registered only after PR1 main merge. PR2 tenant/
write/PII semantics, GQ-AI-04 universal CRUD and unified Strict remain unimplemented.

---

The following is the historical revision-1 record. Its failures, pre-fix main,
and publication statements describe that earlier run, not revision 2.

# Historical revision 1 validation record

Issue: https://github.com/recoweft/goquent/issues/60. This records local checks for
PR1 only, before merge; it is not Issue completion or user acceptance.

## Identity and environment actually checked

The supplied request matches work `fa426a13-bb21-4ecf-b023-2494fb86e3fa`, revision 1,
execution `7716d29b-13fb-4e19-adab-b35c074adfc8`. `pwd` was
`/home/murai/github/goquent`; uname identified WSL2 and `/etc/os-release` identified
Ubuntu 24.04.4. `go version`: go1.26.4 linux/amd64. Origin fetch and push both point
to `git@github.com:recoweft/goquent.git`. The initial worktree was clean.

`gh auth status` succeeded outside the network sandbox as recoweft, with repo
scope. Repository API permissions included push/admin. `git fetch origin` and
`git push --dry-run origin HEAD:refs/heads/feature/gq-ai-03-policy-context` succeeded.
The API and fetched origin/main both identified
`a397edb2a74bfed1183448a47c0ec9fb36b38380`. PRs 53, 56, 58 and 59 each reported
merged=true and base=main. All-state Issue search found open Issue 60; all-state
head-branch PR search and local/remote branch search found no existing PR1.
The feature branch was created directly from that main, without copying unmerged
changes. AGENT.MD, applicable ancestor/repository AGENTS.md searches (none found),
PR template, CI workflow, current contracts and affected entry points were read.
These are performed checks, not assumptions about other machines or credentials.

## Commands and outcomes

Explicit local integration environment:

- TEST_MYSQL_DSN explicitly supplied (value omitted)
- TEST_POSTGRES_DSN explicitly supplied (value omitted)
- TEST_DB_DSN equals the MySQL DSN for registered custom-driver coverage.

These are disposable local test database credentials from the repository's compose
configuration. Both services were healthy. Actual SELECT version() results were
MySQL **8.4.6** and PostgreSQL **16.10** (Debian 16.10-1.pgdg13+1).

| Command | Result |
| --- | --- |
| `go test ./orm ./orm/query -count=1` | Initial sandbox attempt could not write Go cache; escalated rerun passed both packages. |
| `go test -race ./orm ./orm/query ./tests -run 'Test(DBSettings\|DBLegacySettings\|QuerySettings\|RiskEngineConfigOwns\|SettingsDatabase)' -count=1 -v` with both explicit DSNs | Initial new tests failed because one expected only LIMIT_MISSING while also selecting all columns, and a test used unsupported scalar bool scanning. Corrected test inputs to explicit id projection and supported struct bool field; rerun passed all three packages, both DBs, no skips/cache or race reports. |
| `TEST_DB_DSN=… GOFLAGS=-json make test-integration` | Exit 0; invokes explicit-DSN `go test ./... -count=1`. 1,106 test/subtest passes, 18 passing test packages, 20 packages without tests. No test skips, failures or cached results. |
| `go test -race ./orm/... ./tests/contracts -count=1 -json` | Exit 0; 794 test/subtest passes, 16 passing test packages, 18 without tests. No test skips/cache or race reports. |
| `go test ./... -count=1 -json` with all three explicit DSNs, after adding raw-gate/soft-delete regression | Exit 1: 1,106 test/subtest passes, one pre-existing hook failure; 17 passing test packages, one failed (orm), 20 without tests. No test skips or cached results. Both DB integration subtests passed. |
| `go test -race ./orm -run 'TestDBSettingsRawRiskAndSoftDeleteUseLocalSnapshots\|TestDBSettingsUnsupportedAndMissingContext' -count=1 -v` | Exit 0, both tests pass, no cache/race. Covers the final added tests. |
| `go -C /tmp/gq03-main-7716 test ./orm -run TestRunTransactionWithHooksRollsBackOnHookError -count=30 -json` against an archive of unchanged origin/main | Exit 1: 29 passes, one identical column-order failure. Establishes baseline reproduction; no production/test workaround applied. |
| `go run ./cmd/goquent review --fail-on high --format json ./...` | Exit 1; 361 findings, none suppressed. See classification below. |
| Same review CLI against an archive of origin/main source | Exit 1; 338 findings. Used the same analyzer to compare source findings, not historical CI. |
| `go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json` | Exit 0, schema/policy match. generated_code/database fingerprints absent: skipped and unverified despite aggregate fresh=true. No live schema freshness claim. |
| `gofmt` on changed Go files; `git diff --check` | Clean. No migrations; migration-plan check not applicable. |

The final full-suite failure is
`TestRunTransactionWithHooksRollsBackOnHookError`: the generated INSERT column
order was name,id while sqlmock expected id,name; subsequent rollback expectation
also fails. The unchanged main reproduced the same problem. This known hook/map
ordering issue is outside PR1 and remains unfixed, unskipped and unsuppressed.
The earlier successful full run is not substituted for the final failed run.

The full/race suites retain the 65,535/65,536/65,537 argument ownership regressions,
payload budgets/cycles/DAGs, condition-tree SQL correspondence, private write-scope
evidence and integer bounds. Both live DB suites retain raw/DB/Tx/external Tx,
registered custom driver, custom Executor, struct/map scan and BoolCompat tests.
New live tests concurrently run two DB configurations on one same-named table,
through Query, Count, transaction, parent WrapTx/WrapExecutor and standalone
explicitly configured sql.Tx/custom wrappers. This proves isolation/inheritance,
not trusted-value tenant equality or all-branch semantics.

## Static review and manifest limits

Current findings: blocked 11, destructive 11, high 53, medium 286;
precision precise 206, partial 107, unsupported 48. Baseline: blocked 11,
destructive 11, high 49, medium 267; precise 193, partial 100, unsupported 45.
Comparing file/code/level/message/precision (ignoring line shifts) found no removed
findings and 23 additions, all in new tests:

- orm/query/settings_test.go: one partial chain.
- orm/settings_test.go: four partial chains, six missing-limit, three raw SQL,
  three select-star warnings from deliberate inspection/rejection test cases.
- tests/settings_test.go: two partial chains, three unsupported dynamic raw SQL
  cases in fixture setup/delegation, one raw SQL warning for the bool scan.

These are reported findings, not suppressions or safety proof. No added production
source finding was observed by this analyzer; its partial/unsupported limits remain.
The five AI example fixtures are unchanged and their baseline hash tests pass.
Manifest verification does not attest live schema or generated-code freshness.

## Remaining review states

PR creation, remote head and CI run/job state are recorded in the PR and RelayWeft
report, after publication. Nothing here claims merge, user confirmation, unified
Strict, tenant authorization, PR2 semantics or GQ-AI-04 universal CRUD enforcement.
