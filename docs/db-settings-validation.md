# GQ-AI-03/PR1 validation record

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

- TEST_MYSQL_DSN=root:password@tcp(127.0.0.1:3306)/testdb?parseTime=true
- TEST_POSTGRES_DSN=postgres://postgres:password@127.0.0.1:5432/testdb?sslmode=disable
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
