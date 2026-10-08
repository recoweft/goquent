# Typed repository validation record

GQ-AI-08 PR1 (1/2), [Issue #79](https://github.com/recoweft/goquent/issues/79).
This records measured implementation evidence, not merge, whole-Issue acceptance,
live authorization or user completion. The final PR/report records the commit and
CI URLs. API and limits are in [typed repositories](typed-repositories.md).

## Identity and environment checked

- Supplied work b8a46ad4-9e72-47d8-b980-6264b5f3615f revision 1 and execution
  73eb0513-8378-45f6-aa3d-1b18ad32df52 matched this request and the delivered
  specification answer. No other task was selected and start was not registered
  again. The controller's internal registration state was not independently read.
- Actual cwd /home/murai/github/goquent; Ubuntu 24.04.5 on WSL2, Go 1.26.4
  linux/amd64. Origin fetch/push was recoweft/goquent over GitHub SSH.
- Worker API identity/permissions and SSH authentication were checked separately
  from Issue connector access. Push dry-run succeeded; actual publication is
  recorded separately. Sandbox DNS and .git write restrictions initially refused
  checks/branch creation; permitted retries succeeded. This was not invalid auth.
- Fetch established main d5a5e2477b34ab46cf1e59c81138f694c96e2578. GitHub MERGED
  status and ancestry of PR67/68/69/77/78 were checked. A second prepublication
  fetch returned the same main. The initial working tree was clean on an older
  task branch; the requested new branch was created from origin/main.
- Issue79 was OPEN with the supplied plan. All-state matching PR searches returned
  zero before publication. Branch protection returned 404 and branch rules [];
  no configured required-status-check list was found. Existing CI stays unchanged.
- Healthy existing fixtures: MySQL 8.4.6 and PostgreSQL 16.10
  (Debian 16.10-1.pgdg13+1), queried directly. No production database was tested.

The same-revision specification answer selected opt-in generation, a shared SQL
type parser, nominal keys, explicit projections, the shared private read adapter,
and readonly metadata, with the limitations documented in the API guide.
An initial detailed consultation payload was rejected by automatic approval
review; a minimized specification-only question was accepted and answered.

## Commands and measured results

All full/race/integration suites explicitly supplied TEST_MYSQL_DSN,
TEST_POSTGRES_DSN and TEST_DB_DSN. Values are omitted. Shared DB suites ran
sequentially. Go JSON pass/fail/skip counts include subtests. No-test packages are
counted separately. A writable local Go cache was used.

| Source / command | Result |
| --- | --- |
| Clean archived starting main: go test ./... -count=1 -json | 1931 pass, 0 fail, 0 test skip; 22 tested / 24 no-test packages; exit 0 |
| Final implementation: go test ./... -count=1 -json | 1997 pass, 0 fail, 0 test skip; 22 tested / 26 no-test packages; exit 0 |
| Final implementation: go test -race ./... -count=1 -json | 1997 pass, 0 fail, 0 test skip; same package counts; exit 0, no race report |
| Final implementation: GOFLAGS=-json make -s test-integration | 1997 pass, 0 fail, 0 test skip; same package counts; exit 0 |
| go vet ./... | Exit 0 |
| go test ./orm/manifest ./orm/operation ./orm ./cmd/goquent -count=1 | All four packages pass |
| sh scripts/test-public-output-check.sh | CI_WRAPPER_CHECK_PASSED |
| gofmt on changed Go source; git diff --check | Pass |

Delta from independently measured main: +66 test/subtest passes and two no-test
packages (the internal parser and generated fixture package). Generated fixtures
are compiled both by the repository build and in a temporary external module.
The explicit compile test measures a positive call set, eight intended negative
call sets (each must fail actual go test compilation), and a further successful
collision/keyword/single-key fixture. No parser-only test is used as compile proof.
The temporary module compiler output is captured rather than publishing source.

Local ephemeral Go JSON log SHA256:

- Baseline: bc262581c904224914fcc56692110ffe432bd71a1fc1a7949bcfb486198757e7
- Full: 469f48689f8bdfcee953162d25aba5beead2bd1f89317f2db3eed73c01d2114d
- Race: d095049b73248098787781b431f23cce5c4b1c8f01d5c4b8508671581b6d281f
- Integration: f69a8594aec4a785b1435b8f44caef30195c58593b2277b463230fca18109b15

Development failures are not included in passing evidence. The first parser
extraction accidentally capitalized words inside two SQL suffix literals;
existing type tests caught unsigned/time-zone differences, and the grammar was
restored before proceeding. Initial new tests used incorrect existing option
names and one incorrect refusal identity; these test-source errors were fixed.
The first both-DB generated read run recorded 43 pass / 4 fail events because
PostgreSQL TIME returns time.Time. The generated result Scanner now handles that
representation, and also refuses the driver's special 24:00 next-day form instead
of discarding its day. Subsequent measured suites above pass. An early narrowly
filtered command had no matching tests in some packages; it was not treated as
full coverage.

## Review and manifest verification

Both baseline and final source ran:
`go run ./cmd/goquent review --fail-on high --format json ./...`.
Both exit 1. Complete in-process reports were also compared locally because the
public view truncates display. No rule, threshold or suppression was changed.

| Source | Findings | Suppressed | Blocked | Destructive | High | Medium | Precise | Partial | Unsupported |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Baseline | 485 | 0 | 11 | 20 | 70 | 384 | 239 | 157 | 89 |
| Final | 488 | 0 | 11 | 21 | 72 | 384 | 242 | 157 | 89 |

Normalized path/code/level/precision comparison has three additions and zero
removals. All additions are in tests/typed_repository_test.go: two RAW_SQL_USED
findings for explicit fixture setup and one DESTRUCTIVE_SQL_DETECTED for fixture
cleanup. These are intentional local test operations; the findings remain visible.
Existing partial/unsupported findings remain limitations, not safe verdicts.

Manifest commands used the unchanged examples/ai-safe-orm schema/policy files:

- Verify the checked-in manifest with --schema, --policy and --format json:
  exit 1; schema stale, policy ok, generated_code/database skipped.
- Generate a new explicit local manifest with the same inputs, --code
  tests/typedfixture and --unsafe-local-output to a new private temporary path:
  exit 0.
- Verify that local manifest with the same schema/policy/code inputs: exit 0;
  schema, policy and generated_code ok; database skipped.

The supplied snapshot comparison is not live freshness or regeneration CI.
No migrations are proposed/applied, so migration-plan review is not applicable.
Handwritten and checked-in example manifests are unchanged. Production driver
and dependency inventories are unchanged.

## Acceptance evidence and unverified scope

| AC | Evidence |
| --- | --- |
| AC1 | Existing skeleton pipeline opt-in; generated model/columns/keys/projections; real positive compilation; name/collision/keyword/import handling; deterministic fixture comparison |
| AC2 | Eight actual negative compilation cases; SQL parser sharing; runtime enum/decimal/date/NULL/IN/zero-reference checks; unsupported mappings remain unknown |
| AC3 | Generated/dynamic SQL, native ordered args, condition tree, warnings, blocked decision and DiagnosticView parity; DB-free planning; refusal executor0; trusted tenant key and automatic/manual context; seal tamper/consumption; real both-DB reads, Strict, custom executor, external Tx and registered driver; scanner/bool/NULL/large integer/decimal/time/LIMIT0/no-row/cancel behavior |
| AC4 | Source presence/readonly/generated retention, snapshot kind/fingerprint/fixed version, omission formatting/JSON, CLI aliases and non-overwrite/0600/CI restrictions; PR2 boundaries documented |
| AC5 | Measured baseline and final full/race/integration/vet, compile fixtures, CLI review/manifest checks and output wrapper; actual final-head CI recorded in publication report |
| AC6 | Specified branch/base/title, Refs #79 and Issue URL, PR1(1/2) plan and PR2 remainder are publication requirements; actual URL and Issue update recorded separately |

Passing tests do not prove every DB type, collation/session, third-party driver,
physical DB identity, live policy/schema freshness, business authorization, or
external-effect atomicity. Ordinary immediate reads do not acquire the extra
ValidatedPlan/current/expiry protocol. Explicit conversions and untyped constants
are outside nominal assignment guarantees. Unknown fields and fractional decimal
private binding remain blocked/refused. No new driver adapter or array binding
was implemented. An independent human review was not performed by this execution.

CI's fixed-output wrapper does not publish individual counts/skips or explicitly
set TEST_DB_DSN. CI success must therefore be reported separately from local
1997/0/0 evidence. Docs publishing runs on main, not this feature PR. PR2 still owns
three-state patches, nonnullable SetNull rejection and regeneration CI. No merge,
Issue close or whole-Issue acceptance is performed here.
