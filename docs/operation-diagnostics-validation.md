# Operation diagnostics validation record

GQ-AI-07 PR2 (2/2), [Issue #76](https://github.com/recoweft/goquent/issues/76).
This is implementation and test evidence, not whole-Issue acceptance, merge,
deployment or user completion.

## Identity and environment actually checked

- Fixed work 972930ca-ea79-4b2d-a7b3-3709c4822a7b, revision 1; execution
  cb05cb7f-0b81-4fd1-8047-c30dc76728c4 matches the supplied request.
- Actual cwd /home/murai/github/goquent, Ubuntu 24.04.5 on WSL2
  6.18.40.1-microsoft-standard-WSL2, Go go1.26.4 linux/amd64.
- Fetch/push remote git@github.com:recoweft/goquent.git. Worker gh identity
  recoweft and repository ADMIN permission, SSH authentication and push dry-run
  verified separately from Issue connector access.
- Specified feature/gq-ai-07-operation-diagnostics already existed locally,
  clean at remote main 471b24a69282668744fe3f035da435ff4399d250. Local main was
  older; it was not treated as the current baseline or overwritten.
- GitHub API verified PR77 MERGED at 2026-10-07T16:18:49Z, merge SHA above.
  All-state head-branch PR searches returned zero before publication. Issue76
  was found. No cross-work RelayWeft search was performed because this execution
  forbids selecting/searching for another work.
- MySQL 8.4.6 and PostgreSQL 16.10 (Debian 16.10-1.pgdg13+1) queried directly
  from the healthy existing local fixture containers.
- Main required-status-check protection returned HTTP 404 (branch not protected);
  branch-rules API returned an empty list. The CI workflow remains unchanged.

Initial sandbox DNS/network failures did not establish invalid credentials;
network-enabled read checks subsequently passed. Push dry-run alone is not
proof of actual push/PR creation; publication evidence is recorded separately.

The A-D diagnostic contract was adopted through the same-revision specification
consultation and its adoption clarification. Transmission permissions and
specification adoption were kept distinct. No code or contract was changed
while that decision was pending.

## Local commands and results

Implementation commit tested:
e08c5a16adc454b65bf7695fcd9993185fc0889d.
The later validation-document commit changes no Go implementation or fixture.
Final publication records the final head and its additional full test result.

All full/race/integration runs below explicitly supplied TEST_MYSQL_DSN,
TEST_POSTGRES_DSN and TEST_DB_DSN (the registered-driver test selects the MySQL
fixture). Values are intentionally omitted. GOCACHE was a writable local /tmp
directory. Suites ran sequentially against shared DB fixtures, not concurrently.

| Source / command | Result |
| --- | --- |
| Clean archived starting main: go test ./... -count=1 -json | Exit 0; 1778 test/subtest pass, 0 fail, 0 test skip; 22 tested packages and 23 no-test packages |
| Implementation: go test ./... -count=1 -json | Exit 0; 1931 pass, 0 fail, 0 test skip; 22 tested packages and 24 no-test packages |
| Implementation: go test -race ./... -count=1 -json | Exit 0; 1931 pass, 0 fail, 0 test skip; same package counts; no race report |
| Implementation: GOFLAGS=-json make -s test-integration | Exit 0; 1931 pass, 0 fail, 0 test skip; same package counts; healthy compose fixtures |
| Implementation: go vet ./... | Exit 0 |
| sh scripts/test-public-output-check.sh | CI_WRAPPER_CHECK_PASSED |
| gofmt on changed Go files; git diff --check | Pass |

Counts are parsed Go JSON events including subtests; no-test packages are not
skipped tests. Delta versus independently rerun main is +153 test/subtest
passes and +1 test-helper package without its own tests. The helper uses only
the standard library and is imported by test files; go.mod/go.sum are unchanged.

Local log SHA256 (ephemeral local evidence, not durable CI artifacts):

- Main: 07c12689c1ae9c78ee0c43f5e7002a42e374afbecd0987eb49f189a590f68f6a
- Full: ec590d23f33ab2f7a88215fc503972ff489b47d81f20bde68a7e165a81520051
- Race: 37030e693efbf94203cce02f7322719eece9a8f824edd69dd3e6f481664130da
- Integration: 038a4b1c79cd8d4761367af05e39a86e37f8efcd14083d2ad5f65b09755398ea

Development failures are not counted as passing evidence. Initial new tests
incorrectly treated Serve output as line JSON for line input; Serve always
frames output. A schema test helper initially compared number lexemes instead
of mathematical enum equality, failing two version-difference cases (one full
attempt recorded 1925 passes / 3 fail events including their parent). These
test-helper errors were corrected. A history assertion initially included a
separate planner warning in the missing-evidence assertion and was corrected.
One baseline runner failed before tests due to its Makefile parsing regex.
One targeted run could not write the default home Go cache for two packages;
dedicated /tmp-cache runs above passed. No failed attempt is presented as a pass.

## Review and manifest

The exact CLI command on both main and implementation was:

go run ./cmd/goquent review --fail-on high --format json ./...

Both exit 1 with 485 findings and zero suppressed findings. Public output is
truncated as designed; the complete in-process review report was separately
summarized locally to compare normalized path/code/level/precision multisets:
zero additions and zero removals. No thresholds, suppressions or rules changed.

| Scope | Blocked | Destructive | High | Medium | Precise | Partial | Unsupported |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Main and implementation | 11 | 20 | 70 | 384 | 239 | 157 | 89 |

Manifest commands used examples/ai-safe-orm/schema.json and policies.json:

- go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json --format json:
  exit 1, schema stale / policy ok; generated_code and database skipped.
- go run ./cmd/goquent manifest --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json --unsafe-local-output <new-private-temp-file> --format json:
  exit 0. Only the existing explicit local artifact API was used.
- The same verify command with that new manifest: exit 0, schema and policy ok;
  generated_code and database still skipped.

An initial command incorrectly included a positional "generate" token and did
not create the intended artifact; the dependent verification failed. Correct
CLI syntax above then passed with a fresh artifact path. The checked-in example
manifest/schema/policies remain unchanged. Fresh compares supplied snapshots,
not live schema or physical database identity. No migration or generated code
changed, so migration planning/code regeneration checks are not applicable.

## Acceptance evidence and continuing limits

| AC | Evidence |
| --- | --- |
| AC1 | Private checked/missing/evidence/action records; stable refusal codes and logical list/IN indexes; known errors.Is; internal/public mapping and API migration in operation-diagnostics.md |
| AC2 | 57 shared canonical/legacy/boundary cases across Schema, API/root/Validate, CLI JSON/pretty, MCP direct/RPC and both Serve input styles; seven wire/lexeme differences; native scalar and DB-scoped settings tests |
| AC3 | Closed catalog fixture, canary/opaque/cycle/oversize rejection, no value/error callbacks, tampering and standalone/nested fmt/JSON, strict wire/version/duplicate/null/error/sink/budget tests, refusal priority after truncation, private MCP adapter and top-level ID boundary |
| AC4 | PR1 typed precision/presence/constraints/budgets/tenant/LimitExact tests and full existing private seal/current/expiry/CAS/owner/Strict/executor/driver/Tx/scanning/bool suites preserved; diagnostics do not dispatch SQL |
| AC5 | Sequential full/race/explicit-DSN both-DB integration/vet; unchanged 485 review findings; manifest outcomes and CI/local distinction above |
| AC6 | Specified branch/base/title, Refs #76 and Issue URL are publication requirements; actual PR and Issue update are recorded after publication, with no inferred PR number |

No public diagnostic restores SQL, args, a private seal or ValidatedPlan.
Decoded/tampered claims are not execution evidence. Internal details cannot be
obtained through a new public getter. Schema coverage is canonical and bounded,
not a universal Schema/runtime equivalence claim. Declared types, builtin driver
subsets and numeric binding checks do not prove live DB/session/collation/physical
identity or arbitrary external-effect atomicity. No array binding or private
decimal-domain expansion is introduced.

CI uses its existing fixed-output wrapper and does not publish per-test/skip
counts or explicitly supply TEST_DB_DSN. CI success is therefore separate from
the measured local complete suites. Exact final-head CI URLs/status are recorded
in the PR and RelayWeft report after publication. This record does not claim a
CI run before it happens, an independent human review or every third-party MCP
client. All whole-Issue acceptance checkboxes remain for separate review.
