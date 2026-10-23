# Scoped external authorization contracts

GQ-AI-13 PR1(1/2), [Issue #103](https://github.com/recoweft/goquent/issues/103).
This PR defines application registration, opaque adapter request/decision types,
and an independent private validator. **Production execution is not connected.**
No Plan, compile, review, Validate or execution path invokes the provider in PR1.
Strict still refuses high-risk operations, even with a registered allow provider.
PR2 is a separate change after this PR merges; this is not whole-Issue acceptance.

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

There is no public request constructor, authorization terminal, permission
consumer or conversion from QueryPlan/source JSON/public views. The private
request constructor and validator are tested independently, with no production
caller in PR1. They reuse existing private snapshot equality and binding context
comparison rather than a caller-provided hash. Existing canonical encodings,
HMAC domains, source wire versions and bindingSettings remain unchanged.

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

PR2 must add authorization registration/current-subject correspondence to existing
private binding checks without canonicalizing provider pointers; preserve
ValidatedPlan's CAS-before-validation, shared-copy single-attempt behavior,
concurrent-loser refusal and consumption on mismatch, cancellation, expiry and
DB failure. It must recheck final private material and deadlines immediately
before dispatch. Those execution properties are **not implemented for the new
contract here**. External revocation and forced interruption of an uncooperative
provider are not guaranteed.

## Compatibility, output and remaining integration

Reasons and legacy Approval record intent; PII AccessReason describes purpose.
Suppression, low displayed risk, manifest Fresh and public/source JSON do not
become permission. Compatibility retains its reason-based execution behavior.
Strict's `external_high_risk_permit_missing` gate is unchanged. A future permit
must never override structural blocked findings, tenant failures, unparsed Raw,
unsupported operations or invalid/missing schema.

Registration/request/decision reject JSON and format as fixed opaque text for
ordinary fmt logging, including nested exported fields. Method-level decode
clears the visited receiver. As with BindingContext, encoding/json syntax errors
before method invocation can leave an old receiver unchanged; decode into a fresh
zero receiver and discard errors. Aliases are not globally invalidated, and nil
pointers retain standard JSON null behavior. This is not protection against
reflection, unsafe memory inspection or explicit sensitive accessor logging.
No PlanView, CLI, JSON/SARIF display or MCP fields are added.

PR2 must enumerate ordinary CRUD, validated families, generic/RETURNING, Raw,
custom Executors, external Tx and callback recipes, including refusals and paths
outside interception. Existing custom Executor/Tx ownership and callback limits
remain unchanged. Direct sql.DB/sql.Tx/Executor calls are outside the gate.
Plans/compile/review remain non-executing and must not implicitly call providers.
CLI/MCP stay read-only; migration apply remains a separate human-controlled
boundary. No DDL permission or migration safety guarantee follows from this API.
Physical database identity, external policy/schema freshness and stale application
assertions remain outside the existing private binding proof.
