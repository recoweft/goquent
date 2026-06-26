# Changelog

## Unreleased
- Moved the SQL query builder into `orm/internal/querybuilder`, removed the external
  `goquent-query-builder` module dependency, and prepared the old Builder repository for archive.
- Added Builder snapshot/state-copy contracts so `QueryPlan`, update/delete state copy, and grouped
  predicates no longer rely on reflection or unsafe field writes.
- Hid Builder callback types from the public query API; `JoinQuery` callbacks now use
  `*query.JoinClause`.
- Hardened `goquent review --require-fresh-manifest`: missing, stale, or unverified manifests now
  fail the manifest gate, and review can verify current `--schema`, `--policy`, `--code`, and
  `--database-schema` inputs directly.
- Added JSON review config support for manifest defaults, `fail_on_precision`, rule severity
  overrides, and config-scoped suppressions with reason/owner/expiration metadata.
- Made static Go review manifest-aware for literal table query chains, including tenant scope,
  soft delete, PII, required-filter, and manifest key metadata checks.
- Tightened bulk write risk detection so foreign-key-looking columns such as `tenant_id` are not
  treated as primary-key-like by name alone; manifest primary keys and unique indexes can provide
  narrow-write metadata.
- `OperationSpec` now rejects unresolved `value_ref` entries instead of compiling placeholder
  strings into query params.
- `Where` no longer attempts to automatically detect dotted values as column names.
  Use `WhereColumn` for column-to-column comparisons.
- Added generic `SelectOne` and `SelectAll` APIs supporting struct and map destinations.
- Added generic `Insert`, `Update`, and `Upsert` helpers with unified struct and map support.
- Added `PK` option to configure primary key columns for map writes.
- Added boolean dialect compatibility with configurable `BoolScanPolicy` and field tags
  `boolstrict`/`boollenient`.
