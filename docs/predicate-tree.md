# Predicate trees and builder snapshots (GQ-AI-02/PR1)

Refs [Issue #57](https://github.com/recoweft/goquent/issues/57). This is the
representation foundation, PR1 of 2. It does **not** implement semantic write
cardinality, a Strict API, trusted tenant binding, or executable-plan validation.
The existing risk engine still uses column-presence heuristics. In particular,
`id`, `RiskLow`, `precise`, and generated inspection metadata are not single-row
proofs or business authorization. PR2 owns key/OR/nullability proofs and improved
diagnostics. Neither this PR nor its tests close the whole issue.

## Generation and inspection

The internal WHERE clause storage now retains nested groups. It is compiled into
an n-ary AND/OR tree with SQL precedence (AND before OR), explicit group and NOT
nodes, and existing dialect leaf payloads. All three WHERE renderers traverse this
same tree. The traversal emits SQL, ordered arguments, and `predicate.Node`
inspection together; it does not parse rendered SQL or re-evaluate values to build
inspection metadata. SELECT, UPDATE and DELETE plans consume `BuildSnapshot` from
the builder which actually emitted their SQL, including the SET/projection/join
argument offset. HAVING uses the same tree compiler and rendering traversal.

`QueryPlan.WhereTree`, `HavingTree` and `Unverified` are additive optional JSON
fields. Column references retain their original qualification (including aliases),
operators retain their original spelling, column comparisons have `value_column`,
and column BETWEEN has `bound_columns`. Groups delimit NOT targets. Leaf SQL is
preserved for dialect-specific review. Parameters are zero-based indexes into the
whole statement's `Params`, not into a WHERE-only list; repeated named bindings
have separate positions in SQL order. UNION's current branch is offset by prior
branches, but other branches are explicitly outside whole-plan inspection.

`correspondence: generated` describes supported structure emitted during that
build, **not** trusted evidence. Opaque expressions, empty IN and unisolated values
use `unverified`; this propagates to ancestors. Positions still describe the
arguments returned by the emitter, even when arbitrary SQL syntax or value
contents cannot be validated. `Unverified` also identifies joins, UNION branches
and selected raw expressions outside the condition inspector's coverage.
Serialization cannot reconstruct the private generated source used by the runtime
compatibility inspectors. Imported JSON and static/manual plans keep their legacy
input path; they are not validated executable artifacts.

Internal `Snapshot()` is a pure structural capture: it never runs a SQL formatter,
Valuer or JSON marshaler. Its condition correspondence is unverified and it has no
parameter indexes or rendered SQL. Named Raw bindings are retained by name there.
`BuildSnapshot()` returns the rendered trace and parameters from a single build.
Snapshot errors are exposed by `QuerySnapshot.Error`; public Build/Plan paths
return errors normally. Both operations remain DB-free.

`PredicateRef` and the existing pretty display remain available. They are derived
from the frozen clauses, not accepted as input by SQL generation. Runtime
column-presence consumers project the private generated tree instead of reading
an independently edited display slice. This is compatibility plumbing, not a new
scope algorithm or protection against callers editing public SQL/Params/trees.
After changing a query, build and inspect a new Plan. Snapshots do not make public
plans immutable or builders safe for concurrent mutation.

## Entry-point inventory and boundaries

| Entry/path | Representation and scope |
| --- | --- |
| `Where`, `OrWhere`, map/simple conditions, `WhereAny`/`WhereAll` | Comparison leaves and SQL AND/OR precedence; arbitrary unsupported operators/arity stay unverified |
| `WhereGroup`, `OrWhereGroup`, `WhereNot`, `OrWhereNot` | Nested group and NOT nodes; callbacks use isolated clause storage |
| `WhereIn`/`WhereNotIn` and OR variants | Ordered values, including nil, singleton and multi-element lists; no cardinality inference |
| `WhereNull`/`WhereNotNull` and OR variants | NULL test leaves, distinct from binding nil to `=`; no rewriting of SQL three-valued logic |
| `WhereColumn`, `WhereColumns` and OR variants | Both qualified references retained; no value equality/key proof |
| BETWEEN/value and column variants | Value parameter order or both bound columns retained |
| Date/time/day/month/year variants | Existing dialect function generation retained, opaque for semantic inspection |
| Raw/SafeRaw/RawNoArgs and OR variants | Original SQL and emitter binding order retained; opaque, no general SQL parser |
| WHERE subquery, IN subquery, EXISTS/NOT EXISTS | Whole leaf opaque; original nested SQL and returned argument order retained, recursive nesting bounded |
| `WhereJSONText`, `WhereJSONHasKey`/`WhereJSONNotHasKey`, text-search helpers | Existing public helpers lower to Raw or grouped predicates; Raw remains opaque |
| Internal full-text/JSON helpers | Existing renderer and errors retained, opaque; snapshot never invokes user marshalers; Build invokes only the existing formatter path |
| Cursor/scopes/soft-delete predicates | Their actual lower-level WHERE calls pass through the same tree; expression-based Raw remains opaque |
| `Having`, `OrHaving`, Raw HAVING | Tree and argument offsets emitted with SQL; legacy omission of empty column/operator/string-value clauses retained |
| JOIN/ON, lateral/derived tables, UNION | Existing SQL and copy storage retained; joined references/aliases remain available; recursive semantic inspection of those clauses is unverified |
| SELECT expressions, ORDER BY expressions, raw plans | Outside predicate semantics; no new raw SQL guarantee |
| Query write plans | WHERE from the actual write builder; SELECT-only projection/HAVING/UNION do not become write constraints |
| Generic CRUD, RETURNING additions, OperationSpec/static reconstruction | Existing paths and limitations remain; this PR does not create universal plan/policy mediation |

Both old dialect leaf renderers and the base write renderer remain in use. Thus
existing unsupported write-specific function/JSON/full-text behavior is not
promoted to supported semantics. A tree records the SQL path used, rather than
assuming SELECT and write emitters support the same expression set.

## Value isolation contract

The original adopted value-isolation decision is A (questions
`d4e3ea76-c310-4ef0-8805-fb5968770593` and
`2335c128-ddcf-4f06-adb6-f988cd5cd0e6`). No new argument types are accepted or
converted by this work; downstream drivers/Executors still decide validity.

| Value category | Snapshot/copy/Plan behavior |
| --- | --- |
| nil, bool, string, built-in signed/unsigned integers (except uintptr), float32/64, time.Time | Value copy; original Go type and SQL NULL behavior preserved |
| Exact built-in scalar slices (including byte slice and time.Time slice) | New backing storage, nil versus empty preserved |
| `[]any`, `map[string]any` | Operation-local memoized copy, limited to 64 value edges and the budgets below; the entire dependent value is unverified if any descendant cannot be isolated |
| Named custom types, pointers (including typed nil), arbitrary structs, Valuer/Marshaler, unsupported containers | Passed through unchanged; `isolation: unverified`, reason `unsupported_recursive_deep_or_oversized_value`; external state can still change |
| Cyclic, deeper or over-budget containers | Bounded copying stops at an unverified retained reference; no recursive reflection, JSON round-trip, normalization or assertion of immutability |

Unverified payloads are omitted from condition-value JSON to avoid invoking user
serialization code. Ordinary, in-budget Plan.Params serialization is unchanged. Revision 3 adopts
the output-limit exception below; this is not redaction. Values that the existing SQL emitter transforms (for
example internal JSON formatting) are recorded as its actual returned argument;
that expression is still opaque. Unknown values must not support key-binding or
equality proofs, even if a current printed value looks equal.

Builder-owned clause nodes, slices/maps, named Raw bindings, subquery/union state,
HAVING values, join maps and supported values are copied. Attached table key/index
metadata is also detached. Independent snapshots/Plans do not share their mutable
supported values. Valuer evaluation timing, its error, and the original custom
Executor argument type remain unchanged.

## Revision 3: bounded copy and output

Revision 3 of work `43118939-d7f2-420a-863f-873803cbeda3` adopts output option A
from consultation `a3530f66-b133-4885-bd48-3f07ac6ba805`. The earlier validation
record describes the original implementation, not evidence for this correction.

Copying uses an active-path marker and a completed-copy memo. A back edge keeps
the original reference and marks its dependent value unverified immediately.
Completed acyclic children are reused, including their isolation result and
height (a deeper occurrence must still pass the depth check). Identical map
identities and slice views (type, starting address, length) share one copy within
an operation. Overlapping but nonidentical slices are independent copies; capacity
and arbitrary alias topology are not guaranteed. Source objects remain reachable
while the memo lives, preventing GC/address reuse between arguments. Each new
copy, snapshot or Plan owns fresh memo state; there is no global cache.

Before allocation, a copier reserves at most **65,536 slots** and **8 MiB of
payload allowance**. A recursive container costs one slot plus its length and
16 bytes per slice element or 64 bytes per map entry; a scalar slice costs one
slot and length times its element size. Empty/nil/scalar values keep their types;
immutable strings are not duplicated. Overflow is checked before multiplication
or allocation. These conservative allowances bound newly copied payload and
traversal, not exact Go heap accounting. The limits allow ordinary SQL parameters
and multi-megabyte binary values while stopping adversarial width as well as
branching recursion. The root has depth zero and values at depth 64 are accepted;
deeper values retain their references and become unverified.

A copier is shared across all values in a query-clone traversal (including its
subqueries/joins), a predicate rendering/inspection pass, a Node-copy traversal,
or an argument-slice copy. Limits are **per copy pass**, not reset for each
argument and not a process-wide or entire-statement heap quota. Existing SQL and
clause construction, separate WHERE/HAVING passes, UNION builds and the finite
successive snapshot/Plan copies still have their own storage. Their cost can grow
with the explicitly supplied SQL/clause count. Unsupported or budget-exhausted
payloads remain opaque references; no condition or execution argument is dropped.
`valuecopy.Node` also memoizes shared condition nodes and propagates value-copy
failure to ancestor correspondence. Normal builder trees are depth validated;
this does not add public-plan mutation validation.

Output has a **separate** preflight: **65,536 expanded nodes**, **8 MiB of estimated
output cost**, and **256 traversal edges**. The larger depth includes library
struct/pointer/slice wrappers around the builder's 64 condition-group levels.
Each visited item costs 32 estimated bytes, plus six times a string's byte length
for worst-case JSON escaping. Map keys count as items. Library field names and
caller-selected indentation are not exact byte accounting; 8 MiB is an estimate,
not a promise about final JSON bytes or runtime heap. Node count and depth also
bound standard formatting overhead. Memoized subtree costs are added for **every**
reference, so a compact DAG may copy successfully but fail output preflight.
The preflight itself visits each distinct supported subtree once and stops before
large serialization allocations. It never calls Valuer, Marshaler, Stringer,
Formatter or any other user method.

`QueryPlan.String` returns an explicit reason-bearing omitted/unverified marker
on failure; no payload is formatted to create that marker. `ToJSON`,
`json.Marshal(plan)`, value-form `json.Marshal(*plan)` and `json.MarshalIndent`
return an error, never a successful null/empty replacement. `errors.As` to
`*query.OutputError` distinguishes preflight errors; `errors.Is` recognizes
`query.ErrOutputCycle`, `ErrOutputDepth` or `ErrOutputBudget`, including through
`json.MarshalerError`. A nil Plan still serializes as `null` and String returns
`<nil query plan>`. Normal JSON fields, indentation, nil/empty distinctions and
String output remain unchanged; no reference-ID encoding is introduced.

The preflight covers all exported library-owned Plan and predicate fields,
including WHERE/HAVING values, named values, Metadata, both warning lists and
Evidence. Standalone predicate Node/Value JSON also applies the guard. Unnamed
slices/arrays and string-keyed maps are traversed; named user types, user structs,
non-string-keyed maps and arbitrary user pointers are opaque. Their existing
formatting/marshaling methods still run at the normal output stage. Their
recursion, allocations and output sizes are **not bounded** by this contract.
Neither arbitrary direct formatting of `Value.Data`/`Params` by caller code nor
other APIs such as SQL interpolation are covered. No whole-Plan/all-user-value
resource guarantee, masking, concurrent-mutation safety or tamper detection is
claimed. After mutation a new output call always performs a fresh check.

For migration, reduce oversized values/metadata or graph expansion before asking
for output, and handle the typed output error. Do not catch it and execute a
truncated condition. Display errors do not reject Plan generation or DB execution;
SQL, Params, custom Executor types and Valuer evaluation/error timing remain on
their existing paths. Unverified values remain unavailable as equality or key
proofs; semantic cardinality remains PR2 work.

## Depth and compatibility changes

`predicate.MaxDepth` is 64; `predicate.ErrDepth` supports `errors.Is`. A nested
group or subquery edge consumes a level. Long flat AND/OR lists use n-ary nodes
and do not consume a level per leaf. Validation runs before recursive copying and
rendering; cyclic query structures also fail. Group construction stops invoking
callbacks beyond the limit and stores the error. Public Build/Plan/PlanUpdate/
PlanDelete return the error, without returning executable truncated SQL. The
limit bounds recursive work and is deliberately much larger than typical DSL
nesting; it is not a database-specific SQL limit.

Intentional behavior changes:

- Nested groups/NOT now preserve the requested brackets. Previously callbacks
  appended into the parent's flat storage and could lose their outer scope. This
  fixes C06–C08/C11 expression construction, not their missing cardinality/tenant
  proofs. Queries which relied on that erroneous SQL must express their intended
  flat or grouped logic explicitly. Single-level fixtures remain unchanged.
- More than 64 nested levels now fail instead of risking unbounded recursion.
  Simplify such queries; do not catch the error and execute without the condition.
- Supported mutable inputs are captured in copies/Plans. Rebuild after changing a
  byte slice or supported container rather than expecting an old Plan to follow it.
- New JSON fields require consumers using strict unknown-field rejection to permit
  the additive fields. Existing fields and the five ai-safe-orm JSON files are not
  removed or regenerated. Use keyed Go struct literals for the extended Plan type.
- Imported/manual plans retain legacy metadata behavior. Runtime-generated
  PredicateRef edits no longer replace the generated inspector source. This is
  not an attestation mechanism; general plan mutation verification remains out of
  scope. No diagnostic code, severity, nullable-unique rule or row guarantee is added.

Empty IN retains the legacy SQL (including invalid empty-list forms); it is marked
unverified, not replaced with TRUE/FALSE or a single-row assertion. MySQL and
PostgreSQL integration tests both observe rejection. `= nil`, `IS NULL`, and
`NOT IN (..., NULL)` remain different operations.

## Evidence and remaining work

| Evidence | Bounded assertion |
| --- | --- |
| `orm/query/predicate_tree_test.go` | C05–C11/C28 structures, precedence, NOT nesting, bindings/offsets, aliases, Raw/subquery limits, JSON round-trip, generated vs display source, snapshots/copies/UNION, custom values, depth boundary, detached key metadata |
| `orm/internal/valuecopy/value_test.go` | Supported nested values, typed nil, custom type, opaque descendants, cycle bound |
| `orm/internal/querybuilder/internal/common/structs/predicate_test.go` | Subquery depth 63/64/65/1000, cyclic query/condition graphs, 10,000 flat leaves |
| `tests/predicate_tree_test.go` | Actual MySQL/PostgreSQL result sets, NULL logic, singleton/multi/empty IN, self-JOIN aliases, composite key conditions, nested UPDATE/DELETE via external sql.Tx |
| Existing builder/query, contracts, review, manifest, operation, MCP, CLI and integration tests | Common SQL/Params and PredicateRef, diagnostics, JSON load/review, five example hashes, custom Executor/sql.Tx, scanning/bool regression |

The registry retains C05–C11/C28 and its existing current/future determinations;
new references are partial representation evidence, not completion of PR2 safety
requirements. No unsafe verdict has been added as a desired golden result. There
is still no Strict API or trusted tenant-context contract (including C09).

PR2 must implement semantic key/alias/full-composite/nullable-unique constraints,
OR whole-expression cardinality, unknown versus broad outcomes and improved
evidence/precision diagnostics. Raw parsing, all-value immutability, whole-plan
artifact validation, join/subquery theorem proving and other issues are not
claimed by PR1. See the PR's validation record for actual commands, skips,
failures and CI head; test registration alone is not execution evidence.

## Revision 4: argument-container ownership

`valuecopy.Slice` / `Copier.Slice` are for library-owned argument lists, not
arbitrary slice payloads. They always allocate an independent outer array
(preserving nil versus empty). Previously, an outer list of 65,536 or more
arguments exhausted the recursive copy slot budget and was returned unchanged.
SELECT then zeroed that same pooled array in its deferred cleanup, destroying
the returned arguments before the caller received them.

Outer ownership now costs O(argument count) interface slots per list, outside
the recursive payload allowance. There is no new statement-size or whole-process
heap guarantee. This mandatory linear cost is needed to preserve every argument
through cleanup and later builds; increasing the recursive limit would only move
the bug. Each element is copied as a payload root (depth zero), sharing the same
operation's memo and depth-64 / 65,536-slot / 8-MiB payload budgets. Recursive
payload reservations still happen before allocation; no budget is reset per
element. `Copy` of a slice payload still refuses oversized containers, preserving
their original type/reference and unverified inspection state. Library argument
lists are never cached as payload memo entries; even two `Slice` calls on one
Copier own separate outer arrays, while safely copied child DAGs may share that
operation's completed memo. Independent operations keep independent child copies.

The shared helper covers SELECT's pool return, query-clone argument fields
(selected expression values, WHERE/IN values, JSON contains values), and
`newQueryPlan` Params (including raw plans). Snapshot/BuildSnapshot use these
clones and the existing bounded Node/inspection copy; unverified payloads still
propagate to dependent conditions and ancestors. UPDATE, DELETE, INSERT batch
and INSERT SELECT already copy their outer arrays before ZeroInterfaces, and
single-row INSERT constructs its returned values separately. Their cleanup does
not depend on recursive-copy success. No pool cleanup or budget threshold is
removed. Existing error exits that do not return pooled buffers are not an alias
escape and were not refactored here.

Output limits and typed JSON errors are unchanged. A large executable argument
list can retain all values while String omits it and JSON rejects its expansion.
No output marker becomes an execution value, and no Valuer, formatter or
marshaler is evaluated by copying. These ownership guarantees do not imply
cardinality, authorization, arbitrary custom payload immutability, or public
Plan tamper detection. See [revision 4 validation](predicate-tree-revision4-validation.md).
