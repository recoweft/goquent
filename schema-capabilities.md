# Schema capabilities and coverage (GQ-AI-11 PR2)

This additive source contract extends [PR1 comparison](schema-definition-comparison.md).
`DiffSchemas` and `CompareSchemaDrift` use the same structural comparison. Neither
connects to a database. The explicit `ReadSchema` path issues catalog SELECTs only.
No new apply, DDL generation, MCP execution, migration phase, or preflight evidence
acceptance path is introduced. Fixture DDL belongs only to tests.

## Identity, scope, and absence

`Schema.Context` carries sensitive dialect, catalog, namespace, actual catalog
version text, requested table selection, and coverage. Each table has a separate
`ObjectIdentity` tuple (catalog, schema, name, state). MySQL's catalog is the
information-schema catalog `def`; its schema is the database. PostgreSQL's catalog
is `current_database()`. A default namespace selected by the reader is recorded
only after a catalog SELECT returns that identity. External inputs are never
bound implicitly to that namespace.

The comparison subset is exact lowercase ASCII components. No trimming, case
folding, SQL quoting interpretation, dotted-name splitting, or omitted-component
completion is performed. Table keys preserve tuple boundaries. Parent/table
contradictions, duplicate identities, quoted/mixed-case names, missing identity,
and unknown enum values remain incomplete. Different known namespaces produce a
`schema_identity_difference` record and an incomplete-scope marker; this is not a
rename, namespace move, or execution sequence. A single export covers one
namespace; multi-namespace snapshots are not a supported complete comparison.
Cross-schema FK references are retained without claiming to inspect their targets.

`SchemaCoverage` separates tables, columns, indexes, foreign keys, checks, identity,
and version. States are `complete`, `partial`, `unsupported`, and `unknown`.
Omitted/zero/unrecognized states mean unknown. `complete` describes a supplied
observation or declaration within its stated scope and represented subset, not
all DB objects, privilege, safety, or freshness. Version coverage feeds capability
assessment; an unknown version does not by itself invalidate a structural
comparison whose other declarations are complete.

Both live readers retain `catalog_visible` visibility and **partial table
inventory and column coverage**, even for an administrator. MySQL also retains
partial index/constraint collections because its information schema is privilege
filtered. PostgreSQL records complete represented index/FK/CHECK catalogs for
observed tables, but not complete table/column visibility. Table discovery starts
from visible column rows; zero-column tables and objects without visible columns
are not a complete inventory. Missing requested
tables can be absent or inaccessible: neither means a known empty namespace.
Query, Scan, and rows.Err failures return errors rather than fabricated empty
collections. Unsupported catalog versions can fail export. No fallback presents
an older query as equally precise.

Explicit desired/source declarations can state `visibility: declared`, complete
inventory, and the same `namespace` or exact `tables` scope on both sides. Only
then can a missing table be a definite collection addition/deletion. Column,
index, and constraint absence also requires the relevant collection coverage and
matching scope. Otherwise a source-bearing unknown marker preserves the candidate
without counting it as a detected change. Legacy unqualified, unique names can
still expose local type/default/index differences when neither side claims an
identity; the whole comparison remains unknown. Known explicit identities are
never matched to contradictory or ambiguous candidates.

## Definition representation

| Source | Finite comparison | Limits retained |
| --- | --- | --- |
| FK | Ordered local/reference columns; reference tuple; update/delete actions; match; validation, enforcement, deferrability | Missing/contradictory members and unknown actions are unknown; external targets are not validated |
| CHECK | Exact expression bytes, validation, enforcement, inheritance | No SQL equivalence or data-satisfaction proof |
| Index | Ordered column/expression union, predicate absent/expression, btree method, ordering/null order, prefix, include columns, visibility, uniqueness | Nondefault PostgreSQL opclasses/collations/options, invalid/unready/partitioned indexes and non-btree methods remain unknown |
| Legacy index | Ordered Columns and Unique local comparison retained | Missing extended attribute declaration adds unknown; Concurrent remains a creation request, not identity |

MySQL does not report historical FK data validation after foreign_key_checks was
disabled. Its FK/CHECK validation stays unknown/partial here. Known action,
reference, or expression changes remain visible alongside that unknown. PostgreSQL
`NOT VALID` is preserved. SQL expressions/predicates/raw catalog definitions are
not normalized: equivalent SQL spellings can produce candidate changes. For
PostgreSQL expression keys with unrepresented collation semantics, raw text is
retained and comparison remains unknown. MySQL partial indexes are unsupported;
the dedicated fixture measures rejection of WHERE index syntax.

A key cannot contain both column and expression. Supplied legacy Columns must
agree with the column keys when both representations are present. Duplicates,
missing local columns, inconsistent reference lengths, and contradictory states
use the same validator for decoded JSON and direct Go construction. Raw data is
retained even when it cannot be compared. Differing nonempty raw index definitions
with identical represented attributes produce unknown, not an equal verdict. Complete coverage does not override
invalid definitions.

New `add/drop/alter_foreign_key`, `add/drop/alter_check`, and
`schema_identity_difference` records are structural facts, **not executable
migration recipes**. `EnsureExecutable` refuses them even with approval and no
statements. PR1 `alter_index`/`alter_column_default` refusal remains intact.
Where collection absence is explicitly known, an add/drop can coexist with a
definition-unknown marker; neither marker nor incomplete scope invents absence.
Unknown markers are not actual changes. `Drifted=false` with incomplete=true is
not a match. Known changes and unknown markers coexist. Detached plan/drift source
snapshots preserve context, collections, definitions, and contradictions, including
whole-table additions/deletions and incomplete candidates.

## Source compatibility and public output

No schema-wide v2 is introduced. Old JSON and keyed Go construction still load,
but zero coverage is not an empty constraint collection. Existing columns/default
unknown and nullable-presence semantics remain unchanged. Newly extended source
objects reject duplicate JSON keys, case aliases of known fields, and explicit
null for known members. Omission remains unknown; nil/empty collections without
complete coverage do not declare absence. Unknown enum strings survive source
roundtrip and fail closed in comparison. Unknown object fields remain outside the
finite source contract and are not promised roundtrip preservation.

Update consumers before exporting new source data. Re-export or explicitly
confirm intended desired declarations; never bulk-promote legacy coverage to
complete. Old readers ignore additive fields and are **not precision preserving**.
Added exported fields affect unkeyed Go composite literals. Fingerprints based on
source JSON change; saved fingerprints are not silently refreshed. Migration plan and drift public views retain version 1 with an additive `capabilities`
member and new closed step kinds. Earlier strict public-view decoders reject
unknown members; upgrade display consumers before publishing these extended
views. New decoders still accept old views with omitted capabilities. Neither
source nor public downgrade is advertised as precision preserving.

Source metadata
is mutable and unauthenticated; neither a complete declaration nor a reader label
proves physical DB identity, freshness, or authorization.

`ToJSON`, `WriteSchemaJSON`, `WriteDriftJSON`, and source MarshalJSON are sensitive
artifacts. New snapshots can contain identifiers, SQL, versions, and raw catalog
text (selected MySQL catalog fields, PostgreSQL rendered definitions). Public migration views contain bounded fixed classifications, including
capability axes, with no such source values. Schema/drift summaries expose known
and unknown status, and drift carries the same bounded capability axes; default CLI output uses these public views. Public unknown
strings are mapped to fixed `unknown`, not echoed. CLI drift returns 2 for
incomplete comparisons, 1 for complete drift, and 0 for a complete structural
match. This intentionally changes legacy empty-input behavior. CI/MCP continue
using the existing fixed-output boundary; no new DB execution surface is added.

## Capability table

General support and target conditions are independent. The four axes are
lock-free operation, no table rebuild, transaction support, and concurrent-index
syntax. No result here satisfies target conditions for a new deployment.
`source=live_observation` requires the in-process reader's private version
observation to match the current context. Source JSON does not carry that private
observation; a desired version or a forged `catalog_select` label yields
unverified/unknown. Even an in-process observation does not prove execution-time
freshness, object identity, or permission. This is not PR3 evidence acceptance.

| Observed version family / operation | Lock-free | No rebuild | Transaction | Concurrent syntax |
| --- | --- | --- | --- | --- |
| PostgreSQL 16.x, add index | Unsupported (locks still exist) | Conditional; target unknown | General support; CONCURRENTLY request is unsupported in transaction | Conditional; target unknown |
| PostgreSQL 16.x, add/drop FK/CHECK or alter default | Unsupported | Unknown | General support; target unknown | Unknown |
| MySQL 8.4.x, add/drop index/FK/CHECK or alter default | Unsupported (metadata locks) | Conditional; engine/table/algorithm unknown | Unsupported (implicit commit) | Unsupported PostgreSQL syntax |
| Other operations, missing version, unknown version, external source claims | Unknown | Unknown | Unknown | Unknown |

Local fixtures exercise MySQL **8.4.6** and PostgreSQL **16.10**, including actual
MySQL implicit commit, PostgreSQL index DDL rollback, and CONCURRENTLY refusal
inside a transaction. Other patch releases are documentation-backed family
classifications, not locally measured releases. MySQL 8.0 and 8.4 vendor-suffixed
version strings are deliberately unknown in this initial matrix. Lock duration,
physical rebuilds, edition, engine alternatives, concurrent workloads, and
production feasibility are unmeasured. Online/algorithm syntax alone never
means no downtime or no locks.

Official references (read alongside the finite code in `schema_capabilities.go`):

- [PostgreSQL 16 CREATE INDEX](https://www.postgresql.org/docs/16/sql-createindex.html): concurrent build restrictions, expression/partial keys, locks and transaction restriction.
- [PostgreSQL 16 ALTER TABLE](https://www.postgresql.org/docs/16/sql-altertable.html): operation-dependent locking and validation.
- [PostgreSQL 16 pg_index](https://www.postgresql.org/docs/16/catalog-pg-index.html): key/include order, validity, options, and predicates.
- [MySQL 8.4 online DDL operations](https://dev.mysql.com/doc/refman/8.4/en/innodb-online-ddl-operations.html): engine/operation-dependent algorithm and rebuild constraints.
- [MySQL 8.4 online DDL limitations](https://dev.mysql.com/doc/refman/8.4/en/innodb-online-ddl-limitations.html): metadata lock and concurrency limitations.
- [MySQL 8.4 implicit commits](https://dev.mysql.com/doc/refman/8.4/en/implicit-commit.html): DDL transaction boundary.
- [MySQL 8.4 foreign keys](https://dev.mysql.com/doc/refman/8.4/en/create-table-foreign-keys.html): foreign_key_checks and data-validation limitations.

No new dependency is introduced. PR3 still owns phase design and preflight
evidence target/freshness validation. Whole-Issue acceptance remains separate.

## Finite source tokens

Coverage uses the states listed above. FK actions are `no_action`, `restrict`,
`cascade`, `set_null`, `set_default`; match is `simple`, `full`, or `partial`.
Validation is `validated` or `not_validated` (MySQL retains `unknown`);
enforcement is `enforced` or `not_enforced`; deferrability is `not_deferrable`,
`immediate`, or `deferred`. CHECK inheritance is `inheritable`, `no_inherit`, or
`not_applicable`. These represent declarations, not dialect acceptance.
Index expression/predicate states use `none` or `expression`; ordering is
`asc`/`desc`, null order is `first`/`last`/`engine_default`, and visibility is
`visible`/`invisible`. Prefix zero with complete index attributes declares no
prefix; a positive prefix records a catalog length. An expression key cannot
have a prefix. Unknown, missing, and contradictory tokens remain unknown.
