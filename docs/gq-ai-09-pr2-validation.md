# GQ-AI-09 PR2 validation

Management ID: GQ-AI-09. Planned PR2 of 3, [Issue #82](https://github.com/recoweft/goquent/issues/82).
Branch: `feature/gq-ai-09-review-precision`; base: `main`.
This records implementation evidence, not whole-Issue acceptance or permission to merge.

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
| `go test -json ./... -count=1` | 2,745 test pass events, 0 fail, 1 skip; 22 passing test packages and 27 packages with no test files |
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
| PR2 source | 1,195 | 0 | 1,086 | 109 | 0 | fail |

Existing codes retain their counts except STATIC_REVIEW_PARTIAL (+14) and
STATIC_REVIEW_UNSUPPORTED (+1). New receipt/planning code and fixtures add bounded
static discoveries; the raw-SQL review entry retains a dynamic-SQL unsupported
finding. The old 156 shape-precise findings move to partial because unknown axes
are preserved. No findings were removed through suppression or threshold changes.
The source coverage measurement retained 932 operation records, with no global
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
  context, mutation, changed source, duplicate, old/foreign/zero receipt and
  altered-reference negatives; strict v1/v2/version tests; privacy canaries;
  Executor-zero spy; opaque Valuer/Stringer/Marshaler refusal; budget/cancel;
  inherited suppression/expiry and PR1 regressions; full/race/vet/regen checks.
- AC6: the specified PR title/branch/base and Issue linkage are used. Actual PR
  publication, Issue append and final CI are external results reported separately.
  No merge, Issue close, whole-Issue acceptance or user-completion action is taken.

PR3 still owns SARIF, changed-line review, expiring narrow baselines, retention of
new/worsened findings, and SARIF/JSON code/location agreement. No such feature is
implemented here. A successful local or CI check is not a runtime safety proof.
