# Migration preflight evidence (GQ-AI-11 PR3)

This is a necessary-condition inspector, not migration execution, deployment
approval, or proof of a successful constraint creation. Planning and offline
verification do not connect. Explicit collection issues finite SELECTs only.
Existing `Apply`, `EnsureExecutable`, structural-only refusals, incomplete
comparison, and PR1/PR2 diagnostics are unchanged. No MCP DB route is added.

## Specification adoption

Work `a6a3a73f-9c8d-4edc-acb9-8a20479bf204`, revision 1, adopted the answer to
consultation `8f4263aa-c355-4489-ac9c-678dedc49603`: recommendation A, items 1–6,
with the controller's receipt-content, visibility, comparison-semantics, bounds,
and verification conditions. The answer was delivered in the same revision.
No signing infrastructure or new physical-identity trust root is introduced.

## Source and phases

`PlanPreflight` accepts an existing `MigrationPlan` with explicit current and
desired schemas, per-step `PhaseContract` declarations, and optional additional
reference schema scopes. It creates a detached `Preflight` source envelope v1.
The plan retains source definitions; derived requirements are recomputed during
validation, not accepted from caller labels. The bounded source decoder rejects
recursive duplicate keys, case aliases, explicit null and oversized/deep input.
Optional values must be omitted. Unknown enum strings remain source data and
become unknown diagnostics. Direct Go construction uses the same binding validator.

Phases are `expand`, `backfill`, `contract`, and unknown. Dependencies are
zero-based earlier step ordinals; forward references, cycles, duplicates, and
phase-order reversal are invalid. Omission is unknown. Old-reader and old-writer
compatibility must each explicitly say `compatible`; this is a declaration, not
proof that older applications have stopped. Contract steps after the first must
have dependencies. Requirements of earlier steps remain in the aggregate, so a
later zero count cannot erase unobserved prerequisite work. Rename/drop, new
objects, type conversions and backfill work retain an external-work unknown.
The library does not execute or certify a backfill job.

Legacy `ReviewModeBackfill`, `Preflight []string`, approval reasons, and phase
names alone never mean inspected or completed. SQL-bearing steps retain an
unresolved-definition requirement: the finite schema model is not a general SQL
parser, and source SQL is not assumed to agree with supplied structural objects.
The initial measured cases are existing-column NOT NULL, ordinary plain-column
unique indexes, and MATCH SIMPLE FK additions/changes. The desired definition
must agree with the step's target. Unknown comparisons remain unknown.

## Identity and canonical binding

Canonical domains are independently named `goquent.preflight.plan.v1`,
`step.v1`, `requirement.v1`, `schema.v1`, and `collection.v1` (each with the common
`goquent.preflight.` prefix). SHA-256 inputs are typed JSON with a domain separator.
Schema and MigrationPlan global wire versions do not change. No existing manifest
fingerprint is refreshed.

The plan digest includes current/desired schemas, additional reference scopes,
actual observed version declarations, dialect, catalog/schema tuple, visibility,
coverage, ordered steps, exact SQL bytes, phase/dependencies/compatibility, and
the definitions from which requirements are derived. Step identity binds plan,
ordinal, and step definition. Requirement identity binds step, kind, ordered
local/reference columns and NULL semantics. Display warnings/reasons and approval
are excluded. Collection time is excluded from plan identity but protected by
the separate collection digest and private receipt.

Names are exact components: no trimming, folding, dotted-name splitting, SQL
quoting interpretation, or omitted-component completion. Measured targets use
lowercase ASCII identifiers. Named table/index/constraint collections and scope
selections are sorted; column, index-key, FK-pair and step orders are preserved.
Duplicate or ambiguous target identities are invalid. Source-only names such as
MySQL `PRIMARY` are retained without claiming that every definition is supported.
Nullable presence, coverage and unknown values remain part of the fingerprint;
missing coverage is not an empty collection. A fingerprint covers only the
represented source and explicit scopes, not unmodeled database attributes.

## Origin, lifetime, freshness

`NewPreflightCollector` accepts a `*sql.DB` using the supported MySQL or PostgreSQL
native driver, and owns a dedicated `sql.Conn`. Arbitrary executor wrappers,
source labels, schema names, caller IDs and fingerprints do not create trust.
`Collect` binds the entire produced evidence to a private receipt and the open
collector. `Collector.Evaluate` checks the current content against that receipt.
Changing counts, completion, time, plan, target or fingerprint fails even if an
external caller recomputes the public digest. JSON never contains the receipt.
Detached JSON copies, other sessions, other processes, reconnection and closed
collectors cannot adopt the evidence; re-collection is required. A Go shallow
copy can still reference the same receipt, but cannot rebind or modify its content.

This proves correspondence to observations in that connection session, not a
permanent physical-server identity, proxy destination, authorization, or resistance
to malicious application code/drivers. A hash detects inconsistent edits but is
not a signature; a third party can recompute it. External source verification is
consistency inspection only and always leaves origin unknown.

Evaluation requires an injected clock and positive maximum age at most five
minutes. CLI uses UTC system time. The start and end must exist and satisfy
`start <= end <= now`; future tolerance is zero. Age is measured from start:
exactly five minutes is accepted, one nanosecond over is expired. Clock reversal,
missing/invalid time and future observations remain unknown. Clock options are
caller configuration, not a trust root. Reuse additionally requires the same
open collector, plan/step/requirement, fingerprint and read scope.

## Collection semantics and limits

The collector re-reads schema before and after collection, and re-observes the
finite target shape/visibility attributes after counts. Mismatch or any read
failure invalidates the entire collection, preserving prior results only as
partial source. This does not provide one transaction snapshot, detect every
concurrent write, or detect an intermediate change that was reverted. There is
no guarantee about later writes, locks, rebuilds, transaction eligibility,
constraint creation or migration success.

- NULL: exact count of rows whose one target column is NULL.
- Unique: exact count of **duplicate groups**, excluding any tuple with a NULL.
  Ordered plain-column btree keys only; no predicate, expression, prefix, include
  or opaque raw index definition. Ordinary NULL-distinct semantics only.
- FK: exact count of non-NULL local tuples for which the ordered reference tuple
  does not exist. MATCH SIMPLE only; references must be observed in explicit scope.

No LIMIT or sampling turns an unmeasured tail into zero. Count is a nonnegative
`*int64` plus completion; missing, NULL, overflow, negative, extra rows and driver
errors are not zero. No row values are retained. Target-specific row-scope
observations are separate from PR2's still-partial inventory/column coverage.

Comparison support is limited to PostgreSQL 16 built-in int2/int4/int8 and MySQL
8.4 integer types (including same signedness), with matching FK types. Special
collation, unconfirmed comparison semantics, generated columns, partitioning,
inheritance, views and foreign tables are unsupported. PostgreSQL RLS on either
side is unknown even for bypass users; full table SELECT privilege is required.
For MySQL the finite subset is InnoDB base tables without partitions; these do
not implement RLS, and the count SELECT must succeed with the needed column
privileges. Successful catalog visibility alone never proves collection coverage.
No role changes, SET statements, privilege grants, DDL or DML are issued by the
collector. Fixture setup and cleanup are test-only.

Maximums: 64 total requirements, 16 local columns and separately 16 reference
columns per requirement, 30 seconds for the collection and five seconds per
query. Optional time bounds may only be shortened. The dedicated connection is
acquired with a bounded context. Drivers must honor cancellation: these are not
hard real-time or lock-free guarantees. Query, Scan, rows.Err, cancellation and
timeout errors are fixed failures, never successful empty results.

## API, CLI, publication and migration

Use `migration.PlanPreflight`, `NewPreflightCollector`, `Collect`,
`Collector.Evaluate`, and `EvaluatePreflight` (offline). Evaluate and generate a
public summary before closing the collector. Source structures and their JSON
are sensitive local artifacts; public output is independent preflight view v1,
with closed statuses/codes and at most 64 check summaries. Counts, timestamps,
identifiers, raw DB versions, SQL, expressions and arbitrary errors are omitted.
The complete internal result, not truncated display data, decides exit status.

```
goquent migrate preflight plan --plan-source plan.json --phases phases.json \
  --unsafe-local-output preflight.json
goquent migrate preflight collect --source preflight.json --driver postgres \
  --dsn "$POSTGRES_DSN" --unsafe-local-output observed.json
goquent migrate preflight verify --source observed.json
```

Additional `--reference-schema` files explicitly declare cross-schema scopes.
`plan` does not acquire schemas; use the existing explicit schema export separately.
Local outputs use existing new-file, mode-0600, no-CI restrictions. Default JSON
and pretty output contain only the public summary. `verify` does not mean trusted
DB verification: even a consistent collected artifact has external origin there.

Exit codes: plan 0 is generation success only. Collect 0 requires all necessary
conditions satisfied within the observed scope; 1 means unsatisfied, unknown or
unsupported. Verify returns 1 for consistent external evidence, 2 for invalid
syntax/binding or failed collection. Input/connect/query/Scan/rows.Err/timeout/
cancel errors return 2 with a partial public/source result when available.
Operational failures dominate; otherwise aggregate severity is unknown,
unsupported, unsatisfied, satisfied. This is not an execution gate.

Old schema/plan JSON remains loadable by its existing APIs. The new inspector
requires explicit source context rather than inventing missing identity. New
source fields and envelope version have independent contracts; old consumers
that ignore fields do not preserve precision and cannot adopt evidence. Update
consumers before exporting. Existing public v1 views remain unchanged. No new
module dependency, low-level import of root/CLI, or public internal-builder type
is introduced. Whole-Issue acceptance remains a separate review decision.

Always decode external source into a fresh receiver and discard it on error.
Method-level decode clears the receipt; standard-library syntax failures before
method dispatch do not promise invalidation of an existing receiver or aliases.
Concurrent mutation of source inputs during collection/evaluation is unsupported.
