# Staged release validation and unresolved evidence

GQ-AI-14 PR2(2/2), [Issue #106](https://github.com/recoweft/goquent/issues/106).
This is a collection workflow, not a release authorization or a replacement for
complete private gates. Historical unchecked roadmaps are not completion ledgers.
The current finite API contract is described in [safe workflows](safe-workflows-migration.md)
and [contracts v3](../specs/goquent_ai_contracts_v3.md), including their later updates.

## Run and interpret

Python 3.12+, the repository Go toolchain, and cached Go dependencies are required.
No new dependency, service, tag, package upload, or release is created. Run from the
repository root. The output directory must be new and outside the repository:

```sh
make release-validation OUTPUT=/tmp/goquent-validation-new
make release-validation-test
python3 scripts/release_validation/run.py --output /tmp/goquent-offline-new \
  --stages inventory,offline,vet,regen,canary,example,schema,binding,observation,manifest
```

Default `inventory` invokes only `go version`. All other stages are `not-run`.
Non-native commands remove all three test DSNs from their child environment.
They never invoke the integration package, provision a service, or execute the
native ORM test names. `GOPROXY=off` and `GOTOOLCHAIN=local` prevent automatic
network dependency/toolchain acquisition. Global GOFLAGS `-json` is rejected;
only explicit `go test -json` supplies machine-readable test events.

Each invocation collects new evidence; it cannot import an earlier result.
Commands execute synchronously in the stage order below. Raw stdout/stderr,
Go/DB versions, test names, source trees, patch and resolved commits stay in the
new mode-0700 directory (files inherit umask 077). `results.json` and `commands.jsonl` (actual commands, exits and log mapping) are
private too.
Do not upload that directory, logs, source JSON, SQL, errors, identifiers, DSNs,
binding material or absolute paths to CI/PR/MCP. The stdout summary includes only
fixed stage names/statuses, numeric exits and counts. Existing CLI public views
remain supplementary displays: actual CLI exit gates use the original complete
private report, never a truncated public projection. No artifact upload is added.

| Result | Meaning |
| --- | --- |
| `pass` | This stage's command and required evidence succeeded within its stated finite scope |
| `fail` | Command nonzero or a reported test failure; preserves the real exit |
| `not-run` | Stage not selected; no evidence claimed |
| `unverified` | Missing prerequisite, unusable evidence, no executed tests, or review unknown |
| `skip` | Explicit `--skip stage`, Go test skip, or missing manifest verification inputs |

Go event counts distinguish passed/failed **test events** (including subtests),
`testskip`, passing packages and packages without test events (`no_test`). A package
without tests is not independent proof. Build/launch failure remains non-success.
Expected example refusals pass only through the existing checker, which builds
the executable and checks exact 3/4 exits and fixed markers; arbitrary nonzero
commands never become success. `go run` wrapping a program exit is not used to
infer its exact refusal status.

`collection=complete` means collection finished, even if checks failed. Exit 1
means release blocked, exit 2 means collection/input infrastructure error, and
exit 0 would mean checks passed, **not** human release authorization. In this
revision the inherited blocker register below always keeps release blocked.
Running full/race or green CI cannot remove those blockers. `--skip` is explicit
non-success, never a bypass. CI runs the orchestration regressions, not release
acceptance; its default green status can coexist with a blocked release.

## Stage checklist and evidence map

All stages use the current checkout; strict additionally requires a clean exact
HEAD and independently resolved base. Paths below are repository-relative. The
runner records individual command exits and raw output in numbered local logs.
Review input failure, risk, precision and unknown remain separate in private
results and the existing review public view. Do not substitute compile or source
existence for execution evidence. Check off a row only with fresh recorded output.

| Stage | Inputs, command / regression | Contract evidence and remaining boundary |
| --- | --- | --- |
| inventory | `go version` | Toolchain only; not ORM validation |
| offline | `go test -json -p 1 -count=1 ./tests/contracts ./orm/operation ./orm/manifest ./examples/ai-safe-orm` | 01/06/07 source-wire/public/type/legacy fixtures and nonexecuting example; supplied schema, not live DB |
| full | `go test -json -p 1 -count=1 ./...` with native prerequisites | Fresh bounded API, CLI/JSON/SARIF/MCP, MAINT03/04/05 and both DB regressions; no universal safety proof |
| race | Same full command with `-race`, after full | Race detector evidence, not distributed exactly-once or commit guarantees |
| vet | `go vet ./...` | Static diagnostics, not execution evidence |
| regen | `go run ./cmd/goquent manifest check-repositories --config tests/typedfixture/repositories.json` | 08 actual generated-source comparison; v1 fixtures retained, no baseline rewrite |
| canary | `sh scripts/test-public-output-check.sh`; Go tests matching `Canary\|Public\|Redact\|SARIF\|Sarif` in CLI/query/review/MCP | 06 public boundaries on positive/negative paths; raw failures remain local |
| example | `sh examples/ai-safe-orm/check.sh` | PR1 actual plan, expected tenant/authorization refusals and zero-dispatch regressions; trusted local deny adapter, no production auth service |
| schema | `go test -json -p 1 -count=1 ./orm/migration` | 11 schema/capability/migration/preflight mock regressions; no deployment or live freshness guarantee |
| binding | Tests `^Test(Binding\|Authorization\|Tenant\|Strict\|Identity)` in query/root ORM | 02–05/10/13 structural/tenant/schema, six families, expiry/current inputs/CAS and ordinary high refusals; spy/mock not native |
| observation | Selected `TestObservation*` unit tests in root ORM (see runner's closed pattern) | 12 bounded observation/budget/row/Tx tests; excludes native integration and benchmarks |
| native | Sequential selected tests in root ORM and tests: ObservationIntegration, PerformancePlanNative, ExternalAuthorizationDatabaseAndExternalTx, ValidatedBindingDatabaseAndExternalTx | Both native DBs, version logs and actual test events; finite restricted-role, PostgreSQL operator/cast and post-Begin loopback fault cases; see [case matrix](native-performance-boundaries.md) |
| ordinary | Built CLI `review --format review-ci-json --fail-on high ./...` | 09 complete private risk gate; separate from strict comparison |
| strict | Built CLI `review --format review-ci-json --fail-on high --fail-on-precision partial --diff PATCH --diff-base-root BASE HEAD` | Clean nonempty exact-commit diff, full private gates and comparison classification; no new suppression/baseline |
| manifest | Built CLI `manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json` | Supplied schema/policy only; generated_code/database remain `skip` even on CLI exit 0 |

Native/full/race require `--native-disposable-fixture` **and all three** explicit
DSNs matching the existing default disposable Compose fixture. TEST_DB_DSN must
be identical to TEST_MYSQL_DSN. The current runner deliberately supports only
the checked-in loopback ports 3306/5432 and default fixture credentials/database;
it rejects other targets as unverified. Supply these fixture-only values from
`Makefile`/`docker-compose.yml` locally, never production DSNs. Existing Compose
services must already be running. The runner checks both service images/ports and
queries both versions sequentially before any native test. It does not run db-up.
DSN presence or container health alone is not native pass; retain actual test
results and version records. Test packages use `-p 1`, and their database subtests
are sequential. Dedicated concurrency cases inside one fixture remain intentional.

```sh
# Export the three explicit disposable-fixture DSNs locally first.
python3 scripts/release_validation/run.py --output /tmp/goquent-native-new \
  --native-disposable-fixture --stages inventory,full,race,native
# Commit the implementation and keep the checkout clean before strict review.
python3 scripts/release_validation/run.py --output /tmp/goquent-review-new \
  --stages ordinary,strict,manifest --base BASE_COMMIT --head HEAD_COMMIT
```

Strict materializes both commits using local git archive into the private output
directory and uses a nonempty git diff; it rejects dirty/current-head mismatch,
missing commits, identical commits and unsupported archive links. Resolving refs
or fetching them is the operator's responsibility. Exact hashes remain private
in `commits.json` and may be stated in the PR as requested metadata.

## Blocker, limitation and unverified register

These are inherited observations, **not fresh validation results for this PR**.
Fresh invocation evidence belongs in the PR/report, with local/CI and native/mock
clearly separated. A later success is not proof of the cause of an earlier failure.

[GQ-MAINT-06 review triage](review-triage.md) links fresh finite risk, precision
and comparison-unknown counts to three reproducible groups and separate repair
priorities. Its uncollected loader-cause and unknown-branch partitions remain
explicitly unavailable. It does not replace these inherited observations or
remove any fixed runner blocker; final-head evidence belongs in the linked PR.

| ID / classification | Guarantee, current evidence and impact | Required next evidence or correction |
| --- | --- | --- |
| review-risk-precision / blocker | PR107 strict input_failed=false, risk and precision failed. Full/race 3701 pass each did not discharge review gates | Fresh ordinary and exact strict review; investigate findings within their supported contract. Do not lower thresholds or suppress/bulk-rebaseline |
| review-unknown / unverified blocker | PR107 1473 unknown/new1; correspondence success is not complete precision | Fresh comparison and classify unknown separately from input failure, new and risk. Review unsupported contexts; preserve unknown until resolved |
| manifest-coverage / unverified blocker | Example supplied schema/policy OK; generated_code/database skipped | Supply independently justified relevant evidence or retain explicit skipped coverage; never regenerate legacy fixture to hide it |
| native-performance-limits / unverified blocker | [Finite added cases](native-performance-boundaries.md) cover restricted roles, PostgreSQL custom index/cast refusal and post-Begin loopback faults; broader combinations remain unverified | Retain per-case fresh evidence, server versions, permissions and fault scope; these tests never clear this fixed blocker |
| historical-testmain-cause / unresolved blocker | Intermediate testmain unknown did not recur; original cause undetermined | Reproducible evidence and source/loader cause analysis; MAINT03 finite exception is not universal elimination |
| historical-expiry-cause / unresolved blocker | Historical expiry failure host/root cause unknown | Correlated fresh deadline/lifecycle evidence and cause investigation; rerun success alone is insufficient |
| human-release-review / decision boundary | Automated checks do not adopt whole-Issue AC or production readiness | Separate authorized human review of all remaining blockers and deployment conditions |

Supported external authorization is only the six Validate*/ExecuteValidated*
families. Ordinary/generic/RETURNING/compound high-risk authorization is
unsupported, owned new-Tx paths retain their restrictions, and directsql remains
outside interception. SQL/value-specific external approval, physical DB identity,
live freshness, revocation, exactly-once and commit are **outside the guarantee**,
not tests awaiting an invented pass. CLI/MCP remain read-only; migration apply
requires separate human control. Rust/Quent is a different project.

If collected evidence suggests a semantic API/wire/hash change, existing gate
reinterpretation or broad runtime repair, stop dependent implementation and
request specification consultation with evidence/options/impact. This workflow
cannot redefine a gate or add a guarantee in prose. Do not close the whole Issue,
merge, tag, publish a release/package, or accept overall AC on these results.
