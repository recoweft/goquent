# GQ-AI-03 PR2 validation record

Work `7958f627-93cb-40aa-8558-cb77075d9d33`, revision 3, execution
`5635be5b-75e6-441f-90e5-0d056fa4a1c8`. [Issue #60](https://github.com/recoweft/goquent/issues/60).
Scope: PR2 only; no merge or Issue closure. See tenant-policy.md for conditional
assumptions, refusal boundaries and GQ-AI-04 exclusions.

## Identity and environment observed

The published request and fixed execution context agree on work/revision.
`pwd` was `/home/murai/github/goquent`; Ubuntu 24.04.4 on WSL2
(kernel 6.18.40.1-microsoft-standard-WSL2), Go 1.26.4 linux/amd64.
The initial working tree was clean. Fetch/push origin was the specified
recoweft/goquent SSH repository. Elevated fetch and GitHub authentication worked;
recoweft was active with repo scope and repository admin/push permissions.
Dry-run push to the requested branch succeeded. Actual PR creation is recorded
in the RelayWeft report, not inferred from permissions.

Issue search found #60 OPEN; all-state branch and tenant-content searches found
no existing PR2. PR61 was merged at f07b802c8b319a7b37b9f8b1b7f2629e2ef65166;
origin/main matched. PR53/56/58/59/61/63 merge commits were verified ancestors.
PR64 was CLOSED, unmerged; it was not changed. The specified new branch was
created from origin/main, without copying the previous branch's work.
AGENT.MD, ancestor AGENTS.md availability, PR template, contracts, settings docs,
API implementation and existing tests were inspected.

Sandbox fetch/network/auth checks initially failed; elevated read/auth/fetch
checks resolved those limitations. One later Go test invocation failed to write
the default build cache; subsequent tests explicitly used the writable repository
.gocache. No authentication failure was inferred from the sandbox-only attempt.

## Test evidence

DSNs were supplied explicitly for MySQL, PostgreSQL and the custom-driver test,
using local repository fixture settings. Values/credentials are not reproduced.
Shared-database suites ran serially. Server reads returned MySQL **8.4.6** and
PostgreSQL **16.10**. These observations validate the fixtures, not arbitrary
application-supplied schema or executor identity.

- Initial related package test: orm/query, internal builder API and orm passed.
- An intermediate alias/subquery test failed: a Query mistakenly passed to generic
  WhereIn was silently omitted by the compatibility builder. Strict WhereIn now
  rejects unsupported input shapes; the real WhereInSubQuery regression also
  refuses its unproven subquery. This failure was fixed, not skipped.
- Initial full `go test ./... -count=1 -json`, `make -s test-integration` with
  GOFLAGS=-json and full `go test -race ./... -count=1 -json`: each 1,170 passed,
  no failures or skips. These precede the final risk regression additions.
- After configured-high-risk enforcement and RETURNING readback: full test and
  full race each 1,171 passed, no failures/skips.
- Added user-method traps and different-policy parallel queries: related tenant
  race selection passed 60 test/subtest events, no failure/skip.
- Final `make -s test-integration` (underlying `go test ./... -count=1` with
  GOFLAGS=-json) and final `go test -race ./... -count=1 -json`: each **1,173
  passed, 0 failed, 0 skipped**, including the explicit custom-driver case.
- Both strict hook regressions ran 100 times each: 200 passed, no failure/skip.
- Contract registry/example/hash compatibility tests passed. No examples or
  manifest hashes were regenerated; no tests were deleted or skipped.

Counts include subtests, using go test JSON pass/fail/skip events. `-count=1`
disables test-result caching; no `(cached)` package result was observed. Go build
cache was reused and is distinct from test-result cache. Full suites retain the
existing predicate/ownership 65,535/65,536/65,537, budget/cycle/DAG, integer write
scope, settings/Tx/executor, scan/bool/dialect and INSERT-order/hook regressions.

New tests cover all-branch equality, unsafe OR/NOT/NULL/IN/range/column/raw cases,
outer-AND SQL/params/tree agreement and caller snapshots, old/missing/unsupported
context, schema mismatch/copy/budgets, alias/self/LEFT JOIN semantics, mixed INSERT
rows, protected writes, final generated RETURNING, both conflict dialects,
additional global unique keys, soft deletion, inheritance/concurrency, executor
zero-call rejection, Plan zero calls and private evidence mutation. Unsupported
JOIN forms and opaque SQL remain refusal boundaries, not theorem-prover coverage.

## Required tools and remaining diagnostics

`go run ./cmd/goquent review --format json --fail-on high ./...` returned exit 1
on both an archive of origin/main and this change. Main: 363 findings
(208 precise, 107 partial, 48 unsupported). PR2: 379 findings
(213 precise, 114 partial, 52 unsupported). Comparing code, severity, precision,
file and message while ignoring shifted line numbers: 16 added, 0 removed.
Additions are in tenant test files: 5 STATIC_REVIEW_PARTIAL,
4 STATIC_REVIEW_UNSUPPORTED, 4 LIMIT_MISSING, 2 BULK_UPDATE_DETECTED,
1 RAW_SQL_USED (server version read). They expose fixtures and static-analysis
limits. Existing blocked/destructive findings remain 11 each; high 53 -> 54,
medium 288 -> 303. No suppression, threshold or baseline modification hides them.

`go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json --format json`
returned exit 0: supplied schema and policy fingerprints matched. generated_code
and database checks were **skipped because fingerprints are absent**. The returned
Fresh boolean is not evidence of independent live verification or authorization.

`git diff --check` passed. Production migrations/DB deployment changes: N/A;
changes add ORM inspection and isolated test tables, not a production schema
migration. No migration apply was performed. No new dependency was introduced.

CI evidence belongs to the actual PR head/run/job in the report. Local success,
PR61 success and main CI are not evidence for this PR's CI. The existing CI
workflow supplies both database DSNs but not TEST_DB_DSN; its nonverbose output
does not establish a complete test-level skip count or custom-driver coverage.
