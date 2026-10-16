# GQ-AI-10 PR1 validation

Refs [Issue #86](https://github.com/recoweft/goquent/issues/86). PR1 of 3 only.
The accepted finite contract and exclusions are in
[command identity](command-identity.md). This evidence does not accept all Issue
ACs, merge the PR or implement PR2/PR3.

## Identity and environment

On 2026-10-16 this worker checked `/home/murai/github/goquent`, Ubuntu 24.04.5
under WSL2, clean initial worktree, and SSH origin
`git@github.com:recoweft/goquent.git`. Initial branch was
`feature/gq-ai-09-review-ci`, HEAD `5ff448b08b31cc69c6a9e31ba34f2464d4cfb25d`;
its tree equalled fetched main `db3b768cdc708dfaf47e45608d08a6c15fe3d83c`.
The specified `feature/gq-ai-10-command-identity` branch was created from that main.

Worker `gh auth status`, repository permission API (push/admin/maintain), SSH
`ls-remote` and `git push --dry-run` succeeded independently of controller Issue
credentials. Initial sandbox DNS/gh checks failed; network-enabled retries
succeeded, so the initial apparent invalid-token output was not treated as an
actual credential failure. Git index and Go cache writes required sandbox
escalation; these were allowed. No approval review rejection occurred.

Issue #86 was read with its body and comments (none at start); it matched the
request. All-state branch PR search and local/remote branch checks found no
existing target. Broad Issue/PR searches found prior Issues/PRs referring to the
plan, not another implementation of this planned PR. Work-search capability was
not available to this worker; the controller's reported zero-work search is not
claimed as independently repeated.

GitHub reported required PRs 67/68/69/71/72/74/75/77/78 and PR85 merged into main.
All nine prerequisite merge commits passed `git merge-base --is-ancestor` against
fetched main; PR85's merge is main itself. Main's `test` and `docs` check runs were
independently observed successful. Branch protection returned 404 (not protected),
and the main branch rules endpoint returned an empty array. Actual PR head checks
are recorded in the publication report separately from these main checks.

`AGENT.MD`, applicable file searches, PR template and current contract/API docs
were read. The specification consultation was answered under question
`aa494f5f-18b5-43fd-b729-97ef6f76468b` in this execution before implementation.
There are no new dependencies or changes to existing recipe/planner/typed-input
behavior. Runtime calls never install the ledger schema.

## Measured local commands

Toolchain: `go version go1.26.4 linux/amd64`.
Existing dedicated containers: MySQL **8.4.6**, PostgreSQL **16.10** (Debian
16.10-1.pgdg13+1). Version queries and healthy container status were observed;
containers were not reprovisioned. Tests created their own fixture tables and
refused to replace an already-existing ledger.

Both full runs and the final command run explicitly set all three environment
variables: `TEST_DB_DSN`, `TEST_MYSQL_DSN` (dedicated MySQL testdb), and
`TEST_POSTGRES_DSN` (dedicated PostgreSQL testdb). MySQL/PostgreSQL command subtests
run sequentially; full, race and final command DB runs were not overlapped.
`-json`/`-race`/`-count` were arguments to the outer `go test`, not GOFLAGS passed to
packages.Load. Raw JSON/diagnostic logs were kept outside the repository.

| Command | Measured result |
| --- | --- |
| `go test ./... -count=1 -json` | Exit 0; 2906 test/subtest pass events, 0 fail, 0 skip; 22 passing packages, 27 no-test packages |
| `go test -race ./... -count=1 -json` | Exit 0; 2906 test/subtest pass events, 0 fail, 0 skip; same package counts |
| `go test -race ./orm -run '^TestCommand' -count=1 -json` | Final focused run after adding canonical-v1 boundary and forced-reservation coverage: exit 0, 48 pass, 0 fail/skip, including both real DBs |
| `go vet ./...` | Exit 0 |
| `go run ./cmd/goquent manifest check-repositories --config tests/typedfixture/repositories.json` | Exit 0; actual supplied-source regeneration comparison, not live DB freshness |
| `sh scripts/test-public-output-check.sh` | Exit 0, CI_WRAPPER_CHECK_PASSED; success/failure canaries withheld |
| `go run ./cmd/goquent review --fail-on high ./...` | Exit 1; retained findings, not a passing safety proof |
| `go run ./cmd/goquent migrate plan docs/sql/command-store-mysql.sql` and PostgreSQL counterpart | Each exit 0, one add-table step, low/precise/0 warnings; structural review only, not deployment authorization |
| Example manifest verify with checked-in manifest/schema/policies and JSON output | Exit 1: schema stale, policy OK, generated_code/database skipped |
| Generate a new private manifest with the same schema/policy and `--code tests/typedfixture`, then verify it | Both exit 0; schema/policy/generated_code OK; database skipped |
| `git diff --cached --check` | Exit 0 |

Counts are test events including subtests, not invented independent assertions.
The last focused run covers the additional canonical-format/bounds test and four
real-DB forced-reservation subtests added after the full suites; final-head CI
also executes all tests. CI uses its own
fixed-output wrapper and does not expose these local per-test counts.

## Failure history

* Initial Go test failed because the sandbox could not write Go's cache; the
  network/cache-enabled rerun passed the existing recipe regressions.
* First new test build failed: test-only `WriteKeyColumn`/`WriteKeyConstraint`
  names needed the public `query` package qualification. Corrected before runs.
* First real MySQL run failed its existing Strict gate because the fixture
  declared lowercase `bigint`; PostgreSQL passed. MySQL declarations were
  corrected to the existing uppercase `BIGINT` contract. No gate was relaxed.
  An unnecessary test constraint precondition also failed after those earlier
  MySQL failures and was removed; the actual save-failure constraint remains.
* CLI `migrate plan --help` returns input error/exit 2 in this existing CLI. The
  actual two plan commands above succeeded. An exploratory `manifest generate
  --help` invocation produced only a summary (no artifact); actual generation
  used the supported `manifest` command and explicit private output path.

These failed attempts are not included in the passing evidence.

## Review and manifest limits

A new complete source report on this implementation tree measured **1239
findings, 0 suppressed, 0 precise, 1106 partial, 133 unsupported**. Prior reported
1197/1088/109 counts were not reused. The source report count was obtained through
`review.Run` and only aggregate counts were emitted; a truncated public view was
not mistaken for the complete report. No thresholds or suppressions changed.

The actual fetched-main tree was materialized with `git archive`, and the staged
patch was passed to `review --format review-ci-json --diff ... --diff-base-root
... --fail-on high .`. Exit 1: full-report risk and input gates failed,
`compared=true`, all **1239 head findings unknown**, diff 1 modified/9 added,
0 invalid/0 unlisted and 1 unknown. The known generated-loader/testmain comparison
limitation remains unresolved. The public output does not expose its detailed
cause; this run establishes unknown comparison, not verified correspondence or
an independently attributed root cause. No “no regression” claim is made from it.

The checked-in example schema fingerprint is still stale. Fresh generation of a
separate local artifact demonstrates supplied-input agreement only; database
verification remains skipped. Existing manifests were not rewritten to hide this.

## Acceptance mapping and limits

| PR1 AC | Evidence and boundary |
| --- | --- |
| AC1 | Explicit bounded structural API; full typed snapshot/canonical v1, type/lexeme/null/missing/map/step-order and mutation tests; opaque methods never evaluated; trusted private tenant and fixed source summary |
| AC2 | Both DBs: same-input replay, different input/kind conflicts, tenant and namespace isolation; current-policy replay refusal. Both DBs also force both calls through the actual reservation INSERT path (same and different inputs), so a real unique violation and fresh lookup cannot be avoided by a lucky initial lookup. Unit fault tests cover corrupt/unknown/missing rereads |
| AC3 | Both DBs: simultaneous first calls and competing different inputs; business unique failure rolls back earlier step; constrained result-save failure leaves no business row and retry succeeds; pending-reservation cancellation and rollback/retry; fresh connection/store replays persisted results |
| AC4 | Owned handle check; actual external transaction and custom-executor wrappers refused; existing Strict opaque recipe refusal; normal Strict planner checks reused; schema drift/secondary uniqueness/missing ledger rejected; explicit DDL/English migration and source/public boundaries |
| AC5 | Local results above; actual final-head CI is separate publication evidence. Review high gate and existing manifest/comparison limits remain unsuccessful/unverified rather than silently green |
| AC6 | Specified branch/base/title, real Issue URL/Refs #86, PR1 scope and PR2/3 remainder recorded in PR; real PR URL and Issue link update recorded after creation |

Mock-only tests cover commit-return error, rollback error, reservation/save row
count violations and driver RowsAffected failure. They do **not** prove actual
network loss during COMMIT on either engine. Real tests cover commit-before
failure and process-restart/response-loss equivalents through rollback/new
pool/store; they do not kill a DB server or application process. No real network
fault injection, power-loss durability test or third-party executor proof was run.
No throughput/latency benchmark was run; metadata is rechecked on every call.
Source review was performed by this worker; no independent human review is claimed.

PR2 optimistic-version/common expected-row rollback and general UPSERT semantics,
PR3 hook/audit/outbox integration and whole-Issue acceptance remain outstanding.
No automatic merge, Issue close, user acceptance, production DDL application or
external exactly-once delivery guarantee is performed by this PR.
