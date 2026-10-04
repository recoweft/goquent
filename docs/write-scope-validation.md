# GQ-AI-02 PR2 validation record

Work `f6cae07e-3bad-458a-a3fc-3c4cf5ccfcd4`, revision 3, execution
`2bce995b-3069-4dd7-935a-00169beb0c10`.
[Issue #57](https://github.com/recoweft/goquent/issues/57), management ID GQ-AI-02,
planned PR2 of 2; branch `feature/gq-ai-02-write-cardinality`, base `main`.
The PR description records its actual URL, head and CI run after publication.
No previous PR58/main CI result is used as evidence for this change.

## Identity and environment checks actually performed

- Supplied work_id and revision agree with the fixed execution context.
- `pwd` is `/home/murai/github/goquent`; `/etc/os-release` identifies Ubuntu
  24.04.4; `uname -r` identifies Microsoft WSL2. `go version` is
  `go1.26.4 linux/amd64`.
- Read `AGENT.MD`, the database-review PR template and contracts v3; searched for
  applicable AGENTS.md files in repository/ancestor paths. No additional file
  was found. No applicable nested instructions or local handoff files were found.
- Working tree was clean on the specified PR2 branch. Remote fetch/push target
  is `git@github.com:recoweft/goquent.git`.
- Network-restricted attempts at GitHub API/SSH failed; these were not valid
  evidence of invalid credentials. Outside that restriction, `gh auth status`,
  repository permissions and `git push --dry-run` succeeded. Issue search returned
  open #57 and branch PR search returned no existing PR2.
- `git ls-remote` confirmed remote main equals local HEAD/base
  `857b914432bccc6da834ecbae88c5b0749f62443`. GitHub API confirmed PR58 merged to
  main at 2026-10-04T14:47:51Z, with that SHA; PR53/56 also confirmed merged.
- Docker Compose services were healthy. Actual `SELECT version()` in the new
  database tests reported MySQL **8.4.6** (`mysql:8`) and PostgreSQL **16.10**.
  These are the measured versions, not a claim to test every 8.x/16.x release.

## Specification consultation

Question `eca5a874-13a0-42d0-bf54-342ce9f2a6f6` requested the missing technical body
of already-adopted proposal A, not reapproval. After controller delivery the exact
answer was retrieved. It supplies the typed application-asserted integer-key
contract implemented in [write-scope.md](write-scope.md). No historical question
was polled. No additional scope, Strict API, schema reader or authorization system
was introduced.

## Verification

Go commands use `GOCACHE=/tmp/goquent-build-cache` because the default cache is
read-only inside the workspace sandbox. The initial default-cache package test
failed during setup; it did not exercise implementation behavior.

| Command | Result / limit |
| --- | --- |
| `go test ./orm/query ./orm/review ./tests/contracts -count=1` | Pass. Includes new semantic/ownership/correspondence/JSON review tests and updated contract registry. |
| `go test ./...` | Pass in the DB-accessible environment on final code. |
| `make test-integration` | Pass outside the network sandbox; runs `go test ./... -count=1` with explicit MySQL/PostgreSQL DSNs. All packages passed; connection failure cannot silently skip with these DSNs. |
| `go test ./tests -run TestWriteScopeDatabaseSemantics -count=1 -v` | Pass on both actual servers; no skips. Tests log measured versions and execute real UPDATE/DELETE in rolled-back transactions. |
| `go test -race ./orm/query ./orm/internal/valuecopy ./orm/internal/valueguard` | Pass; no race reported for these tests. Does not support concurrent caller mutation. |
| `go run ./cmd/goquent review --fail-on high ./...` | Exit 1. Existing example/test Raw SQL, destructive DDL, missing-WHERE and weak predicates remain; new DB fixture DDL/Raw operations also produce findings. Unknown bulk evidence is partial. No findings were suppressed to obtain a pass. |
| `go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json` | Exit 0, schema/policy match. Aggregate fresh=true does not establish database freshness: generated_code/database fingerprints are absent and explicitly skipped. |
| `git diff --check` | Pass. |

The PR1 65535/65536/65537 argument ownership and Build/BuildSnapshot/Plan regressions,
copy/output-budget, depth/cycle and PredicateRef compatibility tests are included
in the all-package run. Existing Tx, scanning, bool, custom-executor and generic
CRUD tests remain. No example JSON baseline was regenerated. No hook/CI workaround
or unrelated flaky Insert-map-order fix was included. A successful run does not
claim that historical unrelated instability is resolved.

## Acceptance scope

| Condition | Implemented and tested scope | Limits |
| --- | --- | --- |
| AC1 | Range/inequality/multi-IN/non-key or differing-key OR do not prove one row; same complete integer-key OR can. | Broad is possible/unbounded, not measured multiplicity. |
| AC2 | Partial composite key and cross-alias/self-JOIN cannot establish a bound. | JOINs are conservatively unknown. |
| AC3 | Single/composite signed 16/32/64-bit PK equality/singleton IN; nullable integer unique with all non-NULL equalities; actual boundaries in both DBs. | Other types/unsigned DB types unknown; supplied context is asserted, not live-verified. |
| AC4 | Private SQL/typed params/tree/target correspondence; mutation/manual/JSON/Valuer/budget negatives; unknown/broad evidence and retained diagnostic codes. | No universal execution-time revalidation or authorization. |
| AC5 | PR1 ownership/output regressions; unchanged execution parameter types/order and call timing; full compatibility suite. Migration and limitations documented. | Custom internals/concurrent mutation and other CRUD pipelines are not certified. |
| AC6 | Repository implementation/tests/docs and specified PR preparation. | Actual PR/head/CI results are recorded in PR/controller report. Static review has unresolved findings. No merge or Issue closure. |

The PostgreSQL deferred-unique negative test deliberately inserts duplicate values
inside a deferred transaction and observes two deleted rows while scope stays
unknown. It is separate from the common positive tests. A string UNIQUE containing
`'1'` and `'01'`, compared with numeric 1, matches two MySQL rows and one PostgreSQL
row in these drivers; both scope verdicts stay unknown. These results support the
integer-only boundary rather than extending it to implicit coercions.

Implementation, local verification, PR publication and merging are separate
states. Keep **Refs #57** while any required verification/review remains incomplete;
never close dependency Issue #52 or merge automatically. Strict, tenant context,
external authorization, live schema freshness, arbitrary custom values, all-CRUD
routing and execution-time version binding remain unimplemented here.
