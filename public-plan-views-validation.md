# GQ-AI-06 PR1 validation

Work `85b19f72-1c52-4856-88e2-aade50def7b1`, revision 2, execution
`6e3f9ac2-474b-4679-a241-428e52132247`.
[Issue #73](https://github.com/recoweft/goquent/issues/73), PR1 (1/2).
Branch `feature/gq-ai-06-public-plan-view`, base `main`.

Tested implementation source: `c5d6d94631d334d985bf0b71e89f0680adba512d`.
The next commit adds only this validation document. Publication/head CI are
recorded in the PR and RelayWeft report, separately from these local results.

## Identity and prerequisites checked

- Supplied work/revision/execution agree with this request. Measured cwd is
  `/home/murai/github/goquent`, Ubuntu 24.04.5 LTS, WSL2 Linux kernel
  `6.18.40.1-microsoft-standard-WSL2`, Go 1.26.4 linux/amd64.
- Initial tree was clean, on the previous `feature/gq-ai-05-plan-binding` at
  `afa880fdfabbd1e226e84d2c25b06b4dac401e67`. Created the requested new branch
  from fetched main `a463e9f13f575ac4411f0d6429667e7275688cfe`; no unrelated
  changes were overwritten or unmerged branch changes copied.
- Origin fetch/push are both `git@github.com:recoweft/goquent.git`. Worker
  `gh api user` is recoweft; repository push/admin/maintain permissions true,
  active worker token has repo scope. SSH fetch and specified-head push dry-run
  succeeded. These checks are not Issue connector PAT checks.
- Initial sandbox network/.git restrictions prevented checks; approved escalation
  allowed them. The first sandbox gh auth display was not evidence of invalid
  credentials: authenticated API checks subsequently succeeded outside that
  restriction. No secret token, DSN or key is recorded here.
- GitHub Issue73 was re-read and OPEN. All-state management-title Issue search
  returned only #73; specified-head all-state PR search returned zero initially.
  Relay cross-work/report search is not exposed and remains unverified. No other
  Issue, work, revision or execution was selected or created.
- API confirmed PR53/56/67/68/69/71/72 merged. PR72 merge SHA matches fetched
  main, merged at 2026-10-07T02:04:18Z. Main CI37560199154 and Docs37560199157
  were completed/success for that exact SHA via API; their logs were not re-read.
- Root AGENT.MD, PR template, contracts v3, plan/binding/execution/settings,
  review/writers, CLI, MCP, manifest, OperationSpec/fixture paths were inspected.
  The inventory distinguishes current output risks from new-view guarantees.

## Adopted contract and acceptance evidence

The published revision-2 proposal defines the authorized implementation: fixed
classification without caller declassification, omission of all SQL/names,
whole opaque/nested Evidence/Metadata omission, strict independent view v1,
additive APIs and legacy output compatibility. No historical question was polled
and no further specification choice required consultation. Concrete names,
128/256 per-list limits and reader budgets are documented in
[public-plan-views.md](public-plan-views.md). No subagent or other integration was
used, no new dependency was added, and existing production execution code was
not changed.

1. PlanView/ReportView use closed vocabulary and bounded detached fields. Tests
   place fictional email/token/password/person canaries in SQL, identifiers,
   params, values/references, metadata/evidence/nested plans, reasons, source,
   unknown codes/enums and returned writer errors. New value/pointer JSON, nested
   JSON, String/fmt and report public JSON/pretty/GitHub writers are checked after
   tampering. Custom value methods panic if invoked; cyclic/deep/large payloads
   are ignored. Bounds, truncation, source versions and strict wire failures pass.
2. Private seal test verifies unchanged source/inspection, original ordered typed
   args/SQL, zero calls before dispatch and one-use behavior. Existing six-family,
   two-dialect, context/non-context binding tests now manipulate public views
   before checking exact six-method Executor dispatch. Both live DB suites now
   project/mutate every binding diagnostic, verify stored canary strings and
   retain external Tx rollback, scanning and bool checks. Refusal still leaves
   all six Executor counters zero. JSON views cannot restore handles or private
   execution material. No second SQL rendering or execution authority is added.
3. The English contract inventories APIs/fields, codes, classifications, wire and
   error limits, legacy migration and PR2 routes. Root guidelines, README and v3
   contract link the current implementation. Existing four envelopes, UseNumber,
   native-width/JSON lexemes, manifest fingerprints, Strict/tenant/key boundaries
   and ordinary CRUD remain unchanged. Legacy canary exposure is deliberately
   asserted by a compatibility test, not represented as fixed in PR1.
4. Required validation is below. PR1 publication references #73 and preserves
   the two-PR plan; actual URL/head/CI and Issue-link readback belong to the final
   report. PR/CI/report is not merge, user completion or whole-Issue acceptance.

## Commands and measured results

All shared DB suites ran serially against local repository fixture containers.
TEST_MYSQL_DSN, TEST_POSTGRES_DSN and TEST_DB_DSN were explicitly supplied (the
last selects the MySQL fixture for registered-driver tests). No production secret
was used. GOCACHE was `/tmp/gq-pr2-go-cache`. Measured MySQL 8.4.6 and PostgreSQL
16.10, Go 1.26.4. These local settings are distinct from CI, whose workflow does
not specify TEST_DB_DSN and uses nonverbose output with unknown test-skip count.

| Command | Result |
| --- | --- |
| `go test ./... -count=1 -json` on fetched main before implementation | Exit 0; 1669 test/subtest passes, 0 fail, 0 test skip; 19 test packages, 23 no-test packages |
| `go test ./orm/query ./orm/review ./tests/contracts -run 'TestPublic\|TestBindingSixDispatch' -count=1` | Exit 0 during focused development |
| `go test ./... -count=1 -json` on tested source | Exit 0; 1678 test/subtest passes, 0 fail, 0 test skip; 19 test packages, 24 no-test packages |
| `GOFLAGS=-json make -s test-integration` with all three explicit DSNs | Exit 0; same 1678/0/0 and 19/24; fixture containers healthy |
| `go test -race ./... -count=1 -json` with all three explicit DSNs | Exit 0; same 1678/0/0 and 19/24; no race reports |
| `go run ./cmd/goquent review --fail-on high --format json ./...` on main | Exit 1; 468 findings, 0 suppressed |
| Same review on tested source | Exit 1; 471 findings, 0 suppressed; details below |
| `go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json --format json` | Exit 0; schema/policy match; generated_code/database absent and skipped |
| `git diff --check`; gofmt on changed Go files | Pass |

Counts come from parsed Go JSON events, including subtests. Package no-test
entries are not skipped tests. The increase is nine new tests and one internal
package without its own test files (covered through public-entry tests). No
migration or code generation changed, so additional migration/codegen checks
are not applicable. No test failure occurred in the measured suites. No passing
result is claimed for the initial sandbox permission/connectivity failures.

Local /tmp log SHA256 (not durable published artifacts):

- baseline: `d25e9ef02b58ebb1b3f1c14b70296074f30a37dd3afab36c371b723f52b44e36`
- full: `5ca0fddda579fd2fb9340fbd8db8fde7177e79fd90447dc971397fa849270725`
- integration: `a5188defabddb28686d9b4f4ce3dbe68a8fc0d1cfa9ee9c8fe36042c2f878c88`
- race: `699d124e6d6b5d5a78d1dc71a3c8bca65390536b087a981e66c2fd7e9ed58c85`

The four logs were mechanically checked for absence of the fixed canary strings.
New view outputs are checked directly inside regression tests; repository fixture
source intentionally contains fictional canaries and is not a generated view.

## Review delta and retained limits

Normalized path/code/level/precision/suppressed multiset comparison ignores line
movement. No findings were removed; thresholds and suppressions are unchanged.

| Source | Blocked | Destructive | High | Medium | Precise | Partial | Unsupported |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| main a463e9f | 11 | 16 | 65 | 376 | 229 | 151 | 88 |
| source c5d6d94 | 11 | 16 | 65 | 379 | 229 | 153 | 89 |

The +3 consists of two medium STATIC_REVIEW_PARTIAL findings in
orm/query/public_view_test.go and one medium STATIC_REVIEW_UNSUPPORTED finding in
tests/plan_binding_test.go. The main baseline matches the supplied PR72 468
findings and 1669 tests. Ubuntu's patch release here is 24.04.5 versus the prior
record's 24.04.4; Go/DB versions and test connectivity match the stated baseline.

Manifest aggregate Fresh compares supplied snapshots, not live DB truth; missing
fingerprints remain skipped. Whole CLI/CI/MCP/GitHub/runtime error/manifest/generated
output leak elimination is unimplemented in PR1 and requires PR2. Source
inspection is not every-consumer coverage. Counts/positions and fixed vocabulary
are intentional disclosures; arbitrary caller field extraction, concurrent
mutation, standard-library error formatting and malicious sink side effects are
outside the documented guarantees. Caller truth, physical DB identity, live
schema state and arbitrary external-effect atomicity are not proven.
