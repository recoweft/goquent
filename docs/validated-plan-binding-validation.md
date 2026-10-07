# GQ-AI-05 PR2 validation

Work `715d6db3-f1ad-4a40-b91f-ff641ee70253`, revision 3, execution
`3d851d1b-a2fa-45fa-ab59-0919711cc598`.
[Issue #70](https://github.com/recoweft/goquent/issues/70), PR2 (2/2).
Branch `feature/gq-ai-05-plan-binding`, base `main`.

Tested implementation source: `368bba2f54d61af264a33cd06e7ed44e88dbd9e4`.
The following commit adds only this validation record. Head CI is recorded in the
PR and RelayWeft report after pushing; it is distinct from these local results.

## Identity and prerequisites actually checked

- cwd `/home/murai/github/goquent`; Linux WSL2 kernel
  `6.18.40.1-microsoft-standard-WSL2`; Ubuntu 24.04.4 LTS; Go 1.26.4 linux/amd64.
- Initial working tree clean on the specified branch at
  `c7eaedec298759f9cb9bd4923f4a083cb7eb9b50`. No unrelated changes overwritten.
- Both origin fetch/push URLs: `git@github.com:recoweft/goquent.git`.
  Worker `gh api user` login recoweft; repository push/admin permissions true.
  SSH fetch and specified-branch push dry-run succeeded. These are worker checks,
  not the Issue connector PAT. Actual push/PR creation is recorded separately.
- PR71 MERGED at `2026-10-06T13:17:58Z`, merge SHA above. Fetched origin/main and
  later ls-remote main matched that SHA. Main CI37469700134 and Docs37469700080
  were completed/success via API; their logs were not independently re-read here.
- Issue70 OPEN. Specified-head all-state PR searches were empty at initial check
  and immediately before publication. No other Issue/work selected or created.
  Relay cross-work/report search is not exposed and remains unverified.
- Actual AGENT.MD, database-review PR template, contracts v3 and relevant
  version/identity/planned-execution/tenant-policy sources and docs were read.
  No fictitious contracts path or replacement guideline file was created.

## Commands and outcomes

All DB suites ran serially against the repository's local fixture containers,
with TEST_MYSQL_DSN and TEST_POSTGRES_DSN explicitly supplied. TEST_DB_DSN was
explicitly set to the same MySQL fixture for registered-driver coverage. No
application credentials or production key was used. GOCACHE was
`/tmp/gq-pr2-go-cache` after the default cache proved read-only in the sandbox.
Database/network and git writes used approved sandbox escalation.

Measured servers: MySQL 8.4.6; PostgreSQL 16.10 (Debian 16.10-1.pgdg13+1).

| Command | Outcome |
| --- | --- |
| `go test ./orm/query -run TestBinding -count=1` | Pass after focused development corrections |
| `go test ./tests -run TestValidatedBindingDatabaseAndExternalTx -count=1 -v` with explicit DSNs | Both MySQL and PostgreSQL passed; six operations, bool scanning, externally owned Tx rollback |
| `go test ./... -count=1 -json` with all three explicit DSNs | Exit 0; 1,669 test/subtest passes, 0 fail, 0 test skips; 19 passing test packages, 23 packages without tests |
| `GOFLAGS=-json make test-integration` with all three explicit DSNs | Exit 0; same 1,669/0/0 and 19/23 package counts; fixture containers healthy |
| `go test -race ./... -count=1 -json` with all three explicit DSNs | Exit 0; same 1,669/0/0 and 19/23 package counts; no race reports |
| `go run ./cmd/goquent review --fail-on high --format json ./...` | Exit 1; 468 findings, 0 suppressed; details below |
| Same review command against an archive of fetched origin/main | Exit 1; 457 findings, 0 suppressed |
| `go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json --format json` | Exit 0; schema/policy match, generated_code/database missing and skipped |
| `git diff --check` | Pass |

No migration was changed, so migration planning is not applicable. No dependencies
were added; go.mod/go.sum and the existing example fixtures are unchanged.
Package-level no-test skips above are not test skips. The three full suite logs
were parsed as Go JSON events; make's recipe line was excluded from event counts.

Local log SHA256 (files under /tmp, not public durable artifacts):

- full: `af9b28a7201a93abef2d532ae5ffbf47fd3532e84ef6bddde08f02acb9ef4f81`
- integration: `15c4df83e1e4e240a67460baa1a740524054057190523f69f2daae7a59562f63`
- race: `c908e539880aa85243033eda910e832b7bff3ee6a05b6c5988032715d67aef51`

Initial focused development attempts included a compilation failure from the
wrong WhereRaw argument form and runtime mismatches from test-only non-Count
column input,
and an approval test incorrectly expecting Strict high-risk approval by reason.
Those tests were corrected; Strict was not relaxed. The final tests explicitly
confirm high-risk refusal, diagnostic approval timestamp detachment, the explicit
handle deadline and the private deadline minimum calculation separately.
A default-cache sandbox failure was an environment failure, not a passing test.

## Static review delta

Compared finding multisets by normalized path/code/level/analysis_precision/
suppressed, ignoring line/column movement. No removed findings.

| Source | Blocked | Destructive | High | Medium | Precise | Partial | Unsupported |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Current main c7eaedec | 11 | 16 | 65 | 365 | 229 | 148 | 80 |
| PR2 source 368bba2 | 11 | 16 | 65 | 376 | 229 | 151 | 88 |

The +11 consists of three medium STATIC_REVIEW_PARTIAL findings in
orm/query/binding.go, six medium STATIC_REVIEW_UNSUPPORTED findings in
orm/query/binding_test.go and two in tests/plan_binding_test.go. Threshold and
suppressions are unchanged. The older 449 baseline predates PR1; this comparison
uses actual PR1-merged main's 457. Partial/unsupported findings and manifest
aggregate fresh do not prove operation safety or live schema freshness.

## Acceptance evidence and limits

1. Six public Validate/ExecuteValidated families reconstruct using the existing
   planners/gates. Returned diagnostics have no private execution/evidence;
   key input detaches and opaque types refuse serialization. Exact SQL and native
   typed args are asserted against every one of the six Executor methods, for
   both dialects. The supported/unsupported path table and migration are in
   [validated-plan-binding.md](validated-plan-binding.md).
2. Tests cover current tenant/policy/schema/config/target/dialect/key/scope/
   generation, Query conditions/projection/order/effective settings, native value
   type, batch order, Count columns/family, owner/copy/executor/context, nil/zero,
   opaque/unknown, expiry and consumed state. Refusal assertions check a six-slot
   Executor counter remains entirely zero. Restoring old input does not restore
   a failed handle; aliases share that state. Same owner with unused INSERT
   WHERE/projection/order remains supported, including an unevaluated opaque
   WhereRaw value. This is the adopted revision-3 meaning boundary, not every
   possible Query mutation.
3. Generation is DB0; success dispatches the same fresh private record exactly
   once. Context fallback, explicit context, unsafe-comparison refusal, final
   cancellation/expiry, DB errors and concurrent handle attempts are exercised.
   Non-comparable custom Executor values work through the private reference;
   external Tx and both DB results/bool scanning are verified. No additional
   Query/data/current concurrent mutation or external-effect atomicity is claimed.
4. Shared validated_binding_v1.json distinguishes legacy/known diagnostic reads
   from unknown-version refusal. None restores a handle; spoofed low risk/approval
   is not execution authority. PR1's native/JSON numeric distinction and version
   fixture remain active. No public value hash or key getter was introduced.
5. Full/integration/race, registered-driver/Row/compound and prior scope regressions
   are included in the passing suites. Review remains exit 1 and manifest has
   skipped inputs as described above; unsupported paths do not become verified.
6. Publication uses the specified branch/base/title, Refs #70 and real Issue URL.
   Actual PR/head/CI and Issue-link update are recorded in the publication report;
   this source validation document does not invent a PR number or claim merge.

The two inherited proposals are applied as published revision-3 requirements:
explicit current supply and six families, all-attempt CAS consumption and context
rules, plus operation-meaning-only binding for unused clauses. No old question ID
was polled and no new design adoption was inferred from execution registration.
No additional specification consultation was required for these implementation
choices. No subagent or other integration was used.

Remaining boundaries: caller truth, physical DB identity, live schema/policy
freshness, key operations, arbitrary callbacks and external effects are not
attested. Generic/scoped adapters, conflict/RETURNING/nested/Raw new terminals,
GQ-AI-06 redaction and other Issues/Quent remain out of scope. Existing high-risk
Strict refusal cannot be overridden by a reason or handle. Issue70 stays OPEN;
PR/CI/report submission is not merge, whole-Issue acceptance or user completion.
