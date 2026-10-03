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

The adopted same-revision decision is A (questions
`d4e3ea76-c310-4ef0-8805-fb5968770593` and
`2335c128-ddcf-4f06-adb6-f988cd5cd0e6`). No new argument types are accepted or
converted by this work; downstream drivers/Executors still decide validity.

| Value category | Snapshot/copy/Plan behavior |
| --- | --- |
| nil, bool, string, built-in signed/unsigned integers (except uintptr), float32/64, time.Time | Value copy; original Go type and SQL NULL behavior preserved |
| Exact built-in scalar slices (including byte slice and time.Time slice) | New backing storage, nil versus empty preserved |
| `[]any`, `map[string]any` | Recursive copy to 64 container levels; the entire dependent value is unverified if any descendant cannot be isolated |
| Named custom types, pointers (including typed nil), arbitrary structs, Valuer/Marshaler, unsupported containers | Passed through unchanged; `isolation: unverified`, reason `unsupported_or_recursive_value`; external state can still change |
| Cyclic or deeper containers | Bounded copying stops at an unverified retained reference; no recursive reflection, JSON round-trip, normalization or assertion of immutability |

Unverified payloads are omitted from condition-value JSON to avoid invoking user
serialization code. This does not redact or change the existing Plan.Params
serialization contract. Values that the existing SQL emitter transforms (for
example internal JSON formatting) are recorded as its actual returned argument;
that expression is still opaque. Unknown values must not support key-binding or
equality proofs, even if a current printed value looks equal.

Builder-owned clause nodes, slices/maps, named Raw bindings, subquery/union state,
HAVING values, join maps and supported values are copied. Attached table key/index
metadata is also detached. Independent snapshots/Plans do not share their mutable
supported values. Valuer evaluation timing, its error, and the original custom
Executor argument type remain unchanged.

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
