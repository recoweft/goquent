# GQ-AI-09 PR1 validation

Issue: https://github.com/recoweft/goquent/issues/82. Planned PR: PR1 (1/3).
Branch: `feature/gq-ai-09-review-types`; base: `main`.
The exact published head and GitHub CI run are recorded in the PR and execution
report, separately from this local validation record.

## Identity and starting state

The worker checked the requested directory, Ubuntu 24.04.5 on WSL2, repository
remote, branch and clean initial worktree. Local HEAD and remote main were
`ff01a7c954994998e662ecf5be54c02f83375739`; ancestry and PR81 merge were checked.
Main CI run 38565946298 and docs run 38565946395 had completed successfully.
Worker GitHub authentication, repository push permission and SSH push dry-run
succeeded. All-state head-branch PR search and remote-branch lookup found no
existing PR/branch; the Issue search found the existing open #82. No new work or
Issue was registered. Dependency PR merge checks supplied in the request were
not independently repeated; they are distinct from these worker observations.
Initial sandbox network failures were resolved with authorized network access.

## Local verification

Toolchain: `go version go1.26.4 linux/amd64`. Test services: MySQL 8.4.6,
PostgreSQL 16.10; Docker server 29.8.0. No production database was used.

| Command / scope | Result |
| --- | --- |
| `go test ./... -json` before explicit DSNs | Passed after the documented C01 fixture expectation migration; one registered-driver test skipped. This is not full integration evidence. |
| `make test-integration` with explicit TEST_DB_DSN, TEST_MYSQL_DSN, TEST_POSTGRES_DSN | Passed with the repository's MySQL/PostgreSQL test services. |
| Final `go test ./... -count=1 -json` with the same three explicit test DSNs | 22 packages passed; 2,718 test/subtest pass events; zero test skips/failures; 26 packages had no tests. Counts are events, not distinct top-level tests. |
| `go test -race ./orm/review ./cmd/goquent` | Both packages passed on the final implementation. |
| `go vet ./...` | Passed. |
| `sh scripts/test-public-output-check.sh` | CI output boundary scenarios passed. |
| `go run ./cmd/goquent manifest check-repositories --config tests/typedfixture/repositories.json` | Passed; supplied-source regeneration comparison only. |
| `go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json --format json` | Exit 1: schema stale, policy OK, generated_code/database skipped. This existing limitation is unresolved. |
| `gofmt` and `git diff --check` | Passed. |

DB suites were not run concurrently with another DB suite. Race checks target
review/CLI only. The changes do not alter execution, dialect, Strict or private
binding semantics. Raw test/driver/source diagnostics were kept local, not
published as evidence. No-test packages are not represented as passing tests.

## Review comparison

Both snapshots were actually reviewed: main was extracted from the exact commit
above, and the implementation was reviewed from the working source tree. The
public CLI command was `go run ./cmd/goquent review --fail-on high ./...`.
Both exited 1. The 256-finding public display cap did not limit the original
reports used to compute this comparison. No threshold changes, bulk suppressions
or input exclusions were used.

| Measure | Main | PR1 |
| --- | ---: | ---: |
| Findings | 499 | 1,122 |
| Suppressed | 0 | 0 |
| Precise | 253 | 160 |
| Partial | 157 | 854 |
| Unsupported | 89 | 108 |
| Blocked risk | 11 | 4 |
| Destructive risk | 23 | 24 |
| High risk | 81 | 109 |
| Medium risk | 384 | 985 |

Matching original code/file/physical-position tuples across unchanged source lines
retained 425 of the 499 original findings. Of the 74 unmatched original findings,
68 were name-based diagnoses on SQL-builder construction methods (including
internal builder tests and builder calls inside the Query implementation), not
DB/Query execution terminals. This accounts for all nine old
UPDATE_WITHOUT_WHERE diagnoses removed at those locations, plus two old
DELETE_WITHOUT_WHERE diagnoses. These are false-positive corrections, not
suppressions. Real Query operations now retain their own diagnostics.

The other six were old partial markers on Query variables now reconstructed:
four supported literal read shapes need no finding; one retains LIMIT_MISSING;
one retains DELETE_WITHOUT_WHERE. The audit also caught and corrected lost
Executor calls and missing projection/limit diagnoses on dynamic tables before
final validation. The final API audit uses an explicit generic operation list: option/hook
factories are excluded, and typed operation execution adapters are included.
New generic, method-value, local-variable and incomplete-flow
markers explain much of the increase. New source/test code also contributes to
the full-tree count; this is not a controlled performance or accuracy benchmark.

## Acceptance mapping and remaining work

- AC1: declaration/receiver/generic identity; aliased imports, shadowing, unrelated
  APIs, Executor, method expression/value and callable escape fixtures.
- AC2: local mutable aliases, assignment ordering, conservative branches, known
  scopes, one-level helpers, opaque/dynamic/loop boundaries, partial type success,
  external test variants, cancellation, input budgets and per-Run isolation.
- AC3: existing source wire and public views retained; loader and JSON canaries;
  unverified QueryPlan marker independent of supplied warnings/suppression;
  recomputed read/write risk retained. No runtime authorization changes.
- AC4: preserved original snippets and typed counterparts; documented C01/JSON
  expectations; SQL, suppression/expiry, gates, privacy and baseline comparison.
- AC5: local results above; published-head CI is recorded separately in the PR.
- AC6: requested branch/base/title and Refs #82; actual PR link added to Issue82
  after creation without checking whole-Issue acceptance boxes.

Scope and compatibility details are in [typed static review](static-review-types.md).
Best-effort loading, arbitrary application dispatch/interprocedural analysis,
complex SQL recovery and opaque callbacks remain limitations. PR2 runtime-plan
correspondence/multi-axis precision and PR3 SARIF/diff review/baselines are not
implemented. MigrationPlan provenance and live manifest freshness are not proved.
PR creation and passing tests are not merge, whole-Issue acceptance or user completion.
