# Schema definition comparison (GQ-AI-11 PR1)

`migration.DiffSchemas` and `CompareSchemaDrift` share a structural comparison.
They do not connect to a database or generate executable DDL. SQL `Plan` and
`DryRun` also remain non-executing. The existing explicit `Migrator.Apply` is
unchanged as an SQL execution surface; this change does not add apply, natural
language execution, or MCP database execution.

## Defaults and migration

`ColumnSchema.DefaultState` / `default_state` adds the declarations `unknown`,
`none`, and `expression`. Existing `HasDefault` and `DefaultExpression` remain.

| Declaration | Meaning |
| --- | --- |
| `none`, false, empty expression | Explicit absence of a default |
| `expression`, true, nonempty expression | Declared SQL expression, compared byte for byte |
| Legacy state omitted, true, nonempty expression | Legacy declared expression, compared byte for byte |
| Legacy state omitted, false, empty expression (including explicit JSON false) | Unknown, not absence |
| Missing expression, contradictory fields, unknown state token | Unknown comparison diagnostic |
| JSON null for any default field | Invalid input (SQL `NULL` is an expression string, not JSON null) |

`NULL`, `''`, `'a'`, `'A'`, and function expressions remain distinct source strings.
No casts, parentheses, whitespace, or case are normalized for SQL equivalence.
Different spellings of equivalent SQL can produce change candidates. Identical
expression strings establish only declared structural equality, never dialect
validity, execution safety, authorization, or live evidence.

To migrate legacy input, confirm the intended absence and declare `none`, or
export again through the explicit reader. Do not bulk-convert omitted values to
`none`. Catalog limitations can still produce unknown after export. Contradictory
Go values and decoded source values use the same comparison function. Source
JSON preserves the declaration and `DefaultSource`, including unknown data.
Unknown state strings are retained on roundtrip and fail closed during comparison.

No schema-wide version 2 is introduced. Old JSON still decodes, but old inaccurate
"no differences" behavior is deliberately not preserved. Consumers must be
updated before relying on new states or step kinds: old readers can ignore
additive fields and lose unknown/none distinctions. Exporting to an older
consumer is not precision-preserving. Added public struct fields also affect
external unkeyed composite literals; use keyed literals. No dependency is added.

## Indexes and steps

Ordered index columns, column composition and `Unique` participate in comparison.
`Concurrent` is a desired creation method, not stored-definition identity; changing
it alone is not a schema change. `alter_index` and `alter_column_default` record
current and desired source definitions. Index additions/deletions also retain
available definitions. Index column slices are detached from caller input.

These names describe structural records, not database ALTER syntax. Replacement
DDL, dependency order, unique-data checks and locking remain unverified. No
automatic drop/add sequence is prescribed. Both new kinds have high structural
risk, partial execution-analysis precision and are blocked by `EnsureExecutable`,
even with an approval reason or empty statements. Public `Blocked` also records
that refusal. A low risk, precise label, name or reason is not evidence or consent.

The supported identity subset is unqualified lowercase ASCII identifiers. Earlier
maps silently trimmed and folded names and overwrote duplicates. Ambiguous
case/quoting/qualification, duplicate names, missing referenced columns and
nil/empty index columns now yield comparison diagnostics. Nil/empty schema lists
still mean empty lists. Invalid tables do not erase changes in other valid tables;
unknown index/default definitions do not erase known changes in their table.
This bounded subset does not implement a general SQL parser or schema identity.

## Unknown and output boundaries

`MigrationPlan.ComparisonIncomplete` and `DriftReport.ComparisonIncomplete` retain
unknown comparisons. `ComparisonUnknown` steps are diagnostic markers, **not**
detected schema changes. `Drifted` counts actual changes only, so false plus
incomplete means unknown rather than a match. The plan and drift share the same
steps. Public drift JSON exposes `unknown` and `known`; CLI drift returns 2 for an
incomplete comparison, 1 for complete detected drift, and 0 for complete equality
within the represented subset. Known changes remain visible alongside unknowns.

Source `ToJSON`, `WriteJSON`, `WriteDriftJSON`, and `WriteSchemaJSON` remain
sensitive artifacts. New source definitions may contain secrets. Public plan,
Pretty/String/Format and CLI JSON expose only bounded classifications and fixed
diagnostics, with no identifiers, defaults, SQL, source labels or raw definitions.
The existing public view version remains 1; the closed step/code allowlists gain
the two new kinds. There is no separate migration JSON-schema generator.

Manifest model generation does not gain default-based validation. The existing
migration-schema fingerprint hashes source JSON, so adding state/source/index
metadata changes that fingerprint. Existing saved fingerprints are not silently
refreshed or treated as fresh. This is not database identity or freshness evidence.

## Explicit reader scope

The reader continues to issue SELECT catalog queries only. Fixture DDL belongs to
tests, not to export/comparison. The actual live cases are in
`tests/schema_diff_live_test.go`; pure comparisons and public canaries are in
`orm/migration/schema_compare_test.go` and the CLI regression suite.

| Catalog representation | PR1 handling |
| --- | --- |
| PostgreSQL non-null `column_default` SQL expression | Preserve exact expression; known declaration |
| PostgreSQL/MySQL catalog NULL | Unknown (absence and explicit NULL are not inferred) |
| MySQL integer column with signed decimal integer default text | Known finite integer expression subset |
| MySQL text/empty-string/other default representation | Preserve raw catalog text and source label; unknown, without guessed quoting |
| PostgreSQL simple unquoted btree ordered columns | Compare ordered columns and uniqueness; retain raw definition |
| PostgreSQL expression/partial/INCLUDE/ordered/opclass/quoted/non-btree index | Unknown; retain raw definition |
| MySQL ordinary visible ascending full-column BTREE | Compare ordered columns and uniqueness |
| MySQL prefix/expression/descending/invisible/non-BTREE or inconsistent index rows | Unknown |

MySQL export now reads `sub_part`, `collation`, `index_type` and `is_visible` to
avoid promoting incomplete index definitions. This path targets the existing
MySQL 8 integration environment; older catalogs that lack these columns return
an export error, not a fabricated complete schema. Actual server versions belong
in validation evidence; none are inferred in plans. There are no lock, rebuild,
transaction or CONCURRENTLY guarantees.

FK/CHECK, complete schema identification and the capability matrix remain PR2.
Migration phases, preflight evidence freshness and target binding remain PR3.
"Complete" here only covers represented supported comparisons, not all database
objects. The whole Issue's acceptance criteria remain unchecked.
