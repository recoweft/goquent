# Plan versions and private identity

GQ-AI-05 PR1 (1/2), [Issue #70](https://github.com/recoweft/goquent/issues/70).
This implements diagnostic/input versions and internal correspondence primitives.
PR2 adds [validated binding](validated-plan-binding.md), explicit current-input
comparison and detached key/target supply for six Query operations. No public
QueryPlan/JSON execution, persistent handle, identity getter or automatic key
management is introduced.

## Envelope inventory and migration

| Representation / consumers | Current writer | Reader |
| --- | --- | --- |
| query.QueryPlan; ToJSON, review inputs, CLI/MCP output | integer `version: 1` | ordinary json.Unmarshal, including nested plans |
| review.ReviewReport; JSON formatter / CLI | integer `version: 1` | ordinary json.Unmarshal |
| query.TablePolicy; registry, PolicySet, CLI policy input | integer `version: 1` | ordinary json.Unmarshal and Go input validation |
| operation.OperationSpec; CLI/MCP, schema, Validate/Compile | integer `version: 1` | ordinary json.Unmarshal, original MVP allowed-field checks |
| query.PolicySet | no independent JSON envelope; private collection | NewPolicySet validates each TablePolicy; normalized snapshot uses the TablePolicy contract |
| manifest.Manifest / embedded manifest.Policy | existing **string** `version: "1"` on Manifest | existing manifest.Load/Validate; embedded Policy has no new version |

Each new integer envelope accepts missing or integer 0 as legacy format, and
integer 1 as the current format. Unknown/negative integers, null, strings, bools,
fractional/exponent spellings (including 1.0 and 1e0), overflow, duplicate version
keys, and case aliases are errors. A case alias alone is rejected too. Other
unknown fields retain their existing per-type behavior: OperationSpec still
rejects non-MVP fields. Its JSON Schema describes the value domain; JSON Schema
cannot establish lexical spelling or detect duplicate keys after a parser has
collapsed them. The Go reader enforces those additional rules.

A zero Go Version or legacy decode writes version 1. Explicit unknown Go versions
fail marshaling instead of being relabeled as current. OperationSpec.Validate /
Compile and TablePolicy's NewPolicySet / RegisterTablePolicy / manifest.Generate
also reject unknown Go versions. `errors.Is(err, query.ErrJSONVersion)` identifies
the version error, including json.MarshalerError wrapping. The operation and
review packages expose the same sentinel as ErrJSONVersion and current constant
as JSONVersion. Ordinary syntax/type errors retain encoding/json identities;
this does not normalize every error into a version error.

Version 0 → writer 1 → reader 1 is a **format migration**, never provenance or
permission. Known version, low risk, blocked=false, approval, precise and public
source/table/SQL/params are still diagnostic data. JSON cannot restore private
execution, tenant/write evidence or identity. EnsurePlanExecutable remains its
existing diagnostic/risk helper, not an artifact authenticator.

The manifest format and its existing schema/policy fingerprint algorithm are
unchanged. Old manifest files and legacy policy inputs remain usable. New
TablePolicy version fields do not enter manifest.Policy's fingerprint layout.
No existing examples must be rewritten merely to import them; newly generated
QueryPlan/ReviewReport/TablePolicy/OperationSpec JSON includes version 1.

## Decode receivers and standard-library limits

When QueryPlan.UnmarshalJSON is actually called, it first clears that receiver's
private execution (including its identity snapshot), tenantEvidence, writeEvidence,
conditionSource and generatedConditions. It decodes into a fresh temporary value.
On success it replaces all public fields, including absent fields, and retains
no private evidence. On method-level failure (unknown/ambiguous version, field
type mismatch, or a directly supplied invalid document), public fields remain
atomic and unchanged, while private evidence remains cleared.

`json.Unmarshal` checks syntax **before** calling the type method. Empty/truncated
JSON, trailing garbage and multiple documents rejected in that precheck leave the
entire old receiver unchanged, including its old evidence. Decoder.Decode also
has pre-method syntax/IO failures. This is not evidence for the failed input; it
is the original plan with its original owner/context/expiry/one-use/gate. Do not
use that old receiver as a successful decode result or error fallback.

Invalidation is local to the receiver actually visited. It does not revoke copies
already holding a shared seal, promise concurrent mutation safety, or cancel an
executor. Decoding `null` into a pointer-to-pointer may set it to nil without
calling the method; aliases remain unchanged. Marshaling a nil pointer retains
normal JSON null. An explicit `version: null` is a version error.

Always read external JSON into a new zero receiver and discard it on **any**
error. Do not decode external input into an existing executable plan:

```go
var diagnostic query.QueryPlan
if err := json.Unmarshal(input, &diagnostic); err != nil {
    return err // discard diagnostic; never fall back to an earlier plan
}
// Display diagnostics only. This is not an execution handle.
```

For a single document, Decoder users must also require EOF after the first
successful Decode; trailing documents/garbage are errors, even if the first
value decoded successfully. CLI values readers and internal precision readers
perform this check. MCP transport remains a message stream; each request and
OperationSpec string is one complete document. CLI readers use fresh receivers;
Review rejects version-invalid JSON recognized by its operation/sql envelope,
while continuing to ignore unrelated JSON files. MCP validates raw nested OperationSpec versions before generic maps can erase
duplicate declarations. No serialized execution entry is added.

## Numeric compatibility

`any` numbers decoded by the new envelopes, CLI OperationSpec values and MCP
operation arguments now use **json.Number**, not float64. Params, numeric
metadata, filter values and IN lists retain lexemes and order. This is an
intentional dynamic-type compatibility change: replace float64 assertions with
checked json.Number.Int64 (when signed integer range is intended), or another
explicit exact policy. Avoid Float64 before identity or SQL parameter decisions.
Native Go Options.Values and typed struct fields retain their declared types.

JSON numbers do not prove the native signedness/width of an earlier caller.
OperationSpec passes preserved numbers into its read-only plan parameters; it
neither converts them to strings nor introduces a DB execution endpoint. Decimal
and exponent lexemes remain readable diagnostics, but the internal identity
domain supports only integer JSON lexemes. Unsupported lexical forms do not
silently become native integers. Strict tenant comparison's existing signed SQL
integer rules remain unchanged; json.Number is not a new trusted native type.
QueryPlan's cycle/depth/output-budget guard remains in place.

## Private correspondence source and availability

The existing planner/finalizer seals SQL, detached ordered args and structural
inspection once. At that boundary PR1 captures canonical bytes from the same
private record and the actual owner's settings/dialect, application-supplied
tenant, target label, policy, schema and configuration. Supported snapshots require
successful **private** conditional tenant evidence and the conservative generated
structure domain below. Public field edits, display masking, JSON decode, warning
order and approval/display clocks never flow back into this capture. Private
identity is not computed from a public diagnostic submitted by a caller.

The initial supported domain uses MySQL/PostgreSQL built-in dialects, generated
condition trees and literal quoted identifiers, generated placeholders and known
SQL tokens. It preserves SQL/condition/column/projection/conflict/order/argument
order. LIMIT/OFFSET magnitudes are omitted only from shape, remaining in exact
execution SQL. Raw/opaque trees, raw projections/order, unknown tokens, literals,
comments, absent application context, invalid private evidence, unsupported values
or budget excess make identity **unavailable**, not an empty successful identity.
The lexer is a conservative check of a private generated statement, not a general
SQL parser, theorem prover or a way to certify arbitrary sanitized SQL.

Owner policies are ordered by table; maps (including risk rules) sort UTF-8 keys.
Other ordered slices remain ordered, including supplied schema columns and
constraint columns. Semantically equivalent reordered SQL is not promised the
same identity. Source labels, CreatedAt/checkedAt, diagnostic warning order and
masked views are not semantic inputs. Approval expiry remains a separate private
lifecycle condition; a stable identity does not extend a plan's validity.

Compatibility CRUD still works without identity keys/context. Existing Strict
refusals and gates remain authoritative. Capturing an identity does not enable
opaque nested Scope/ID callbacks, hooks, idempotent recipes or unsupported Raw.
No new DB access, EXPLAIN, transaction ownership or second SQL generation occurs.

## Canonical and cryptographic contract (internal version 1)

Each canonical record is `tag + decimal UTF-8 payload byte length + ':' + payload`.
Lists concatenate records in order; maps alternate sorted string key and value
records. No whitespace or separator is added. Supported tags:

- `null`, `bool` (true/false), `str` (valid UTF-8).
- `i8/i16/i32/i64`, `u8/u16/u32/u64`: exact decimal integers. Go int/uint use
  `int32/int64` or `uint32/uint64` according to platform width, distinct from i/u.
- `f32/f64`: finite IEEE bits in lowercase hexadecimal, no leading padding;
  positive and negative zero differ. NaN/Inf are unavailable.
- `jsonint`: exact valid JSON integer lexeme, including -0; no decimal/exponent,
  whitespace or leading-zero coercion. It is distinct from every native type.
- `bytes`: padded standard base64; `nilbytes` distinguishes nil from empty.
- `list/nillist` for []any; `map/nilmap` for map[string]any. Nil containers are
  distinct from null and empty containers.

Other named/custom types, pointers, time values and arbitrary containers are
unavailable. Canonicalization never calls Valuer, Stringer, Marshaler or application
callbacks. Limits are depth 64, 65,536 slots and 8 MiB per canonical output;
byte payloads have a conservative 4 MiB input cap. These are identity limits,
not an expansion of Strict's narrower supported SQL value domain.

The fixture specifies the exact records and digest recipe. Shape is SHA-256 of
canonical `["goquent/shape", i64(1), dialect, operation, value-free shape]`.
It contains no SQL literals, tenant, values, policy secrets or metadata. It is
**private**, and equality is never equal permission. Execution material is
canonical `[dialect, target, operation, exact SQL, shape canonical bytes,
ordered args, tenantPresent, typed tenant]`. Policy/schema/config each use a
separate `goquent/<family>` domain and i64 version followed by canonical fields.
Their fingerprints are private HMAC-SHA256, not public unhashed-value digests.

Execution ID is HMAC-SHA256 of canonical `["goquent/execution", i64(1), scope,
generation, execution material bytes, policy MAC, schema MAC, config MAC]`.
The explicitly supplied key must contain at least 32 bytes; nonempty scope and
rotation generation are mandatory and independently bound. The computation
copies key bytes; callers must not concurrently mutate its input. PR1 neither
generates nor persists keys. There is no production key in the repo, fixture or
metadata; fixture keys are conspicuously fictional test-only material.

Same supported input/key/scope/generation/version gives stable results across
processes with the same typed domain. Rotating key, scope or generation changes
execution identity; it does not change value-free shape. Missing/short key or
missing scope/generation is unavailable. PR1 provides no key distribution,
persistence, keyring lookup or operational rotation protocol. PR2 must reject
unavailable/mismatched bindings and require fresh trusted reconstruction after
rotation; a digest must never serve as a bearer permit or signature authorization.
On a suspected collision, refuse/reconstruct and compare the full private material;
a matching digest alone cannot certify a plan. No public IDs or key/digest errors
are added. Existing public Params redaction remains GQ-AI-06 work.

## Deployment comparison versus execution binding

For a deployment input comparison, run:

```sh
go run ./cmd/goquent manifest verify \
  --manifest examples/ai-safe-orm/goquent.manifest.json \
  --schema examples/ai-safe-orm/schema.json \
  --policy examples/ai-safe-orm/policies.json --format json
```

Matching supplied files is not live schema freshness. Missing generated-code or
database fingerprints remain skipped, even when aggregate fresh=true. A supplied
target/connection label cannot attest physical Executor identity, honest driver
behavior, authenticated tenant or truth of asserted schema/unique constraints.

PR2 connects explicit key/target supply, immutable validated reconstruction,
current policy/schema/config/tenant/dialect/target comparison, expiry and final
private SQL/args dispatch. Neither an old manifest comparison at startup nor a
stored plan JSON supplies those runtime inputs. Live DB changes are not
completely detected by this contract. Existing owner/context/one-use gates remain;
PR1 does not claim all Issue #70 ACs or GQ-AI-04/06 are complete.
