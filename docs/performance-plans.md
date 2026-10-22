# Explicit database performance plans

GQ-AI-12, PR1 (1/2), [Issue #94](https://github.com/recoweft/goquent/issues/94).
`Query.ExplainPerformance(ctx, query.PerformanceOptions{})` is an explicit live
operation. `Plan`, dry runs, ordinary CRUD, CLI and MCP do not call it. This PR
adds no observation hooks, query budget, benchmark claim or migration Apply path.

The terminal applies policy once to a detached builder, checks its full internal
structure, and renders with the existing dialect renderer. Structural inspection,
normal policy/Strict/tenant gates and the acquired EXPLAIN use that same private
SQL and ordered argument snapshot. Public QueryPlan fields are never execution
inputs. The terminal captures owner, executor, context and settings before database
calls; it does not regenerate SQL from a subsequently changed Query. Its returned
source describes the captured operation only. Concurrent Query mutation remains
unsupported. A PerformancePlan is not a ValidatedPlan or an execution permit.

## Supported domain

The structural grammar is one table, explicit ordinary column projections, normal
column ordering, LIMIT 1..10000, OFFSET 0..10000, AND/OR/groups, column-to-value
`=`, `<>`, `<`, `<=`, `>`, `>=`, nonempty IN, NULL tests and BETWEEN. Identifiers
are bounded ASCII names; columns may be qualified by the same table. NULL values
retain SQL NULL semantics. There is no rewrite to expand the accepted domain.

The initial catalog-verified type combinations are **signed integer columns with
integer or nil values**. PostgreSQL supports int2/int4/int8; MySQL supports signed
tinyint/smallint/mediumint/int/bigint. Values must fit the actual column width.
All visible columns in the table must belong to this initial domain, including
unselected columns. String, boolean, float, binary and time values may pass the
structural scalar check but have no verified catalog combination in this revision
and are refused before EXPLAIN. No opaque named values, Valuer, Stringer or
Marshaler are invoked to turn unsupported input into accepted input.

PostgreSQL requires an explicit schema-qualified table. It fixes transaction-local
search_path to pg_catalog. Both engines require ordinary persistent base tables
and ordinary integer B-tree indexes. Views, temporary tables, foreign tables,
partitions/inheritance, RLS/rules, generated columns, expression/partial indexes,
custom types/casts/operators/opclasses, extended statistics, relation options,
check/foreign-key constraints and unknown
catalog states are refused. PostgreSQL checks actual bootstrap OIDs, function
language/namespace/support, integer operator implementations and index families;
namespace or a caller's safe flag alone is insufficient. MySQL checks the actual
SHOW CREATE target as well as information_schema to refuse temporary-table
shadowing. MySQL requires a direct global/schema/table SELECT grant for complete
catalog visibility; column-only and role-only grants are conservatively refused.
Cross-database MySQL targets and nonempty CREATE_OPTIONS are unsupported.
Metadata reads do occur for catalog refusals.

Raw/SafeRaw, expressions/functions/aggregates/COUNT/DISTINCT/star projections,
joins/lateral/subqueries/EXISTS/unions, grouping/having, column-to-column predicates,
JSON/date/fulltext operations, all locking modes, DML, unknown nodes and dialects
are refused before connection acquisition or version queries. These restrictions
apply only to the new terminal; ordinary query APIs retain their existing domain.

## Acquisition and trust boundary

The initial native implementations are PostgreSQL 16.x and MySQL 8.4.x. Version
is read on the actual pinned connection. Other versions, MariaDB and unknown
version formats do not receive target EXPLAIN. A version-only source can accompany
an unsupported error; it is unknown, has no plan estimates, and is not a complete
native performance plan.

A real sql.DB and the actual built-in mysql/pq driver and connection type are
required. External sql.Tx, custom Executor/driver wrappers and unknown dialects
are refused before database calls. Registry names and external JSON are not native
evidence. Custom connectors returning a nonnative connection are refused before
Begin or SQL, although detecting that requires acquiring the connection. Existing
external-transaction CRUD remains unchanged.

One physical connection is pinned for an owned read-only transaction, version,
metadata and EXPLAIN. PostgreSQL uses `EXPLAIN (ANALYZE FALSE, FORMAT JSON)`;
MySQL uses `EXPLAIN FORMAT=JSON`. The transaction always rolls back and the
connection returns to the pool. Begin/rollback and the transaction-local setting
are acquisition controls; no target DML, DDL, ANALYZE or query observation is added.
Cleanup failure is not reported as complete success.

The total connection wait/Begin/version/catalog/EXPLAIN/read budget is at most
five seconds from entry, shortened by caller context or Timeout. Query context
cancellation also cancels acquisition. Options cannot increase the maximum.
Cancellation support ultimately depends on the built-in driver and server.

**Operating prerequisites:** use a standard server/driver and a managed schema;
prevent DDL changes during acquisition. Catalog checks do not prove the absence
of all DDL races or defend against a malicious administrator/plugin replacing
built-in server behavior. Physical database identity and application authorization
are not attested. A read-only transaction and rollback alone are not evidence of
arbitrary SQL being side-effect free.

PostgreSQL can evaluate immutable functions during planning and supports planner
support functions. MySQL documents cases where EXPLAIN of a subquery invokes a
stored function that changes data. These are reasons for the narrow structural
and catalog intersection, not claims that an arbitrary SELECT is harmless.
See the official [PostgreSQL EXPLAIN](https://www.postgresql.org/docs/16/sql-explain.html),
[function volatility](https://www.postgresql.org/docs/16/xfunc-volatility.html),
[CREATE FUNCTION](https://www.postgresql.org/docs/16/sql-createfunction.html),
[MySQL EXPLAIN](https://dev.mysql.com/doc/refman/8.4/en/explain.html) and
[derived tables](https://dev.mysql.com/doc/refman/8.4/en/derived-tables.html) documentation.

## Evidence, bounds and publication

PerformancePlan v1 is independent of QueryPlan. Its sensitive source includes the
captured target, actual dialect/version, database name, client UTC acquisition time,
raw EXPLAIN and available estimated rows/index names/scan kinds/costs. Raw JSON may
contain SQL, values and identifiers. Missing values differ from zero. An absent index name remains unavailable in
the source; the public Indexed flag only reports observing a nonempty name.
False does not prove that the entire plan uses no index. Unknown
nodes or omitted metrics remain unknown; partially interpreted results are not
complete evidence. Costs use each engine's own units, not comparable elapsed times. PostgreSQL rows/total cost and MySQL
rows_examined_per_scan/prefix_cost retain their distinct meanings; raw source
also preserves other server-provided metrics.

The parser classifies keys and types at every supported JSON level: the PostgreSQL
root wrapper, plan nodes and child arrays, and the MySQL root, query blocks,
operation wrappers, tables and separate query/table/sort cost objects. Unknown
fields, nodes and extension subtrees propagate to the source and public aggregate
`unknown`; known estimates can remain as partial evidence. Unknown subtrees are
not recursively interpreted as supported plan nodes. The complete JSON still
passes duplicate-key, numeric-finiteness, depth, node-count and byte checks.
Present known fields with incorrect types (including null), invalid metrics or
out-of-range values return `PERFORMANCE_RESULT`, rather than becoming complete
evidence. Absent metrics remain unknown.

Recognized metadata is typed, not a blanket exemption: PostgreSQL boolean flags,
integer width/row estimates, finite costs, string relation/index/predicate fields,
string arrays and finite direction/parent classifications; MySQL integer row/ID
fields, boolean flags, string identifiers/conditions, string arrays, numeric cost
and key-length strings, bounded filtering percentages and unit-bearing data-size
strings. Identifier and expression text stays sensitive and is not evaluated or
used to prove SQL semantics. Free-form messages, unsupported operations and
unclassified auxiliary objects remain unknown. These classifications describe
the [PostgreSQL estimates](https://www.postgresql.org/docs/16/using-explain.html)
and [MySQL JSON metadata](https://dev.mysql.com/doc/refman/8.4/en/explain-output.html);
they do not expand the supported SELECT/catalog/version/driver domain.
No performance improvement, returned-row guarantee, fast/safe verdict or
application authorization follows from these estimates.

`ToJSON` and ordinary source JSON are **sensitive data APIs**. `PublicView`, String
and Format publish only fixed vocabulary and bounded numeric summaries. The view
has its own version-1 strict decoder and reapplies its allowlist after exported
field mutation. It excludes SQL, values, identifiers, version strings, raw plans,
error causes and value-derived hashes. Independent caller field extraction,
methodless casts and application logging remain outside this display boundary.
`Native()` reports in-process acquisition provenance, not authority over edited
source fields. Source serialization also enforces the 1 MiB/depth-32/4096-value JSON budget.
The target envelope and escaping can make source export exceed that budget even
when the raw plan alone fits; export then returns a fixed limit error. Successful
source exports roundtrip within the same budget. Decode into a fresh receiver
and discard it on error, as with existing QueryPlan source readers. Source JSON
roundtrips clear that private provenance; no attach or
view-to-execution API exists. Externally supplied display provenance is a claim,
not a native acquisition fact.

Structural limits are depth 32, 256 condition/group nodes, 1024 arguments,
128 projections/order terms, 64 KiB SQL and 1 MiB scalar payload. Version results
are one row/4 KiB; EXPLAIN is one row/1 MiB. JSON depth is 32 with at most 4096
values. Catalog collectors have explicit row and byte bounds (at most 256 rows
and 64 KiB per query). Overflow, duplicate JSON keys, trailing JSON, invalid numeric
estimates, Scan/rows.Err/Close and query/cleanup failures produce fixed errors
without raw causes. Estimates are finite nonnegative numbers at most 1e15.
The public node list is capped at 128 and marks truncation.

These are client inspection/output bounds. A driver may allocate a single returned
cell before the post-Scan size check. LIMIT does not bound planner CPU, memory,
work or locking. No hard server resource or immediate cleanup latency guarantee
is made. Source files are not exported by a new CLI/UI; the existing exclusive
0600 unsafe-local-output rules and CI refusal are unchanged.

## Compatibility and remaining work

The API and source/view v1 envelopes are additive. Existing QueryPlan JSON,
ValidatedPlan, policy, Strict, typed values, CLI/MCP and executor interfaces are
unchanged. No dependency was added. Internal builder types remain internal;
lower-level packages do not import the root facade or CLI.

Tests distinguish mocks from native PostgreSQL/MySQL fixtures. The validation
record reports fresh results, unavailable checks and existing review/manifest
limitations separately. PR2 owns observation hooks, context query budgets, N+1
suspicions, benchmarks and operating guidance. Neither this PR nor its tests
close the whole Issue or adopt its overall acceptance criteria.
