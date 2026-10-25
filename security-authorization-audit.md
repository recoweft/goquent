# Bounded authorization and tenant isolation audit

GQ-SEC-02 / planned PR1, [Refs #133](https://github.com/recoweft/goquent/issues/133).
This audits a finite set of execution and structured-input boundaries. It does
not close Issue 133, certify all attacks absent, or approve a release. PR2's
all-consumer public-output audit, SEC03 and REF01 are separate work.

## Source and prerequisites

The source baseline is main `cfca8a457a7c80988afced0db2cfa7c8bfe79ff9`.
PR137 head `0e72ba90874c5cf166a5c98bd07a3fad49f71f76` is its ancestor
(local merge-base and GitHub compare); PR137 is merged. Before changes, exact
main runs CI **40203761962**, Security **40203761954**, and Docs
**40203761912** completed successfully. Docs build and publish both succeeded;
gh-pages `e3dc5c48c884922ed8964366fa5024dfaaf29441` records deployment of that
main SHA. CI was initially in progress and was not counted as success then.
The first weekly scheduled Security event remains unverified; no substitute
dispatch was performed. Issue 132 was read back as open.

The worker checked the requested cwd, Ubuntu 24.04.5/WSL2 kernel, clean starting
tree, HEAD, origin, root AGENT.MD, contract inventory and PR template. GitHub API
authentication, repository push permission, SSH read access and push dry-run to
the planned branch succeeded under approved execution. Sandboxed DNS failed;
that was not evidence of invalid host credentials. No existing remote branch or
PR in any state matched the planned branch. Actual push/PR results and final
source SHA belong in the PR/controller report, not a self-referential hash here.

The contract sources are [tenant policy](tenant-policy.md),
[planned execution](planned-query-execution.md),
[validated binding](validated-plan-binding.md),
[external authorization](external-authorization.md), and
[the versioned inventory](../specs/goquent_ai_contracts_v3.md). Historical rows
in the inventory are read with their later implementation updates.
[Dependency audit](security-dependency-audit.md) and
[workflow security](security-workflows.md) remain applicable: Go 1.26.10/local,
scanner v1.9.0, two residual x/mod module-only advisories, and 15 historical
graph-only advisories. No old raw capture or scan is fresh evidence for this PR.

## Authority and entry matrix

Caller DSL, payloads, primary keys, reasons and decoded plan/operation fields
are input, not authority. Trusted server code must authenticate and authorize
tenant/subject before supplying NewApplicationTenantContext, application schema,
binding current state and provider registration. Merely calling an application
constructor on a request's tenant claim does not authenticate the claim.
The explicitly trusted provider adapter authenticates its external authority;
the ORM does not authenticate provider internals or observe external revocation.

Private planner seals, owners, executor references, registration instances and
single-attempt handles are distinct from public JSON, reasons and hashes. Public
diagnostics cannot replace the private execution material. Schema freshness,
physical database identity, dishonest executors and stale application assertions
remain application responsibilities.

| Entry / implementation source | Supported boundary and refusal | Existing evidence reused |
| --- | --- | --- |
| Query SELECT/Get/First/Count, UPDATE/DELETE; query.go, plan.go, execution.go | Ordinary sealed dispatch; conditional Strict requires supported typed tenant predicates on all branches. Compatibility without tenant policy retains documented heuristics, not isolation proof. | TestTenantAllBranches, TestTenantAutomaticOuterAnd, TestPlannedExecutionReadDispatch/WriteDispatch, TestPrivateExecutionIgnoresPublicVerdictsAndValues |
| Query INSERT/batch/ignore/upsert/update-or-insert; tenant_write.go, write_plan.go, query.go | Every effective candidate; tenant fill only when absent; mismatches refused. Protected assignments and unsupported conflict constraints refuse. MySQL checks all potential unique conflicts; PostgreSQL the supplied target. | TestTenantWriteRowsAndReturning, TestTenantMalformedSchemaAndConstraints, TestTenantPolicyDatabaseSemantics |
| InsertUsing / subqueries / related joins | INSERT SELECT and unsupported subqueries/compound expressions stay refused under Strict. Supported INNER/LEFT reads need per-alias tenant proof; nullable-side ON is not moved to WHERE. | TestTenantAliasesAndUnknown, TestTenantPolicyDatabaseSemantics |
| DB/generic CRUD, scoped reads/writes, batch/upsert; write.go, scope.go, querybridge adapter | Shared Query gates; effective Columns/Omit and PK extraction precede inspection. Destination DB supplies settings/executor for scoped destination helpers. No generic external-authorization terminal. | TestGenericPlansAndRejections, TestGenericConflictCoverageAndRiskParity, TestScopedBranchesAliasesSoftDeleteAndSettingsIsolation |
| PostgreSQL RETURNING and InsertGetId | Final projection is reinspected and privately sealed; PII/forbidden columns refuse. MySQL generic RETURNING unsupported. A returned row is not cardinality or commit proof. | TestReturningReinspectionAndPrivateOriginal, TestGenericReturningDispatchAndResults, TestReturningIdentifierContracts |
| Nested collection, idempotent recipe, hooks; compound_plan.go, nested_write.go, transaction_hooks.go | Known statements prechecked; opaque Strict scopes/hooks refuse before Begin/callbacks. Compatibility preserves earlier valid effects and late refusal. Arbitrary callbacks are not statically authorized. | TestCompoundKnownRefusalsBeforeBegin, TestStrictOpaqueRecipes, TestStrictScopeEffectsNeverRun, TestCompatibilityScopeChangesLaterDelete, TestAuthorizationNestedAndHooksRetainGate |
| Six Validate / ExecuteValidated families; binding.go, authorization.go, authorization_execution.go | Validate provider0/DB0; high-risk supported execution calls provider once, then rebuilds/checks and checks at dispatch. Low-risk calls provider0. CAS consumes any attempt including refusal/error. | TestAuthorizationValidation, TestAuthorizationBindingCurrentAndOwnerRefusal, TestAuthorizationExecutionRejectionsAndConsumption, TestAuthorizationConcurrentSingleAttempt |
| External Tx, custom Executor, inherited DB wrappers | Same gates through supported ORM entries; no ownership transfer, physical identity claim or direct-executor interception. | TestGenericTransactionAndExternalExecutorInheritance, TestStaticNestedTransactionAndCallerOwnedTx, TestExternalAuthorizationDatabaseAndExternalTx, TestValidatedBindingDatabaseAndExternalTx |
| Owned atomic/version writes; write_results.go | High-risk authorization refuses before provider/Begin when an owned transaction would introduce another executor/probe boundary. Existing low-risk ownership and storage checks remain. | TestAtomicWriteResultLifecycle, TestAtomicRefusalsBeforeSQL, TestAtomicMySQLStorageRefusal, TestAtomicMySQLTriggerRefusal |
| DB Raw / SQLDB / direct driver or executor | Raw retains its gate and explicit unsupported Strict status; direct DB/Tx/Executor access is outside interception. CLI/MCP remain read-only. | TestAuthorizationOrdinaryFacadeRefusesWithoutFallback, TestAuthorizationRiskDowngradeCannotPermitRaw |

Test sources are under `orm/query/*_test.go`, `orm/*_test.go`, and
`tests/{tenant_policy,external_authorization,plan_binding}_test.go`; the native
tests validate disposable local fixtures, not a production authorization service.
No implementation of #60/#66/#70/#103 is recreated.

MySQL owned atomic writes can issue two internal storage probes per supported
table (engine, then trigger count), with early termination on refusal; PostgreSQL
has no equivalent storage probe in this path. A probe is not a permitted body
write. This audit retains those existing tests and distinguishes them from the
new spy tests, whose rejected paths have zero executor calls of any kind.

## Structured DSL trace

| Input | Trace and finite conclusion |
| --- | --- |
| Table/reference/alias | Query builders call dialect SQLUtils, then sqlutils/reference.go. Supported reference parts/aliases are quoted; an unparsed reference falls back to one quoted identifier with doubled quote characters. Strict separately resolves known table/schema/alias identities. Quoting is not an allow-list or tenant proof. |
| SELECT columns | Query.Select validates identifier-like fields before the builder. Expressions/aliases require their explicit expression APIs; Strict refuses unsupported projections. Generic literal/path quoting is a distinct documented contract. |
| WHERE/JOIN operators, sort direction | Public Query methods use validateConditionOperator/validateOrderDirection before builder dispatch. The operator set is closed; invalid directions are errors at this public boundary. Internal builders are not independent untrusted-input APIs. |
| JSON | WhereJSONText/HasKey use quoted column paths and bound key/value parameters on PostgreSQL, then a Raw predicate representation. Arrow/path assignment and lower-level JSON expression construction are not interchangeable with safe identifier quoting. Strict refuses unsupported JSON/expression forms. Arbitrary expression strings are not certified safe by this audit. |
| Column sets, PK, conflict and assignments | Generic extraction applies Columns/Omit/PK options, then querybridge and tenantRows/planReturning inspect effective payload/projection. PK selection alone does not authorize a tenant. Protected columns and unsupported expressions/conflict coverage refuse. |

No real tenant, credential or hostile production system is used. Punctuation and
invalid-token fixtures are harmless. Raw APIs, arbitrary callbacks and all
possible expression grammars are not expanded into supported authorization.

## Added evidence and repairs

No new supported-contract runtime defect was reproduced in this bounded review;
there is no production-code repair or API/wire/Compatibility change.

- `TestSecurityAuditAuthorizationSixFamilyMutation`: both dialects, all six
  families, unchanged inputs versus provider-time and pre-dispatch changes.
  Changes cover predicates, count column sets, typed write values and batch row
  ordering. Validate is provider0/DB0; rejected dispatch is zero; attempts remain
  consumed. Normal cases compare SQL to the validated statement and independently
  assert concrete typed arguments at the Executor boundary before driver coercion.
- `TestSecurityAuditMassAssignmentSelection`: both dialects, full payload and
  tenant assignment refuse, while Columns/Omit select the intended score update.
  A caller's excluded tenant value does not replace the trusted application tenant;
  typed PK/score/tenant arguments are checked and caller data stays unchanged.
- `TestSecurityAuditStructuredDSLRefusal` and
  `TestSecurityAuditCompatibilityIdentifierRendering`: invalid operator/direction,
  projection/alias/column sets, JSON/path assignments and punctuation quoting.
  Compatibility rendering is tested separately from Strict support/refusal.

Existing independent tests supply request/subject/tenant/owner/executor/family/
scope/target/context/expiry mismatch, old decision reuse, provider missing/deny/
error/invalid decisions, opaque JSON lifecycle, deadline equality, context/error
identity and one-attempt concurrency evidence. The audit adds missing cross-family
mutation coverage instead of duplicating those matrices. No sleep or repeated
cancellation fixture is used to manufacture concurrency evidence.

Sensitivity was tested outside the repository by replacing authorization
rechecks with a no-op while retaining dispatch and the hook: all 12 unchanged
cases passed and all 24 changed-input cases failed (test exit 1). The real source
passed. An earlier mutant omitted the required hook entirely and was too broad;
a subsequent temporary edit had an unused-variable build failure. Neither is
counted as sensitivity evidence. Temporary mutants were never committed.

## Validation and remaining work

Actual toolchain: Go **1.26.10 linux/amd64**, `GOTOOLCHAIN=local`, explicitly
selected from the installed toolchain directory. Default PATH/local Go was 1.25.0
and was not used for verification. Raw local diagnostics stay in an owner-only
temporary directory outside repository/analysis source and are not uploaded.
Sandbox Docker socket refusal was followed by a successful approved daemon
diagnostic; this alone was not treated as successful integration.

The final PR/controller report records exact-head CI/Security and commit
identities. `make test-integration` passed with explicit local fixture DSNs
(MySQL 8/PostgreSQL 16; `go test ./... -count=1`). The non-verbose full log does
not enumerate individual skips. A subsequent verbose run of
`TestExternalAuthorizationDatabaseAndExternalTx`, `TestTenantPolicyDatabaseSemantics`
and `TestValidatedBindingDatabaseAndExternalTx` passed both dialects: 9 PASS
entries including the 3 parents, zero SKIP and zero FAIL entries.
Focused added tests, targeted race, go vet, public-output
wrapper regression, strict-input regression (8 cases), and repository regeneration
comparison passed locally. SQL review exits **1**, with 1510 findings and a
truncated public list; this is not a clean SQL review. Manifest verification exits
**1**: schema/policy OK, generated_code stale, database skipped. No manifest or
gate was regenerated/weakened to conceal those results. Initial test-development
failures were fixture expectations (SQL spacing and rejected SELECT punctuation),
not demonstrated runtime defects; they were corrected before verification.

Reproduction commands (with the selected Go first in PATH and a writable GOCACHE):

```sh
GOTOOLCHAIN=local go version
go test ./orm ./orm/query -run '^TestSecurityAudit' -count=1
go test -race ./orm ./orm/query -run 'Test(SecurityAudit|Authorization|Tenant|Binding|PlannedExecution|Generic|Compound|Strict|Nested|StaticNested|InsertOnce|CompatibilityScope)' -count=1
make test-integration
# Use the same explicit local fixture DSNs as make test-integration:
go test ./tests -run '^Test(ExternalAuthorizationDatabaseAndExternalTx|TenantPolicyDatabaseSemantics|ValidatedBindingDatabaseAndExternalTx)$' -count=1 -v
go vet ./...
sh scripts/test-public-output-check.sh
python3 scripts/release_validation/check_strict_inputs.py
go run ./cmd/goquent manifest check-repositories --config tests/typedfixture/repositories.json
go run ./cmd/goquent review --fail-on high ./...
go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json --format json
git diff --check
```

Capture command output before execution under the repository's public-output
rules. Neither the nonzero review/manifest results nor non-verbose skip inventory
is converted to a passing acceptance claim. Docs and schedule are not selected by
this PR's CI/Security events and are not claimed as executed for its head.

| Classification | Owner and finite exit condition |
| --- | --- |
| Actual defect | None reproduced in the supported finite cases; maintainers triage any future concrete reproduction separately. No claim of zero vulnerabilities. |
| Missing evidence | PR maintainer checks final exact-head CI/Security and recorded native results. Schedule first actual event stays with SEC01 workflow maintenance; PR2 owns all-output-sink audit. No new concrete public leak was established here. |
| Guarantee limit | Application/provider owners authenticate tenant/subject and supply current schema/settings; executor honesty, external revocation, callback effects, physical DB identity and post-dispatch ordering remain outside this contract. Do not reinterpret these as repairable test failures. |
| Historical cause unknown | Existing SQL-review failures, stale/skipped manifest dimensions, historical native unknowns and fixed release blockers retain their owners and original closure evidence requirements. Green tests here do not resolve their causes. |
| Release decision | Release owners retain acceptance/release authority. No readiness, merge, Issue closure, release/tag/package publication or next work is performed by this PR. |

Implementation (tests/documentation), verification (bounded executed checks),
integration (draft PR/review), and release approval are four separate states.
Only maintainers can accept integration or release. Issue 133's overall acceptance
also includes separate PR2 work and remains open.
