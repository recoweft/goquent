# GQ-AI-02/PR1 revision 4 validation

Work `43118939-d7f2-420a-863f-873803cbeda3`, revision **4**, execution
`949ee320-43e6-4db4-bfc2-01571f3cd7db`. Refs
[Issue #57](https://github.com/recoweft/goquent/issues/57), existing
[PR #58](https://github.com/recoweft/goquent/pull/58), branch
`feature/gq-ai-02-predicate-tree`, base `main`. This addresses High finding
`64b625b9-6719-4241-876a-033da500ef36` from saved review
`17fd60a3-8e36-4c95-ae9f-311132f518c6`. Revision 3 report
`542d24e4-0620-48b0-8ae9-bd70e0e3363a` and CI 37194643257 are historical evidence.

## Checks actually performed

The fixed work/revision/execution context matches this request. `pwd`, `uname -a`,
`/etc/os-release`, Git remote/status/HEAD, `git ls-remote`, GitHub Issue/PR APIs,
`gh auth status`, repository permissions, branch push dry-run, worktree listing,
index-lock absence, ancestor/repository instruction searches and a visible
process listing were checked. Cwd is `/home/murai/github/goquent`, WSL2 Ubuntu
24.04.4; origin is `git@github.com:recoweft/goquent.git`. The initial tree was
clean; local/remote/PR head all matched `c0df370263a85ff456ba2910b34913dce0f6fca8`.
Remote main matched `0d08f5233e55015aac6e5b2eb626e8d92c6c3202`. The PR was OPEN,
with the specified branch/base/title; Issue 57 was OPEN. GitHub account recoweft
has repo scope and admin/push permissions; SSH push dry-run succeeded. New-PR
creation was not exercised because this request reuses PR58.

Read AGENT.MD, the API inventory/contracts, PR template, current predicate design,
revision-3 validation and current public Build/Snapshot/Plan/Executor paths.
No additional applicable ancestor/repository AGENTS.md was found. Only this
worktree was listed, no index lock was present and no competing execution was
visible in the process listing. Controller-wide/remote execution exclusivity
is unverified; these local observations do not prove it.

## Fix and acceptance evidence

The shared `Copier.Slice` now always owns its outer array. Payload copying retains
operation-scoped budgets/memo, including active/completed distinction and source
lifetimes. The mandatory linear outer allocation is separate from payload depth
64, aggregate slots 65,536 and bytes 8 MiB; each argument is a payload root. See
[ownership design and audited cleanup paths](predicate-tree.md#revision-4-argument-container-ownership).
No SQL/NULL/type normalization, user-code evaluation, output-budget change or
cardinality proof was added.

| Acceptance | New evidence |
| --- | --- |
| Reproduce actual failure safely | With only value.go restored temporarily to reviewed c0df370, the final boundary test passes 65,535 and fails 65,536/65,537 in both dialects: returned argument 0 is nil instead of int64(0). No DB/OOM test involved. Fixed source was restored immediately. |
| 65,535/65,536/65,537 public Build, BuildSnapshot and SELECT Plan | `TestArgumentOwnershipBoundaries`: every type/value/order including nil and typed-nil bytes, complete placeholder sequence and every rendered condition position. Pure Snapshot values are checked separately because it intentionally has no rendered positions. |
| Rebuild, other builder, clone, independent snapshots/Plans | Same regression checks retained arrays after repeated/other builds, CopyStateToSelect and NewRawPlan; mutates all returned args/Plan elements and all first snapshot values, then checks independent copies and a fresh build. |
| Deterministic cleanup rather than pool scheduling | `TestSliceOwnsContainerAfterBudgetExhaustion`: exhausts the copier first, explicitly zeros every source element, then checks all retained elements and independent outer arrays even with the same Copier. Public Build tests also exercise its unconditional deferred cleanup. |
| Opaque child and total budget | `TestSlicePayloadBudgetAndMemo`, `TestArgumentOwnershipOpaqueAndAggregatePayload`: wide opaque slice, two distinct >half-budget byte payloads, shared memo child, pointer, nil/typed nil, original opaque references and ancestor/value unverified reasons. |
| Actual execution handoff without DB | `TestLargeArgumentsReachCustomExecutor`: 65,537 mixed arguments in both dialects reach recording Query unchanged; exact sentinel error and one-call timing preserved; subsequent Build cannot destroy captured args. |
| Display does not damage execution | Boundary regression checks String marker and typed budget errors for ToJSON, Marshal(pointer/value), MarshalIndent, then rechecks all Params and SQL. |
| Prior A contracts | Related suites include branching/mutual/mixed cycles, shared DAG/source lifetime, depth/slot/byte boundaries, Node propagation, snapshot isolation, Valuer/Executor and custom formatter/marshaler timing; all pass. |
| Consumers/fixtures | Existing contracts, PredicateRef/Plan JSON, review, manifest, operation, MCP and CLI tests pass. Five existing example JSON files are unchanged; no regeneration. |

An initial new test wrongly required rendered positions from pure Snapshot; it
was corrected to honor the existing API. The final old-source reproduction above
uses the corrected test. The final regression later tightened rendered-position
assertions and was rerun successfully; production code did not change during
validation.

## Measured commands and results

Go **1.26.4 linux/amd64**, `GOCACHE=/tmp/goquent-gq-ai02-cache`. Repository Compose
services: MySQL **8.4.6**, PostgreSQL **16.10**, verified by server version queries.
Local loopback DSNs selected testdb explicitly; credentials are omitted here.
Database suites ran serially. Large-argument tests never send SQL to a database.
Counts below are test/subtest events, not package totals.

| Command | Result |
| --- | --- |
| `go test ./orm/query ./orm/internal/querybuilder/... ./orm/internal/valuecopy ./orm/internal/valueguard ./tests/contracts ./orm/review ./orm/manifest ./orm/operation ./orm/mcp ./cmd/goquent -count=1` | PASS |
| Final targeted `go test ./orm/query ./orm/internal/valuecopy -run 'TestArgumentOwnership\|TestSlice\|TestLargeArguments' -count=1` | PASS |
| Initial sandbox `go test ./... -count=1 -json` | Interrupted (exit 130) after DB connection-denial skips; partial 739 pass / 1 fail / 9 skip. Not a full-run success. Hook failure described below. |
| Network-enabled `go test ./... -count=1 -json` | Exit 1: **888 pass / 2 fail / 1 skip**. Both failures are the existing hook column-order defect; registered driver skipped without TEST_DB_DSN. |
| Explicit registered-driver DSN plus `GOFLAGS=-json make test-integration` | Exit 0: **891 pass / 0 fail / 0 skip**. Both DBs and registered driver actually ran. |
| Explicit MySQL/PostgreSQL DSNs: `go test ./tests -run '^TestPredicateTreeDatabaseSemantics$' -count=1 -v` | PASS, both DB subtests exercised NULL/IN, aliases, composite predicates and external sql.Tx semantics. |
| `go run ./cmd/goquent review --fail-on high ./...` | Exit 1: 236 Medium / 44 High / 9 Destructive / 11 Blocked; 225 precise / 41 partial / 34 unsupported. Existing findings and new test helpers remain unsuppressed; not a safety proof. |
| `go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json` | Exit 0, schema/policy match. generated_code/database fingerprints absent and skipped/unverified despite fresh=true. |
| `git diff --check`; example JSON diff against c0df370 | PASS, all five existing JSON files unchanged. |

Full-run failures: `TestRunTransactionWithHooksCommitsAuditHook` and
`TestRunTransactionWithHooksRollsBackOnHookError`, actual INSERT columns `name,id`
versus expected `id,name`. The initial interrupted run also failed the first.
`orm/write.go`, `orm/transaction_hooks.go` and its tests are unchanged from main.
Successful integration does not fix or erase this intermittent existing failure;
no unrelated hook changes or test relaxations were made.

## CI and remaining limits

New-head CI and exact pushed commit are recorded in the PR and RelayWeft report;
revision-3 CI is not evidence for this correction. CI omits TEST_DB_DSN, so the
registered-driver claim comes from explicit local integration.

Payload reference retention beyond the copy limits is intentional and unverified;
arbitrary custom internals, concurrent caller mutation, complete heap/final JSON
size bounds and public Plan tamper detection remain outside the contract.
Output limits still reject large Params. No live manifest fingerprints obtained.
PR2 key/alias/composite/nullable-unique/OR-cardinality and diagnostic improvements
remain separate. ChatGPT's remaining final review is not claimed completed.
Implementation/testing, PR update, report receipt and merge are distinct states:
this change does not merge PR58, close Issue57 or confirm user completion.
