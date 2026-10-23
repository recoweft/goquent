# Scoped external authorization contracts

GQ-AI-13 PR2(2/2), [Issue #103](https://github.com/recoweft/goquent/issues/103).
External authorization is connected only to the six existing
`Validate*` / `ExecuteValidated*` Query families: select, count, insert, batch,
update and delete. This is a finite execution boundary, not authorization support
for every CRUD helper or whole-Issue acceptance.

Plan, compile, review and Validate remain provider-free and database-free.
A successful Validate returns a **pending single-attempt candidate**, not an
execution permission. Ordinary Strict high-risk execution remains refused.

## Registration and trust

Use `query.NewAuthorization(provider, subject)` (also available through `orm`)
and `orm.WithAuthorization(registration)` or
`settings.WithAuthorization(registration)`. A zero registration clears the option;
it never means allow. Nil/typed-nil providers and empty, whitespace-only or
oversized subjects fail with `ErrAuthorizationInvalid`. Subjects are exact
strings of at most 256 bytes, with no case folding or trimming. They are separate
from `ExecutionContextInput.Source`, reasons and PII access purpose.

`AuthorizationProvider.Authorize(context.Context, AuthorizationRequest)
(AuthorizationDecision, error)` is implemented by an explicitly trusted adapter.
The application authenticates the subject, tenant and scope and verifies external
authority. Goquent cannot authenticate a dishonest application assertion or repair
a malicious/misconfigured provider. `AllowUntil` does not contact an authority.
No authentication server, approval workflow, token signature protocol, keyring,
secret storage, provider networking or permission distribution format is added.

Each successful constructor creates a distinct private registration instance,
even for the same provider and subject. Value copies, Settings, DB Clone,
WithOptions and inherited transactions retain the registration. Replacing it
creates a new setting without changing existing queries or transactions.
Separately created DB settings have no global provider fallback. Deliberately
sharing a registration shares its adapter, not an operation permission. Provider
internals are not deep-copied: the application owns synchronization and must
re-register when its authority configuration changes. Private instance identity
is neither a public digest nor a physical DB identity.

## Request and response boundary

There is no public request constructor, new authorization terminal, serialized
permission consumer or conversion from QueryPlan/source JSON/public views.
ExecuteValidated constructs the request from the rebuilt private planner seal.
It reuses private snapshot equality and binding context comparison rather than
a caller-provided hash. Existing canonical encodings, HMAC domains and source
wire versions remain unchanged. Registration identity is compared separately
from bindingSettings bytes: providers are never canonicalized or publicly hashed.

The trusted adapter can explicitly read Operation (select/count/insert/batch/
update/delete), Subject, Scope, Target, Dialect, Tenant (detached value plus
presence) and ExpiresAt (the upper bound). These are sensitive application inputs,
not logging fields. Missing tenant presence is rejected; explicit nil presence
is distinct and does not itself prove a tenant policy. Existing value-copy and
canonical budgets apply. SQL, parameters, keys, snapshot/IDs and arbitrary
metadata are not available through accessors or public views.

The descriptor supports decisions based on scope and authenticated application
context. Request correspondence does **not** prove that an external service
independently inspected SQL or every value. A workflow requiring statement/value
approval beyond these inputs needs a separately designed adapter contract; do not
silently substitute a family-wide permission.

For example, two updates with the same subject, scope, tenant and operation
family but different assigned values can have identical adapter descriptors.
Their private snapshots and request instances remain distinct: a decision for
the first update cannot validate the second. This correspondence check does not
prove that an external service reviewed the business meaning of either value.

`request.AllowUntil(expiry)` and `request.Deny()` create opaque candidates tied
to that exact private request. Reconstructing an identical request creates a
different instance. An allow response from another request is rejected, including
another query/DB with shared configuration. A decision is never an unbound bool.
The validator compares registration, subject through that immutable registration,
owner/executor/context, binding scope/generation/key/target/dialect, family,
tenant presence/value, full private snapshot, settings, operation and inspection.
Changes to original mutable inputs or detached getters cannot rewrite the request.
The private constructor is not a replacement for structural inspection.

## Expiry, errors and lifecycle

Expiry is mandatory and must be strictly after the validator's current instant.
Equality is expired; no grace is provided. The caller of the private boundary
must supply the minimum binding/approval bound. Request construction further
caps it by effective and stored context deadlines. A decision beyond the known
request/current/context bound is rejected, not silently extended. Nil context
has no deadline but never makes decision expiry optional. The independent tests
inject local instants; no global clock hook or existing binding-clock change is
introduced.

| Fixed error | Independent validator meaning |
| --- | --- |
| ErrAuthorizationMissing | Missing registration or request |
| ErrAuthorizationDenied | Same-request deny |
| ErrAuthorizationProvider | Non-nil provider error, including error plus allow |
| ErrAuthorizationInvalid | Zero/unknown candidate, unsupported/missing input, absent expiry or excessive expiry |
| ErrAuthorizationMismatch | Request instance or current private correspondence differs |
| ErrAuthorizationExpired | Request/current/decision expiry reached |
| ErrAuthorizationCanceled | Effective or stored context reports cancellation |

Provider errors are discarded without invoking Error, Format, Is, As or Unwrap.
No source error is retained or exposed. Context cancellation is checked separately,
not inferred from provider text. Context/input failures can precede response
classification. Validation is non-consuming and can be repeated: success is not
execution, consumption, a transaction commit or external exactly-once evidence.
There is no second ledger or public consume API in PR1.

## Execution, migration and compatibility

Supply a trusted `BindingCurrent` and explicit finite expiry to the existing
Validate method, then supply current trusted state again to ExecuteValidated:

```go
current := query.BindingCurrent{Settings: db.Settings(), BindingContext: binding}
q := db.Table("orders").Where("tenant_id", tenantID).
    RequireApproval("record the intended maintenance operation")
handle, diagnostic, err := q.ValidateUpdate(ctx, current, expiresAt, patch)
if err != nil {
    return err
}
_ = diagnostic // Display only; blocked authorization-pending output is expected.
_, err = q.ExecuteValidatedUpdate(ctx, current, handle, patch)
return err
```

The application constructs `binding` with `NewBindingContext` using its current
key, scope, generation, target and dialect, and registers its authenticated
subject/provider on `db` before creating the Query. No key/scope is inferred or
automatically generated. Settings and BindingCurrent are value snapshots: the
application must supply fresh assertions; reassignment of an unrelated variable
cannot notify an already running call of an external configuration change.

Validate uses a local private planner copy, never a shared bypass switch. It may
produce a handle only when structural inspection succeeds. Pending high-risk
diagnostics stay blocked and lose private execution evidence before returning.
Changing diagnostic SQL, args, risk, Approval or JSON cannot change the handle.
Ordinary Plan retains its Strict refusal. Both built-in and configured risk are
checked; suppression or a configured downgrade cannot remove required authority
or rescue a built-in structural block.

ExecuteValidated consumes the shared handle by CAS **before** validating current
inputs. Copies, concurrent losers and same-handle provider reentry cannot attempt
it again. Initial mismatch/cancellation/expiry/structural rejection calls neither
provider nor executor. A high-risk supported attempt calls the provider at most
once, after current binding checks. Low-risk attempts call it zero times. The
provider result is checked against a new reconstruction after return and again
at private execution and immediately before executor dispatch. Callback changes
to the Query, input values, settings or contexts fail closed; there is no automatic
retry or second authorization request. Missing/deny/error/unknown/mismatch/expiry
and database failure all leave the handle consumed. The independent PR1 validator
remains non-consuming; its repeatability is not repeatable execution authority.

The request ceiling is the minimum explicit binding expiry, relevant Approval
expiry and effective/stored context deadlines. Decision expiry is mandatory;
equality is expired and excessive expiry is rejected rather than rounded down.
Final dispatch rechecks current material, cancellation and the shortest deadline.
No guarantee spans the interval after handing control to the executor, transaction
commit, external revocation or an uncooperative provider. Providers own their
synchronization and side effects. Unsynchronized mutation of a Query or input
from another goroutine is unsupported; concurrent attempts with immutable inputs
and a concurrency-safe provider share the handle's one-attempt gate.

Reasons and legacy Approval record intent and retain their existing reason and
expiry requirements. PII AccessReason is a purpose statement. None is permission.
Neither suppression, displayed risk, manifest Fresh nor public/source JSON is
permission. Compatibility without a provider retains its existing reason path.
Registering a provider intentionally changes Compatibility: supported high-risk
validated execution requires that provider, and ordinary high-risk execution
refuses rather than falling back to a reason. Migrate such calls to the six
validated families with explicit binding inputs, or retain the documented
unregistered Compatibility behavior without claiming external authorization.

| Entry | External authorization integration and limits |
| --- | --- |
| ValidateSelect/Count/Insert/InsertBatch/Update/Delete | Provider0/DB0; structural rejection returns no handle; pending high risk returns a single-attempt candidate and blocked diagnostic |
| Corresponding ExecuteValidated methods | Connected; high risk requires provider, low risk provider0; structural rejection cannot be overridden |
| Ordinary Query CRUD, batch, insert variants | High risk: Strict always refuses; Compatibility with provider refuses/provider0; unregistered Compatibility retains reason path |
| Generic/scoped CRUD and RETURNING | Same ordinary gate; no new authorized RETURNING family; existing dialect restrictions remain |
| Nested collections, idempotent commands, hooks | Their existing statement gates and preflights remain; no composite external authorization; opaque Strict recipes remain refused; Compatibility can have earlier effects before a later statement refusal |
| Raw DB methods and raw recipes | No authorization support; Strict refuses unsupported Raw; provider-registered high risk refuses even with internal/user reason |
| Custom Executor, WrapExecutor, external Tx | Same gate when using supported six families; ownership, scan and transaction responsibilities unchanged; inherited registration does not allow another Query's handle |
| AtomicWrite/OptimisticVersion starting an owned Tx | High-risk authorization refuses before provider/Begin: the second executor/storage-probe boundary is not authorized by the original request; existing low-risk behavior and caller-managed Tx paths retain their limits |
| Direct sql.DB/sql.Tx/Executor, SQLDB | Outside interception; never presented as Goquent-authorized |
| CLI/MCP and migration | CLI/MCP remain read-only; migration apply is a separate human-controlled boundary, with no new DDL permission |

Tenant violations, unknown schema, structural Blocked, unsupported expressions
and unparsed Raw are never rescued by an allow. In provider-registered
Compatibility, validated binding still requires the existing trusted schema,
tenant and private identity material; ordinary compatibility heuristics do not
establish that identity.

Registration/request/decision reject JSON and format as fixed opaque text for
ordinary fmt logging, including nested exported fields. Method-level decode
clears the visited receiver. As with BindingContext, encoding/json syntax errors
before method invocation can leave an old receiver unchanged; decode into a fresh
zero receiver and discard errors. Aliases are not globally invalidated, and nil
pointers retain standard JSON null behavior. This is not protection against
reflection, unsafe memory inspection or explicit sensitive accessor logging.
No PlanView, CLI, JSON/SARIF display or MCP fields are added.

Physical database identity, external policy/schema freshness and stale application
assertions remain outside the existing private binding proof. Tests distinguish
local trusted adapter fixtures, spies/sqlmock, and native MySQL/PostgreSQL
execution. They do not establish a production external authorization service,
statement/value-specific external approval, exactly-once execution or commit
success. Existing review risk/precision/unknown findings and unverified native
performance/role/operator/cast/network-fault cases are not repaired by this change.
