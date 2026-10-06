# GQ-AI-05 PR1 validation

[Issue #70](https://github.com/recoweft/goquent/issues/70), PR1 of 2.
Work `4dfaaad6-882b-459c-ba62-0ed103d4a477`, revision **3**, execution
`0eadf998-6c06-4eb7-a6ac-ec6cdb97e4d5`.
Tested implementation source: `b71173fbfdd68e700aa7ba087867da8766818d23`.
The subsequent validation-document commit changes documentation only. Actual PR
URL/final head/CI are recorded in the RelayWeft report after publication.
Branch `feature/gq-ai-05-plan-contract` → `main`; no merge or Issue closure.

## Checks actually performed

The work/revision in the supplied request match the fixed execution context;
execution identity was taken from that context, not independently queried from
a controller registry. `pwd` matched `/home/murai/github/goquent`; uname showed
WSL2 `6.18.40.1-microsoft-standard-WSL2`, os-release Ubuntu **24.04.4 LTS**.
Origin fetch/push both resolve to `git@github.com:recoweft/goquent.git`.
The initial branch was already the requested branch, with the prior revision's
uncommitted PR1 files. Those were inspected and reused. No unrelated changes were
found or removed; no alternate task/branch was selected.

Sandbox-only gh auth initially failed; the network-enabled repeat verified the
active worker `recoweft`, repo scope, repository id **1004301492**, push/admin
permissions and successful `git push --dry-run origin HEAD:feature/gq-ai-05-plan-contract`.
These are worker credentials, not the separate controller Issue-connection PAT.
Actual creation/push are separately recorded after they occur.

Fetched origin/main and starting HEAD both equaled
`cafbd255404520c328921cd6aab3c1b8675c1091`. GitHub PR69 was MERGED with that SHA.
Main CI **37455754844** and Docs **37455754932** were success. Issue70 was OPEN;
all-state specified-head PR search returned no PR. Other dependency PR merges
were inherited facts, not separately revalidated beyond this main/PR69 check.
No worker tool exposed Relay work search or ChatGPT project/generation/permission
verification, so those were not independently checked. No start_work, alternate
work lookup, delegation, historical get_answer or other MCP integration was used.

Read root AGENT.MD, contracts/API inventory, PR template, planned execution and
tenant-policy docs; checked ancestor/repository AGENTS.md availability (none found).
Inventoried QueryPlan custom writer, ReviewReport formatter, runtime TablePolicy /
PolicySet, manifest Policy/Version/fingerprint, OperationSpec reader/compiler,
review plan-file readers, CLI and MCP consumers. Low-level internal packages add
no root/CLI dependency or new module dependency.

## Revision decisions implemented

The published revision adopts the two historical recommendations; their original
pending labels were not treated as present blockers. No new specification question
was needed. Implemented internal-only key/scope/generation primitives and actual
private planner snapshot provenance, with no public key/context/ID/execution API.
Implemented the four version readers/writers, Go input checks and UseNumber dynamic
type migration. QueryPlan method-level invalidation is explicitly distinguished
from standard-library pre-method syntax rejection (old receiver unchanged), and
from alias/pointer-null behavior. External readers use fresh receivers; single
JSON-document readers reject trailing input. See the [contract](plan-version-identity.md).

## Final runtime validation

Environment: **go1.26.4 linux/amd64**, MySQL **8.4.6**, PostgreSQL **16.10**.
Reused the repository's healthy local Docker Compose containers. The suites ran
**sequentially**, with `GOCACHE=/tmp/goquent-gq05-cache` and all three explicit
repository test DSNs: `TEST_MYSQL_DSN`, `TEST_POSTGRES_DSN`, and `TEST_DB_DSN` (the
registered custom-driver suite uses the local MySQL test database). Credentials
are the existing fictional local Compose defaults, not application credentials.

| Command at tested source | Result |
| --- | --- |
| `go test ./... -count=1 -json` | exit 0; 1,591 test/subtest pass, 0 fail, 0 test skips |
| `make test-integration GOFLAGS=-json` | exit 0; 1,591 pass, 0 fail, 0 test skips |
| `go test -race ./... -count=1 -json` | exit 0; 1,591 pass, 0 fail, 0 test skips |
| `git diff --check` | pass |

Each suite has **19** passing test packages and **23** packages with no test files;
package skip events are not skipped tests. The supplied prior baseline was 1,431
passes/18 test packages, not independently rerun here. The increase is 160
cases/subcases and one tested internal package; it is not a claim of 160 distinct
safety guarantees. Earlier focused runs found an old float64 assertion and an
ambiguous JSON-vs-native fixture digest. They were corrected to the adopted
json.Number contract; the digest was independently recalculated with Python's
standard-library HMAC/SHA-256 as well as Go. A temporary test import error was
fixed before final suites. No failing test was skipped or weakened to pass.

Coverage includes:

- Four envelope version tables: missing/0/1, unknown/negative/null/type errors,
  exponent/fraction, overflow, duplicate and case collision; Go writer/input
  rejection; legacy roundtrip; nil pointer output; numeric metadata/params.
- Actual private planner SQL/args/owner snapshots on both dialects, map insertion
  order, native widths/values, target/tenant/policy/schema/config changes,
  projection/condition/order/limit, INSERT map/value changes, public SQL/Params /
  risk/blocked/approval/source/table/warning edits, and zero planning dispatch.
- Method-reached successful replacement and failed atomic public preservation /
  private invalidation, all six Executor signatures blocked without dispatch;
  syntax-precheck old receiver unchanged; Decoder and pointer-null/alias limits.
- Language-neutral canonical/HMAC vectors and relationships, integer precision,
  list/map ordering, null/empty distinctions, key/scope/generation changes and
  missing inputs; detached bytes; unsupported custom types/cycles/budgets; separate
  expiry refusal despite stable identity. Raw/opaque/missing context and invalid
  private key assertions remain identity-unavailable.
- OperationSpec IN ordering/large numbers, CLI values and MCP precision/trailing
  documents/unknown versions, and explicit review rejection of version-invalid
  plan envelopes. Existing private lifecycle, Strict/compatibility, six-method
  spies, CRUD/Row/compound, both DBs, external Tx/custom driver/scanning/bool suites
  ran unchanged except the authorized JSON dynamic-type expectation.

## Review and manifest

`go run ./cmd/goquent review --fail-on high --format json ./...` returns **exit 1**.
The same CLI reviewed an archive of origin/main cafbd255 at
`/tmp/gq05-main-review`, also exit 1. Compare without source line numbers:

| Source | Blocked | Destructive | High | Medium | Total |
| --- | ---: | ---: | ---: | ---: | ---: |
| main cafbd255 | 11 | 16 | 65 | 357 | 449 |
| PR1 b71173f | 11 | 16 | 65 | 365 | 457 |

Eight additions are `STATIC_REVIEW_PARTIAL` in `orm/query/identity_test.go`;
none removed, thresholds unchanged, suppression count 0. Final precision counts:
229 precise, 148 partial, 80 unsupported. Partial fixture reconstruction remains
unproven; CI success does not certify it or make the review threshold pass.

`go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json --format json`
returns exit 0. Existing schema/policy fingerprints match unchanged examples.
`generated_code` and `database` are **skipped: fingerprint not present**. Aggregate
fresh=true is only supplied-file correspondence, not live DB/Executor proof.
Production migration planning is not applicable: no schema migration was added.

## AC evidence and limits

1. Four version contracts, public error constants and actual reader/writer inventory
   are documented/tested. PolicySet remains private; manifest string version 1 and
   fingerprint layout remain unchanged. JSON migration confers no authority.
2. Stable internal canonical/typed domain and separate shape/execution contracts
   have fixed vectors plus real planner tests. Meaningful ordering and exact
   integer/native type distinctions remain; no public ID retrieval exists.
3. Private source-bound snapshots include SQL, dialect, target, ordered args,
   typed tenant, policy/schema/config. Unavailable is explicit. Current-input
   runtime verification and public supply are PR2, not claimed here.
4. Only internal value-free shape SHA and keyed execution/context digests exist.
   No new ID/key/secret metadata output; fixture key is test-only. Key persistence,
   scope/generation rotation and collision refusal boundaries are documented;
   production key management and complete public output redaction are absent.
5. Final required suites/review/manifest results above distinguish pass, fail,
   test skips, no-test packages and missing evidence. Benchmarks and arbitrary
   custom dialects remain unverified. CI does not set TEST_DB_DSN; its green result
   must not be substituted for the local explicit custom-driver run.
6. Actual PR publication, Issue link update and final-head CI are separate report
   fields. Issue70 retains two PRs and unchecked overall ACs. No automatic merge,
   closure, user completion proxy, PR2, GQ-AI-06, Rust/Quent/FFI/server work.

Fingerprint labels do not prove physical Executor identity, live schema freshness,
application authentication, honest executors or truth of asserted schema facts.
Existing Strict opaque-recipe refusals and compatibility partial/external effects
remain. Copies of a plan are not globally revoked by receiver decode. No serialized
JSON execution, signature permit, external attestation, exactly-once or general
transaction interception is introduced. Report receipt, CI, merge and user
completion remain separate states.
