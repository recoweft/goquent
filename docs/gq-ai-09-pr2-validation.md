# GQ-AI-09 PR2 validation

Management ID: GQ-AI-09. Planned PR2 of 3, [Issue #82](https://github.com/recoweft/goquent/issues/82).
Branch: `feature/gq-ai-09-review-precision`; base: `main`.
This records implementation evidence, not whole-Issue acceptance or permission to merge.

The following original sections record **revision 1**, source
`257bf1d6b34084d7b2ab4308c5787a8e182e5125`. They are historical evidence, not
measurements for revision 2. See the revision-2 record appended below.

## Identity and prerequisites

The worker checked `/home/murai/github/goquent`, Ubuntu 24.04.5 on WSL2, a clean
initial working tree, and origin `git@github.com:recoweft/goquent.git`. It read
AGENT.MD, searched applicable AGENTS.md locations, and read the PR template and
current contracts. Worker SSH and GitHub API authentication identified `recoweft`;
repository permissions included push/admin. These checks were separate from the
Issue connector. Initial sandbox DNS/cache/.git restrictions were resolved with
authorized tool escalation; the DNS-related auth-status output was not treated
as proof of invalid credentials.

Remote main and fetched origin/main were
`c0ce71f4ded7746cea1b616b67f11ce329f1e5f6`. PR83 was independently observed MERGED
with that merge commit, and the ancestor check passed. Postmerge CI run
[38604254503](https://github.com/recoweft/goquent/actions/runs/38604254503) and Docs
run 38604254652 were successful. Existing PR2 PR/remote-branch searches were empty.
The specified branch was created from this main. A pre-publication remote check
still returned the same main commit. Final head, actual PR URL and its own CI are
recorded in the PR/RelayWeft report rather than inferred from PR83.

Specification answer `19d44419-8484-446e-b8c9-6c6abb9743ac` selected the minimal
in-process application-test receipt path, real shared CLI-runner testing, and
opt-in v2 while retaining v1 defaults. The standalone CLI has no trusted-positive
artifact or producer input. See [the contract and migration](static-review-precision.md).

## Local measurements

Toolchain actually observed: `go version go1.26.4 linux/amd64`.

| Check | Result |
| --- | --- |
| `go test -json ./... -count=1` | 2,746 test pass events, 0 fail, 1 skip; 22 passing test packages and 27 packages with no test files |
| Skip | `tests.TestOpenWithRegisteredDriver`: `TEST_DB_DSN` was absent; not counted as a pass |
| `go test -race ./orm/query ./orm/review ./orm/mcp ./cmd/goquent` | Passed |
| Focused race reruns after boundary adjustments | Receipt/session, real CLI runner, v2/MCP fixtures passed; final source-consistency fixture rerun recorded separately in the report |
| `go vet ./...` | Passed |
| `sh scripts/test-public-output-check.sh` | Passed |
| `go run ./cmd/goquent manifest check-repositories --config tests/typedfixture/repositories.json` | Passed; actual regeneration comparison, not live evidence |
| Example manifest verification with manifest/schema/policies and `--format json` | Exit 1: schema stale, policy OK, generated_code/database skipped; unresolved |
| `go run ./cmd/goquent review --fail-on high ./...` | Exit 1; findings retained |
| Formatting/diff whitespace | gofmt and `git diff --check` passed |

The full/race commands were rerun as relevant source boundaries changed; earlier
successful runs are not substituted for final-source checks. Test counts are
local JSON-event counts (including subtests), not inferred CI counts. Dedicated
explicit-three-DSN sequential MySQL/PostgreSQL integration was not performed:
this change adds inspection only and changes no execution semantics. DB versions
were not independently collected. CI's private per-test counts and DB versions
remain unverified; aggregate CI success cannot fill these fields.

## Baseline and head review

An independent source-API counter ran on the clean latest-main tree before
implementation and again on the PR source. It reported only aggregate counts.

| Source | Findings | precise | partial | unsupported | suppressed | high gate |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| Latest main c0ce71f | 1,180 | 156 | 916 | 108 | 0 | fail |
| PR2 source | 1,196 | 0 | 1,087 | 109 | 0 | fail |

Existing codes retain their counts except STATIC_REVIEW_PARTIAL (+15) and
STATIC_REVIEW_UNSUPPORTED (+1). New receipt/planning code and fixtures add bounded
static discoveries; the raw-SQL review entry retains a dynamic-SQL unsupported
finding. The old 156 shape-precise findings move to partial because unknown axes
are preserved. No findings were removed through suppression or threshold changes.
The source coverage measurement retained 933 operation records, with no global
budget-failure diagnostic. These are discovered records, not proof that every
possible runtime operation was found. Historical PR1 reports (including the old
499-findings baseline and 2,721-test run) are not current-head evidence.

## Acceptance evidence and limits

- AC1: independent closed axes, all-unknown versus partial coverage, reasons and
  missing evidence, conservative private operation gate, explicit source detail
  version and migration. No axis is used by Strict/execution authorization.
- AC2: real common-planner SELECT receipts; exact session/source-input binding;
  per-component operation/target/generated SQL+conditions/ordered typed args/
  context comparisons. Private execution handles and public QueryPlan JSON are
  not receipt inputs. Source association remains a test declaration.
- AC3: existing typed/alias/helper/receiver/branch/loop/escape/generic/validated
  API/load-failure and external-JSON marker regression suites pass. Dynamic
  bindings, runtime policy, physical DB state and unsupported families stay
  unconfirmed even with a matching sample.
- AC4: v1 defaults/strict readers remain, opt-in v2 normalizes closed values and
  bounds lists/counts. API/CLI/MCP use the same classifications. Names, SQL,
  values, paths, digests, loader errors and private receipts are absent from
  public output. Source APIs remain sensitive and display is not a gate input.
- AC5: real planner API/CLI positives; native argument-type, SQL, target, dialect,
  context (including all-six-matched without filling static unknowns), missing or
  replaced owner, mutation, changed source, duplicate, old/foreign/zero receipt and
  altered-reference negatives; strict v1/v2/version tests; privacy canaries;
  Executor-zero spy; opaque Valuer/Stringer/Marshaler refusal; budget/cancel;
  inherited suppression/expiry and PR1 regressions; full/race/vet/regen checks.
- AC6: the specified PR title/branch/base and Issue linkage are used. Actual PR
  publication, Issue append and final CI are external results reported separately.
  No merge, Issue close, whole-Issue acceptance or user-completion action is taken.

PR3 still owns SARIF, changed-line review, expiring narrow baselines, retention of
new/worsened findings, and SARIF/JSON code/location agreement. No such feature is
implemented here. A successful local or CI check is not a runtime safety proof.

## Revision 2: source-report precision and independent review

Request: work `5052147d-9158-4a3f-8846-4d67187afab2`, revision 2;
execution `c12fe8e8-6b5b-4880-95c6-19a15650c0fc`.
Actual [PR84](https://github.com/recoweft/goquent/pull/84), same branch/base/title.
Starting local and remote head: `257bf1d6b34084d7b2ab4308c5787a8e182e5125`.
Final publication head and matching CI are recorded in the PR and structured report.

The worker independently checked cwd, Ubuntu/WSL, clean initial tree, origin,
branch/head, applicable ancestor/repository rules and PR template, current
contracts, worker GitHub authentication and repository push permission. Remote
main still equals `c0ce71f4ded7746cea1b616b67f11ce329f1e5f6`; ancestry passed.
PR83's MERGED state and its matching successful main CI 38604254503 were checked
again. These are separate from the old PR84 CI 38611294468, which is historical.
The supplied prior ChatGPT review/hash assertion is controller-provided context;
this execution did not retrieve/hash that external stored review object.

### Reproduction and correction

Minimal programs were run against a fresh archive of the exact starting head and
against the revised tree. No source diagnostic content is published here.

| Input or action | Starting head | Revised tree |
| --- | --- | --- |
| Missing/0/1 version, findings only, precise, detail absent | partial gate false; public v1/v2 precise | partial gate true; public v1/v2 partial; source finding retained |
| Same report as an explicit review file | zero findings, no error | fixed unsupported diagnostic; partial/unsupported gate true |
| Serialized unsupported operation axis | unsupported gate false; public partial | unsupported gate true; public unsupported |
| Append source operations after decoding an empty report | public operation count becomes 1 | private empty snapshot remains empty |

API decoding now keeps independent private finding snapshots for both lists,
retains unsupported axes, and keeps an explicit empty private operation snapshot.
Source diagnostics, risk, locations, wire fields and existing finding suppression
semantics remain. Atomic failure leaves the prior receiver unchanged. This is a
precision compatibility change, documented in the migration guide, not authority.
In-process reports retain their existing path.

Delivered specification answer `405b080f-aa34-4e4c-a396-789d1132b7af` selected A:
CLI/file discovery recognizes possible source reports and emits a fixed unsupported
marker rather than importing diagnostics or trusting their claims. Only application
configuration controls the marker; JSON strings are not inline suppressions.
Mixed QueryPlan/MigrationPlan risks and existing QueryPlan version error identities
are retained. Public display artifacts and unrelated JSON remain separate. The
bounded detector does not claim an audit of all possible JSON forms.

### Independent review and acceptance

The review read `precision.go`, type-loader snapshot handling, source/public/gate
paths, `query/review_receipt.go`, and the common planner/canonical identity
boundaries they use. Actual private SELECT snapshots, current owner/context and
executor checks, native ordered canonical arguments, token expiry/cancel/budgets,
source/input-set versions and optional IDs remain separate from execution handles.
No receipt/session runtime code, DB semantics, dependency or execution authority
was changed in this revision.

Existing positive/negative receipt tests were rerun. Additional fixtures check a
changed helper file loaded outside selected review paths, collisions between IDs
on distinct operations, and public operation mutation versus private gate/view.
Legacy tests cover missing/0/1 versions; absent/empty operations; missing, known
and unsupported detail; supplied matched correspondence; both finding lists and
ShowSuppressed; failed decode atomicity; config suppression/expiry; bounded,
duplicate, damaged and mixed envelopes; default v1 and opt-in v2 CLI gates;
privacy canaries; and preservation of QueryPlan/MigrationPlan risks.

AC1/AC4: unknown and unsupported survive private normalization and public
projection; source claims are not evidence. AC2/AC3: existing receipt, loader,
helper, type/escape/loop/validated API, unverified-plan and authorization boundaries
are retained and exercised. AC5: measurements below include legacy/source/public,
gate/privacy, PR1 regression, full DB tests and relevant race checks. AC6: PR84 and
Issue82 linkage, all three planned PRs, unchecked whole-Issue acceptance and PR3
limits remain. No merge or Issue close is performed.

### Final local measurements

Go: `go1.26.4 linux/amd64`. Existing dedicated database servers were healthy;
read-only server-version queries returned MySQL **8.4.6** and PostgreSQL
**16.10 (Debian 16.10-1.pgdg13+1)**. All three DSNs were initially unset. Explicit
`TEST_DB_DSN`, `TEST_MYSQL_DSN`, and `TEST_POSTGRES_DSN` were then supplied from the
repository's dedicated test environment without publishing their values.

| Command/check | Revision-2 result |
| --- | --- |
| `go test -json ./... -count=1 -p=1` with explicit three DSNs | Final source: 2,825 pass / 0 fail / 0 test skips; 22 passing test packages, 27 no-test packages |
| Dedicated MySQL/PostgreSQL regression | Included in the full command; packages run sequentially, DB subtests run sequentially; custom-driver test passes |
| `go test -race ./orm/query ./orm/review ./orm/mcp ./cmd/goquent` | Passed |
| Final focused `-race` over source-report, CLI, decoded unsupported/empty snapshot and helper/ID fixtures | Passed after mixed-plan risk correction |
| `go vet ./...` | Passed |
| `sh scripts/test-public-output-check.sh` | Passed |
| `go run ./cmd/goquent manifest check-repositories --config tests/typedfixture/repositories.json` | Passed |
| Example manifest verify with manifest/schema/policy and `--format json` | Exit 1; schema stale, policy OK, generated_code/database skipped |
| `go run ./cmd/goquent review --fail-on high ./...` | Exit 1; findings preserved |
| gofmt / `git diff --check` | Passed |

The full command was rerun after the final mixed-envelope risk correction; earlier
intermediate counts are not substituted. The revision-1 2,746/0/1-skip result and
unperformed dedicated integration remain historical; the new explicit-DSN run
resolves that local skip for this revision. `make test-integration` itself was not
invoked because its databases were already running; the equivalent full test
command used explicit three DSNs. No CI test counts or DB versions are inferred
from these local results.

A newly extracted exact-main baseline measured 1,180 findings (156 precise,
916 partial, 108 unsupported). Revised source measures **1,197** (0 precise,
1,088 partial, 109 unsupported), suppression 0; both high gates fail. Existing
risk-code counts are unchanged. Static partial increases by 16 and unsupported by
1 versus main: revision 1 added 15 partial and 1 unsupported; the new helper/session
receipt regression fixture adds one partial finding. No threshold/suppression was
changed to make the review pass. Example manifest stale/skipped checks remain
unresolved; no physical/live schema guarantee is claimed.

PR3 remains separate after PR2 review/main integration: SARIF, diff review,
expiring narrow baselines, new/worsened diagnostic retention and SARIF/JSON
location/code agreement are not implemented or accepted here.
