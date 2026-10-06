# GQ-AI-04 PR3 validation

[Issue #66](https://github.com/recoweft/goquent/issues/66), PR3 of 3.
Work `1cb93652-1caf-48e7-a631-1f3dd023269b`, revision **3**, execution
`5723e31a-8b47-4ad2-8751-b2455b29dfce`.
Implementation/test source: `bf55890d76639d1cff05506c8d7479392c32ce70`.
The subsequent documentation commit changes prose/API comments only. Actual PR,
final head and head CI are reported through RelayWeft following publication.
No merge, Issue closure or user completion is performed.

## Identity, environment and prerequisites actually checked

The supplied request work/revision matched the fixed execution context. `pwd`
matched `/home/murai/github/goquent`; uname identified WSL2
6.18.40.1-microsoft-standard-WSL2 and os-release Ubuntu 24.04.4 LTS. The initial
tree was clean on the requested `feature/gq-ai-04-compound-writes` branch at
`c30f8dc1e984ea63005eec0dc57ed700acef06d0`. Origin fetch/push URLs were
`git@github.com:recoweft/goquent.git`. Local origin/main, remote ls-remote main,
and GitHub PR68's MERGED commit matched this SHA; mergedAt was
2026-10-06T06:55:26Z. Issue66 was OPEN and all-state PR search for the specified
head returned no PR. Existing local branch was reused; PR1/2 were not copied.

Initial sandbox-only gh authentication failed. A network-enabled repeat confirmed
active worker recoweft, repo scope, admin/push permissions and successful git push
dry-run to the specified branch. These are worker credentials, not verification
of the controller's separate Issue-connection authentication. Actual push/PR
creation are separately reported after they occur. The fixed work was used;
no start_work, alternate work selection, delegation or historical get_answer was
performed. Older dependencies' individual merges were supplied baseline facts,
not independently rechecked beyond PR68 and the main SHA in this execution.

Read AGENT.MD, checked ancestor and repository AGENTS.md availability (none
applicable found), read the PR template, contracts v3, tenant-policy and planned
execution docs, and inspected Query/generic/scoped/Raw/compound/scanning/Tx,
private bridge, builders and snapshot paths. `go version` was
`go1.26.4 linux/amd64`. The installed Go1.26.4 database/sql source was inspected:
Row has private error/rows fields and public Scan/Err, not an error constructor.
No unsafe/reflection/private-field or custom-driver workaround was added.

## Adopted revision conditions

Revision 3 explicitly publishes the earlier proposals as implementation conditions;
their historical pending labels were not treated as present blockers. Implemented:

- Public `*orm.Row` for DB.QueryRow/QueryRowContext; six-method Executor and
  QueryRowE signatures unchanged. Original refusal identity with zero dispatch.
- Strict opaque nested ID callbacks, hooks/idempotent recipe entries, and every
  nonnil nested Scope rejected before callbacks/Begin/known statements. Built-in
  Scope factories and SkipParent/empty Children do not create exemptions.
- Supported static constituents receive private ordered preflight. Compatibility
  scopes keep their original position and single evaluation; later changed delete
  and child inputs receive new destination-bound plans. Dynamic grandchild plans
  are added only after their factories execute, not claimed prevalidated.
- MySQL contiguous-ID guessing removed. Per-row MySQL LastInsertId and per-row PG
  inspected RETURNING establish input/ID correspondence. Evaluated options are
  reused; MySQL affected options apply to the aggregate, PG typed-Many option
  nonapplication is retained. Existing scoped RETURNING option contract is unchanged.

No new specification question was needed in this execution. Historical question
IDs are provenance only, not answer-delivery targets.

## Runtime checks

Healthy repository containers were reused, with observed versions MySQL **8.4.6**
and PostgreSQL **16.10**. All final suites ran sequentially at bf55890 with:

- `GOCACHE=/home/murai/github/goquent/.gocache`
- `TEST_MYSQL_DSN=root:password@tcp(127.0.0.1:3306)/testdb?parseTime=true`
- `TEST_POSTGRES_DSN=postgres://postgres:password@127.0.0.1:5432/testdb?sslmode=disable`
- `TEST_DB_DSN=root:password@tcp(127.0.0.1:3306)/testdb?parseTime=true`

| Command | Observed result |
| --- | --- |
| `go test ./... -count=1 -json` | Exit 0; 1,427 test/subtest pass, 0 fail, 0 test skips |
| `make test-integration GOFLAGS=-json` with explicit DB DSNs | Exit 0; 1,427 pass, 0 fail, 0 test skips |
| `go test -race ./... -count=1 -json` | Exit 0; 1,427 pass, 0 fail, 0 test skips |
| `git diff --check` | Passed |

Each suite reports 18 passing packages and 22 packages without test files; those
package skip events are not skipped test cases. Counts are parsed JSON events,
not inferred from CI green. Initial home-cache writes failed due to sandbox
permissions; workspace cache resolved that. During edits, existing nested fixtures
expecting contiguous IDs/one PG multirow statement failed and were updated to the
intentional per-row contract, including noncontiguous IDs. A test policy-field
compile typo was corrected. No failing safety test was hidden or skipped.

New tests cover all-six-method refusal spies, Raw and Row error identity/delegation,
context/plain dispatch and no redispatch, no-row/driver/scan/bool outcomes,
known later child/projection/delete refusal before parent/Begin, opaque callback
counters and panic/effect bodies not called, built-in and late scopes, private
plan/diagnostic mutation, tenant destination rebinding, external/caller-owned Tx,
compatibility scope timing/single evaluation/alternate Query destination and later
input changes, split input ranges, aggregate options and partial driver/result
failures. Existing generic/Query parity, policy, RETURNING, settings, expiry,
one-use, bool, hooks and idempotent regressions also ran.

`TestCompoundGeneratedIDCorrespondence` runs on both databases, with allocation
increment **7** on the actual transaction connection/PG sequence. Through a custom
Executor over caller-owned sql.Tx it verifies each actual ID against input_key,
assigned ID and the grandchild foreign key. It does not infer arbitrary custom
Executor honesty or universal multirow RETURNING order.

## Static review and manifest

`go run ./cmd/goquent review --fail-on high --format json ./...` returned **exit 1**.
The same unchanged CLI reviewed an archive of main c30f8dc at `/tmp/gq-pr3-main`
for comparison, also exit 1. No thresholds, suppressions or rule files changed.

| Source | Blocked | Destructive | High | Medium | Total |
| --- | ---: | ---: | ---: | ---: | ---: |
| main c30f8dc | 11 | 11 | 55 | 344 | 421 |
| PR3 bf55890 | 11 | 15 | 64 | 357 | 447 |

Line-independent path/code/severity/precision multiset comparison gives 26 added
findings, none removed. Additions: two partial reconstructions (compound test and
internal DELETE adapter), three unsupported Raw facade reconstructions, seven
unsupported Row fixture calls, one unsupported dynamic DB fixture, nine high Raw
fixture findings, and four destructive fixture statements (Row refusal and test
DDL/cleanup). Final precision: 227 precise, 140 partial, 80 unsupported; suppressed
0. This is a failing review threshold, not a clean review or runtime proof.
Line shifts in existing files are not counted as new findings.

`go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json --format json`
returned exit 0. Supplied schema/policy fingerprints matched. **generated_code and
database checks were skipped because fingerprints are absent**. Aggregate fresh
is not evidence of live schema freshness, physical DB identity or authorization.
The supplied manifest/schema/policy files and dependency inventory are unchanged.
Production migration planning is not applicable; test fixture DDL is not a
production migration.

## Acceptance coverage and remaining boundaries

1. [Current inventory](planned-query-execution.md) maps actual public terminals,
   generic/scoped/Many/RETURNING, compound, hooks, Raw aliases and bypasses. Historical
   contracts remain explicitly historical; case evidence is updated without
   declaring family-level completeness or resolving static uncertainty.
2. Internal compound preparation is DB/Executor/Begin-free for managed statements;
   known refusals prevent preceding dispatch. Strict opaque entries refuse before
   user construction callbacks. TableName/WriteOpt outside those refusals remain
   ordinary application callbacks, not universally effect-free code.
3. Ordered private phase/range/statement records use the shared structural builder
   and seal. Dynamic compatibility slots are unresolved until evaluated; changed
   inputs are re-inspected. No public execute-artifact/split API was added.
4. Generic and compound constituents retain strict tenant/protected/projection
   checks, including DO NOTHING conflict coverage. Raw remains unknown/refused in
   Strict; facade Row no longer calls cancelled harmless SQL to transport errors.
5. Required local suites, both databases/custom driver, review and manifest ran as
   above. Benchmarks and arbitrary custom dialects were not tested. CI's workflow
   does not set TEST_DB_DSN: CI success alone is not local skip-zero evidence.
6. PR publication/actual final-head CI and Issue link update are separate actions,
   recorded in the Relay report. Issue66 remains OPEN; no automatic merge/closure.

Source compatibility breaks for explicit *sql.Row consumers and DB-as-Executor
are intentional and documented. Strict nested is primarily Parent/Children without
cleanup; no complete collection-replacement claim is made. Opaque recipes are
unsupported under Strict, not dynamically certified. Compatibility can partially
execute; rollback does not erase callback/external effects. Ordinary transactions,
standalone hook.Run and direct SQLDB/driver/Tx/Executor are not whole-recipe
interception. Application authentication, asserted schema/PlainTable/unique facts,
live freshness and physical Executor identity remain conditional/unverified.
No new retry, automatic rollback, cardinality, external permit, versioned artifact
or exactly-once guarantee is introduced. These limits remain open Issue-level
considerations; this report does not close the Issue or confirm user completion.
