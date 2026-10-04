# Conditional write scope

GQ-AI-02, [Issue #57](https://github.com/recoweft/goquent/issues/57), PR2 of 2.
Builds on merged [PR #58](https://github.com/recoweft/goquent/pull/58).

`Query.WithWriteKeyContext` supplies explicitly asserted key facts for UPDATE and
DELETE planning. `AnalyzeWriteScope(plan)` checks private builder evidence and
returns a new result; `plan.WriteScope` is a display snapshot produced by the
built-in risk engine. Neither is an authorization decision, a live schema check,
an affected-row count, or proof that a row exists. Plan generation executes no SQL.

## Application assertions and migration

Only trusted application code should call `WithWriteKeyContext`. The caller must
assert that the context describes **this Query's current database/executor and
entire target row set**. `Database` is a nonempty application database identity,
not a server identity independently authenticated by Goquent. `Dialect` must
match the built-in MySQL or PostgreSQL dialect, `Table` must match the exact table
path including required schema, and `Alias` must match the target alias. For an
unqualified table the caller also asserts the correct database/search path.
Goquent neither reads catalogs nor verifies schema freshness automatically.

Each `WriteKeyConstraint` declares `primary` or `unique`, its complete ordered
`Columns`, and explicitly true `Valid`, `AllRows`, and `NotDeferrable`. A primary
key cannot declare nullable columns. Partial/expression indexes, deferrable
constraints (even if currently immediate), uncertain validity/scope, missing or
unsupported type information are unknown. `AllRows` includes inheritance and
partition effects; if uniqueness over the actual target set is uncertain the
caller must not assert it. Every supplied constraint is validated; an invalid
one prevents a proof even if another key appears usable.

For migration from the old id/presence heuristic:

1. Obtain actual key facts in trusted application code using a separately managed
   schema source. Do not infer facts from a field named `id` or a manifest's column
   names. Ensure facts remain applicable to the executor and current schema.
2. Supply a `WriteKeyContext` before `PlanUpdate`, `PlanDelete`, `Update` or `Delete`.
   For example, a MySQL `users` primary key `id INT` uses `Dialect: "mysql"`,
   `Table: "users"`, a `primary` constraint with all three affirmative assertions,
   and a column `Name: "id", DBType: "INT", Bits: 32, Unsigned: false`.
3. Inspect `WriteScope.Status`, `Reason`, `Precision`, `Provenance`, and
   `Assumptions`. The application still owns tenant and business authorization.
   If a public plan is edited, call `AnalyzeWriteScope` again and use its returned
   result, or regenerate the plan from the query. Do not reuse the old display.

The context is copied on entry and again into private builder evidence. Later
caller slice changes cannot change a generated plan's key facts. Query copies
can share immutable private context. There is no global registry or automatic
promotion from `Query.PrimaryKey`, default `id`, `TableRiskMetadata`, public
Metadata, manifest data, JSON `trusted`/`verified`, or serialized results.
CLI/static/JSON review has no private builder/context evidence and reports unknown.
This includes static chains with a dynamic table expression: an `id` predicate
cannot suppress the bulk warning in the fallback path; its precision is partial.

## Supported types and comparison

| Dialect | Exact accepted DBType | Bits | Database range |
| --- | --- | --- | --- |
| mysql | SMALLINT | 16 | -32768 through 32767 |
| mysql | INT | 32 | -2147483648 through 2147483647 |
| mysql | BIGINT | 64 | -9223372036854775808 through 9223372036854775807 |
| postgres | smallint | 16 | Same signed 16-bit range |
| postgres | integer | 32 | Same signed 32-bit range |
| postgres | bigint | 64 | Same signed 64-bit range |

All require `Unsigned: false`. Other DB type spellings/types, unsigned DB columns,
and width mismatches are unknown in this implementation. Go built-in `int`,
`int8/16/32/64`, `uint`, `uint8/16/32/64` are accepted only when losslessly
representable as int64 and inside the declared DB range. Nonnegative Go unsigned
values can bind signed DB columns within that range. No floating-point conversion
is used. The original SQL parameter types, values and order are **not changed**.
Go type aliases to built-ins behave as built-ins; newly defined types do not.

String/[]byte, float, decimal, time, bool, pointer, custom/Valuer and missing-type
key values are unknown. Inspection never calls a Valuer, marshaler, Executor or
other user method. Correspondence currently supports only built-in scalar
statement parameters; an unsupported SET payload also makes the whole analysis
unknown. Custom Executors keep their interfaces and call timing; the proof
assumes database/sql-compatible integer semantics and does not certify arbitrary
Executor conversions. Execution of unsupported values is unchanged.

References: [MySQL integer types](https://dev.mysql.com/doc/refman/8.0/en/integer-types.html),
[PostgreSQL constraints](https://www.postgresql.org/docs/16/ddl-constraints.html),
[PostgreSQL CREATE TABLE / constraint timing](https://www.postgresql.org/docs/16/sql-createtable.html).
The supported subset is deliberately narrower than either database's type system.

## Logical outcomes

| Structure | Outcome with valid matching context |
| --- | --- |
| All columns of one key, non-NULL integer `=` or singleton `IN` | `at_most_one` |
| Nullable unique, all key columns bound to supported non-NULL values | `at_most_one` |
| AND combining the same target key's columns | Combines bindings; additional understood filters can only restrict the bound |
| OR | Only the same complete key binding in every branch contributes a bound; int8(1) and uint64(1) agree |
| Different-key OR, non-key branch, incomplete composite key | `broad` unless evidence is unknown |
| Range, inequality, multiple IN, NULL/IS NULL, column comparison, NOT | Never positive evidence; by themselves `broad` |
| Empty IN, Raw/function/subquery, unsupported value or unverified clause | `unknown`, including under AND with an otherwise complete key |
| Conflicting equalities | Conservatively `unknown`; no zero-row shortcut |
| JOIN/self-JOIN | `unknown`; never combine keys across aliases |
| Missing/mismatched context, edited/manual/JSON plan | `unknown` |

`broad` means an understood expression has no supported single-row proof, **not**
that multiple rows were measured or must exist. NULL is never used to infer a
zero-row proof. `unknown` carries partial precision and a reason; it takes
precedence over a locally bound key. The tree is inspected completely for each
key; flat PredicateRef fields are never proof inputs. Ordinary identifier tokens
and at most schema.table paths are supported, with exact case/spelling/alias
matching. Quoted/expression identifiers, ambiguous targets and joins are outside
this initial proof. No theorem prover, DNF expansion or general SQL parser is added.
Empty IN preserves existing SQL rendering (`IN ()`), which both tested databases
reject. Its unknown verdict is not a claim of successful zero-row execution.

## Evidence, budgets and diagnostic compatibility

At the builder boundary, a private copy seals operation, SQL, parameter values and
Go types, targets, columns, joins, WHERE/HAVING trees and unverified markers.
Tree parameter positions are checked against actual statement parameters.
Reinspection compares the public fields to this evidence. A public edit cannot
create new private evidence; copying a plan preserves it but changing any sealed
field yields unknown. Metadata and cached verdict edits do not affect the proof.
JSON round trips lose private evidence by design. This is a conditional inspection
contract, not GQ-AI-05 execution-time sealing or concurrent mutation protection.

The correspondence preflight reuses the output guard (depth 256, expanded slots
65536, estimated 8 MiB), rejects cycles and excessive DAG expansion before walking
or copying, and returns unknown without truncating SQL or arguments. Key context
copying allows at most 64 constraints, 256 total key columns, 4096 bytes of identity
strings, 256 bytes per key name, and 512 bytes per column name/type pair. Overflow
becomes invalid context/unknown. PR1's argument-container ownership, payload copy
budgets (depth 64 / slots 65536 / 8 MiB), and public OutputError behavior are
unchanged. These are not total statement memory or arbitrary custom-object bounds.

`BULK_UPDATE_DETECTED` / `BULK_DELETE_DETECTED` keep their codes, medium severity
and configurable/suppressible behavior. They now mean **no at-most-one proof**,
carry the scope result as evidence, and reappear for id or legacy metadata alone.
This is the intentional C05/C06/C07/C10 diagnostic change. Missing-WHERE blocking
remains. RiskLow, suppression and reason strings cannot supply key evidence.
Static/JSON review preserves partial precision on these new unknown diagnostics;
prepopulated JSON warnings cannot hide the recomputed write warning.

Existing compatibility gates, sql.DB/sql.Tx/custom Executor behavior, scanning and
BoolCompat remain. A unified Strict API is **not** implemented here. Strict's
future contract must reject missing/unverified context and must not treat an
approval reason as external authorization. Tenant binding, all-CRUD enforcement,
execution-time correspondence, schema freshness and affected-row enforcement
remain separate work (GQ-AI-03/04/05 and related issues).

## Validation evidence

See [the validation record](write-scope-validation.md) for actual commands,
failures/skips, database versions and CI identity. Unit tests cover logic, integer
boundaries, aliases, deferred/partial/expression facts, mutation, JSON, Valuer
non-invocation, budgets and suppression independence. Real DB tests execute
UPDATE and DELETE in rolled-back transactions for signed 16/32/64-bit keys,
composite/singleton/nullable cases, AND/OR/NOT/NULL, ranges and column comparisons.
They also show the PostgreSQL deferred-unique negative case separately and the
MySQL/PostgreSQL difference for numeric comparison against string unique keys.
Passing these tests does not verify arbitrary supplied contexts or schema freshness.
