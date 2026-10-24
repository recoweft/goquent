# Finite native performance boundary evidence

GQ-MAINT-09 PR1, [Issue #115](https://github.com/recoweft/goquent/issues/115).
This supplements [the acquisition contract](performance-plans.md); it does not
expand the accepted grammar, native versions, error meanings, or authority.

## Case matrix

All cases require the existing disposable MySQL/PostgreSQL Compose fixture,
three explicit fixture DSNs and the staged runner's explicit disposable opt-in.
The test additionally requires `GOQUENT_NATIVE_DISPOSABLE_FIXTURE=1`, which the
runner supplies only to native commands. Native/full/race use sequential dialects
and `-p 1`. No services are provisioned. Missing prerequisites remain skip or
unverified, never successful native plan acquisition.

| Group | MySQL 8.4 | PostgreSQL 16 | Evidence and finite boundary |
| --- | --- | --- | --- |
| Restricted role | Unique user with one column SELECT grant, zero direct broader SELECT grants; `PERFORMANCE_UNSUPPORTED` | Unique non-superuser, non-inheriting, non-bypass role with no memberships/table or column SELECT; `PERFORMANCE_QUERY` | Effective privilege queries precede collection. PostgreSQL can read catalogs but target EXPLAIN lacks SELECT permission. MySQL refuses incomplete visibility before target EXPLAIN. A direct SELECT grant on only the owned table supplies a same-principal positive control. This is not all privilege arrangements. |
| Custom operator | Not applicable: no PostgreSQL-style CREATE OPERATOR/opclass facility; no native pass claimed | Owned operator/function and B-tree opclass/index; `PERFORMANCE_UNSUPPORTED`; explicit Raw operator expression separately refused | Ordinary integer predicate reaches the custom-index catalog gate. A custom schema operator merely existing is not itself a refusal condition. The transaction fixes search_path and verifies bootstrap operators. This test does not modify pg_catalog or existing operator families. |
| Custom cast | Not applicable: no corresponding user-defined CREATE CAST; no native pass claimed | Owned composite destination type/function and bigint-to-owned-type assignment cast; `PERFORMANCE_UNSUPPORTED`; explicit Raw cast separately refused | The current column gate conservatively rejects a user cast touching a built-in integer, even if the target would not use that cast. No existing cast is replaced. Other sessions could observe this temporary catalog entry, so fixture tests must remain sequential. |
| Communication fault | Dedicated real connection and loopback proxy | Dedicated real connection and loopback proxy | After read-only Begin, the proxy forwards the version request and holds the real server response. Disconnect expects `PERFORMANCE_CLEANUP`; explicit cancel expects `PERFORMANCE_CANCELED`; deadline expects `PERFORMANCE_TIMEOUT`. These are failure-path successes, not acquired performance plans. |

## Nonexecution and cleanup evidence

PostgreSQL custom functions contain a nontransactional sequence marker. It is
unchanged before/after both refusals and after the normal control. A separate
request-framing proxy observes zero target dispatches during the refusals, then
one target EXPLAIN and zero target executions in the positive control after the
custom index/cast is removed. Thus a known SQL string match is not the sole
nonexecution evidence. Existing structural and catalog mock/spy boundaries remain
separate from these real-driver tests. No target query or EXPLAIN ANALYZE is run.

Each case uses cryptographically unique owned tables, schemas, roles and passwords.
Cleanup is registered immediately after creation; errors use fixed messages.
Only grants on owned objects to owned roles change. Missing role/schema creation
capability skips that case; later setup/cleanup or observation failures fail it.
The disposable fixture's administrative capabilities remain prerequisites for
custom opclass creation and full server session observation.

The proxy frames PostgreSQL and MySQL requests rather than assuming TCP packet
boundaries. A response-arrival channel establishes the injection point. Cancel
and deadline are followed by explicit transport teardown; this bounds drivers
which await a cancellation response. It does **not** prove that every driver
returns promptly against an indefinitely unresponsive peer without teardown.
The real server session/transaction must disappear within a bounded condition
poll, the pool must have zero connections in use, and all proxy workers/listeners
must close. The direct fixture must then collect a normal native control.
TLS, remote networks, arbitrary protocol corruption, faults at every catalog or
EXPLAIN phase, and all server-side cancellation implementations remain unverified.
Closed-pool and pool-acquisition-timeout tests are distinct from these real
communication faults.

Every expected failure requires the exact existing error identity, fixed public
formatting, no wrapped driver cause, and no returned performance evidence.
Metadata unknown is never promoted to a complete plan. Normal controls require
native provenance and known metadata; this is finite collection evidence, not
execution authorization. An empty MySQL target initially yielded unknown plan
metadata in development; the control fixture was populated instead of weakening
the assertion or changing parser/runtime behavior.

## Validation and residuals

Fresh local collection on this implementation used Go 1.25.0 linux/amd64
(`GOTOOLCHAIN=local`), MySQL 8.4.6 and PostgreSQL 16.10. The preliminary default
Go selector reported 1.26.4; that is not the local collection's toolchain.

| Check | Actual result | Collector interpretation |
| --- | --- | --- |
| Full | Exit 0; 3738 pass, 0 fail, 1 testskip; 24 passing packages, 30 no-test | skip |
| Race | Exit 0; 3738 pass, 0 fail, 1 testskip; 24 passing packages, 30 no-test | skip |
| Dedicated native stage | Exit 0; 25 pass, 0 fail, 0 testskip; 2 packages, 0 no-test | pass |
| Added boundary suite in each of full/race/native | 13 pass, 0 fail, 0 testskip, including all 10 added leaf cases | Actual native execution, not compilation alone |
| Vet / regeneration | Exit 0 each | pass |
| Canary | Shell positive/negative checker exit 0; 47 Go test passes, 0 fail/skip; 4 packages | pass |
| Runner | Exit 0; 12 orchestration tests | mock/orchestration evidence only |
| Formatting / diffcheck | Exit 0 | Local formatting evidence |

The full/race skip is the separate opt-in local type-loading census. The
collector completed with exit 1 and release blocked; passing command exits do
not discharge its fixed blockers. Ordinary/clean exact strict/manifest and CI
are recorded separately against the final commit, rather than inferred here.

Fresh final stage results, actual exits, collector status, Go/DB versions and
exact head/CI checks are recorded in the PR and RelayWeft report. Raw logs and
source evidence stay in protected local storage outside the repository; no
sensitive artifact upload is added. Runner evidence requires each added leaf
case to pass, not just a compiled suite or passing parent. MySQL operator/cast
N/A does not create artificial passing or skipped native test events.

All fixed staged-runner BLOCKERS remain. Review risk/precision failure and the
historical comparison with 1474 unknown findings, manifest generated_code/database
skips, historical testmain/expiry causes, and separate human release review are
not discharged by these tests. No threshold, suppression, baseline, API, wire,
hash, authorization or production runtime change is introduced. Whole-Issue
acceptance, merge, Issue closure and release publication remain separate.
