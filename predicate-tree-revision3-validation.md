# GQ-AI-02/PR1 revision 3 validation

Work `43118939-d7f2-420a-863f-873803cbeda3`, revision **3**, execution
`1253d645-c85f-49c5-97e4-e35133d1b201`.
Refs [Issue #57](https://github.com/recoweft/goquent/issues/57), existing
[PR #58](https://github.com/recoweft/goquent/pull/58), branch
`feature/gq-ai-02-predicate-tree`, base `main`.

## Identity and prerequisites actually checked

The supplied fixed context and revision-3 request agree. Historical revision-2
wording and consultation IDs are history, not a different execution target.
`pwd`, `uname -a`, `/etc/os-release`, Git remote/status/HEAD/worktree listing,
GitHub Issue/PR APIs and remote refs were checked. Cwd is
`/home/murai/github/goquent`, WSL2 Ubuntu 24.04.4; origin is
`git@github.com:recoweft/goquent.git`. Initial local/remote/PR head was
`fa773b1707f1d94ed0464f03cbcebd3e7e38f9fb`, clean, PR OPEN/unmerged, with the
specified branch/base/title. Issue #57 was OPEN. Latest remote main remained
`0d08f5233e55015aac6e5b2eb626e8d92c6c3202`; no new branch or reset was used.
Only this worktree was listed and no index lock was present. Other controller or
remote-agent executions beyond these visible facts were not independently checked.

Read AGENT.MD, searched ancestor and repository AGENTS.md locations (none
applicable), read the PR template, API inventory/contracts and original predicate
contract. The global Codex AGENTS.md was empty. Authenticated GitHub account
`recoweft` has repo scope, ADMIN and API push permission; branch push dry-run
succeeded before implementation. Actual new-PR creation was deliberately not
exercised. Initial sandbox network/cache/Docker denials were environmental;
authorized network/DB access and a writable `/tmp` Go cache resolved them.

## Correction and evidence

Source analysis confirms that the old depth-only recursive copy expanded every
shared edge up to depth 64. No old implementation was deliberately run to OOM or
stack overflow. The adopted revision-3 output option A is documented in
[predicate-tree.md](predicate-tree.md#revision-3-bounded-copy-and-output).

| Acceptance | Regression evidence |
| --- | --- |
| Branching map/slice self-cycles, mutual cycles, mixed map/slice | `TestBranchingCyclesHaveBoundedCopies`: 1–2 memo entries, 3–6 charged slots, measured 3–6 allocations; type/reference retained and unverified |
| Acyclic sharing is not a cycle | `TestSharedDAGMemoAndIndependentCopies`: 30 binary levels, 31 memo entries, 91 slots, measured 104 allocations; detached, sharing preserved within a copy and absent across independent copies |
| Depth/width/total allocation limits before allocation | `TestCopyBudgetsAndDepth`, `TestWideMapAndDeeperMemoOccurrence`: depth 64/65, slots 65,536 boundary, map/slice width, byte budget 8 MiB boundary, aggregate arguments, deeper memo occurrence, original reference on refusal |
| Node traversal and propagation | `TestNodeGraphMemo`, aggregate Node-value budget test: shared Node DAG memoized, cyclic views terminate unverified, failed isolation propagates to ancestors |
| Public builder/copy/Snapshot/BuildSnapshot/Plan paths | `TestRecursivePayloadsAcrossBuilderAndPlan`, `TestDAGOutputAndIndependentSnapshots`: SELECT copies and UPDATE/DELETE plans, SQL/parameter type/order, ancestor state and independent snapshot mutation |
| Output expansion has a separate budget | valueguard tests: depth 256/257, 65,536-node and estimated-byte boundaries, map width, aggregate repeated payload, 40-level DAG rejected before output with measured 48 allocations |
| String/JSON behavior and all payload locations | Plan regression exercises String marker, ToJSON, Marshal(pointer/value), MarshalIndent, nil Plan, Metadata, both warning/Evidence lists, WHERE/HAVING/named values and standalone condition JSON |
| Normal output and custom evaluation | Safe DAG JSON equals legacy standard encoding; ordinary String matches previous parameter formatting. Custom Marshaler/Formatter/Stringer call counts and custom JSON error identity tested; preflight makes no user calls. Existing opaque Valuer/custom Executor/error-timing tests still pass. |
| Existing consumers and fixtures | Query/builder, contracts, review, manifest, operation, MCP and CLI tests; all five example JSON files byte-for-byte unchanged from the reviewed head |

Allocation counts are observations on Go 1.26.4, not cross-version performance
promises. Tests assert graph/budget invariants and loose allocation ceilings,
not wall-clock thresholds. A development test exposed a memo source-lifetime bug;
the final memo retains sources to prevent GC/address reuse across arguments.
An initial new DAG test incorrectly assumed the DSL did not flatten slice arguments;
the fixture was corrected to use a map payload. Neither failure was ignored.

## Commands and results

Go `go1.26.4 linux/amd64`; cache `GOCACHE=/tmp/goquent-gq-ai02-cache`.
Repository Compose MySQL **8.4.6** and PostgreSQL **16.10** were queried directly
for their versions. DB commands ran serially. Explicit local DSNs used the
repository defaults (MySQL root/password, localhost:3306/testdb, parseTime=true;
PostgreSQL postgres/password, localhost:5432/testdb, sslmode=disable).

| Command | Measured result |
| --- | --- |
| `go test ./orm/query ./orm/internal/querybuilder/... ./orm/internal/valuecopy ./orm/internal/valueguard ./tests/contracts ./orm/review ./orm/manifest ./orm/operation ./orm/mcp ./cmd/goquent -count=1` | PASS; final affected-package rerun also PASS after the last Node-memo cleanup |
| `go test -v ./orm/internal/valuecopy ./orm/internal/valueguard -count=1` | PASS; resource observations above; subsequent map/deeper-reference boundary additions also PASS |
| Explicit both-DB DSNs: `go test ./tests -run '^TestPredicateTreeDatabaseSemantics$' -count=1 -v` | PASS, both databases actually ran; NULL/IN/empty-IN rejection, aliases, composite predicates and nested writes through external sql.Tx |
| Final source: `go test ./... -count=1 -json` | PASS: 879 pass / 0 fail / 1 skip test/subtest events. Only registered driver skipped without TEST_DB_DSN. |
| Final source: `TEST_DB_DSN=... GOFLAGS=-json make test-integration` | PASS: 880 pass / 0 fail / 0 skip test/subtest events. Both DBs and registered driver actually ran. |
| `go run ./cmd/goquent review --fail-on high ./...` | Exit 1: 232 Medium / 44 High / 9 Destructive / 11 Blocked; 225 precise / 37 partial / 34 unsupported. Existing findings plus partial reconstruction of new test helpers; no suppression or safety claim. |
| `go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json` | Exit 0; schema/policy match and aggregate fresh=true. generated_code/database fingerprints absent: skipped/unverified. |
| `git diff --check`; diff of five example JSON files against reviewed head | PASS; no JSON fixture changes or regeneration |

## Existing failures and limits retained

An earlier full run during this revision failed
`TestRunTransactionWithHooksRollsBackOnHookError`: actual INSERT columns `name,id`
versus sqlmock's expected `id,name` (878 pass / 1 fail / 1 skip). Subsequent final
source success does **not** resolve that intermittent defect. `orm/write.go`,
`orm/transaction_hooks.go` and its tests were independently diffed against main
and remain identical. The original validation record contains earlier 20-trial
main/PR evidence; those old samples were not rerun or represented as new results.
No unrelated hook repair, skip or expectation relaxation is included.

Original head CI run 37114901605 was success; it is **not** CI evidence for this
revision. The updated PR and RelayWeft report record the actual new commit and
its CI URL/result. CI does not set TEST_DB_DSN; registered-driver coverage comes
from the local explicit-DSN integration run.

Resource limits cover the documented built-in payload and library-owned output
paths. Custom serialization/formatting internals, caller-side direct formatting,
SQL interpolation, arbitrary concurrent mutation, whole-process heap limits and
public artifact validation are unverified/out of scope. SQL/clause storage is
not a global statement quota. No live-manifest fingerprints were obtained.
PR2 cardinality/key/alias/nullability proofs, Strict authorization and other
issues remain unimplemented; Issue #57 and PR #58 remain open and unmerged.
