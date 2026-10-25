# Saved model repository and manifest evidence

This package owns the original `../goquent.manifest.json`'s saved string-tenant
model. GQ-MAINT-14 deliberately extends its former schema/policy-only declaration
with generated-code and declared database expectations. It does not retrospectively
turn the old two optional skips into four-axis evidence.

| Owner / input | Meaning | Evidence and limits |
| --- | --- | --- |
| Handwritten `../main.go` | Integer tenant, `datetime`, application nullable-known declarations | Offline planning/refusal only; not the saved model or generated code |
| `../schema.json`, `../policies.json` | String tenant, `timestamp`, omitted nullable-known | Saved schema/policy fingerprints; not live DB metadata |
| `generator-input.json`, `repositories.json`, `users_generated.go` | Actual typed repository generator output for the saved model | Nonwriting regeneration plus a hash of source bytes **and the repo-relative path** |
| `declaration.go`, `mysql-database.json` | Independent DDL/reader declaration of canonical `users` and `idx_users_tenant_id` | Original MySQL manifest's database expectation; supplied agreement is not a live read |
| `postgres-database.json`, `postgres.manifest.json` | Separate PostgreSQL reader declaration and expectation | Separate dialect result; never attributed to the original MySQL artifact |
| Native owned derivatives | Pre-read instantiation of table/index names and their context/raw definition references | Current catalog read, drift and cleanup; not verification of a live canonical `users` table |
| `tests/manifestfixture` | Existing finite companion with its own generated code/owned declarations | Kept separately; none of its passes count as original artifact evidence |

The generated `StoredUserRow.TenantID` is `string`. Its timestamp maps to `any`
under the existing generator rules; the source schema does not assert nullable
knowledge. Do not silently change it to the handwritten `User`'s integer tenant
and `*time.Time`. `NewStoredUserRepository(db)` constructs the repository for an
explicit caller-owned DB; construction is not authorization. The typed columns,
patch and read methods are generated from this package's fixed input. Neither
ordinary example startup nor verification calls them or connects to a database.

From the repository root, verify without changing expectations:

```sh
go run ./cmd/goquent manifest check-repositories --config examples/ai-safe-orm/storedmodel/repositories.json
go run ./cmd/goquent manifest verify --format json \
  --manifest examples/ai-safe-orm/goquent.manifest.json \
  --schema examples/ai-safe-orm/schema.json \
  --policy examples/ai-safe-orm/policies.json \
  --code examples/ai-safe-orm/storedmodel/users_generated.go \
  --against-db --database-schema examples/ai-safe-orm/storedmodel/mysql-database.json
```

The built CLI's exact exit is authoritative (`go run` wraps nonzero exits).
`check.sh` and the runner use the built CLI and require four unique `ok` checks
and exit 0. Hashing uses exactly
`examples/ai-safe-orm/storedmodel/users_generated.go` relative to the repo root.
Moving an identical file changes that fingerprint.

For intentional maintenance only, run
`go run ./examples/ai-safe-orm/storedmodel/generate` from the repo root, review
all changed expectations and commit them together. It builds schema/policy-only
generator input from the saved declarations using the existing fixed timestamp,
then calls the actual typed generator. It never embeds the code hash into its own
generator input. It writes the two declared database snapshots and expected
manifests from the independent declaration, without a DB read. This command is
**never** a verification prerequisite. Tests compare committed artifacts and
retain stale negatives; do not regenerate to conceal unexpected drift.

## Finite verification scopes

```sh
python3 scripts/release_validation/run.py --output /tmp/goquent-saved-new \
  --stages manifest,example-manifest-evidence
# Explicitly export the existing three disposable fixture DSNs first:
python3 scripts/release_validation/run.py --output /tmp/goquent-saved-native-new \
  --native-disposable-fixture --stages example-manifest-native
```

`manifest` verifies the actual original MySQL artifact with declared supplied
inputs. `example-manifest-evidence` records 24 real CLI cases per dialect, with
separate original MySQL and PostgreSQL results. Existing `manifest-evidence` and
`manifest-native` continue to address only the companion, 12 cases per dialect.
Each negative regression can pass while its verification remains fail or skip.

| Cases per dialect | Verification / real exit | Missing coverage addressed |
| --- | --- | --- |
| Four-axis positive | All four `ok`, 0 | Actual saved artifact with explicit current inputs |
| Code bytes/path, current DB/schema/policy drift | Stale, 1 | Change-sensitive comparison; policy flags can also stale schema |
| Stored schema/policy content/hash drift | Stale, 1 | Expectations stay fixed; no stale-to-green regeneration |
| Missing current code, missing stored code/DB hash | Stale, 1 | One-sided optional evidence |
| Omitted schema/policy | Stale, 1 | Required content remains checked |
| Disabled database check | Database skipped, 0 | Explicit omission is not a pass |
| Both optional expectations and inputs absent | Both skipped, 0 | Existing optional-axis contract preserved |
| Missing DB with `--against-db`, invalid manifest/schema/policy/DB, unreadable code/DB | Rejected, 2 | Real input rejection, no fabricated checks |
| Generator/input drift | Regeneration comparison fails | Generated artifact linked to saved owner |

Native tests require the already-running disposable Compose fixtures, all three
exact DSNs and explicit opt-in. They run with `-p 1`, MySQL then PostgreSQL, and
create only unique owned tables/indexes. No services are provisioned and canonical
`users` is never created or modified. The expected database hash is instantiated
**before connection** from the declared identities; schema/policy/code expectations
remain unchanged. Both dialects preserve their real type spellings, index raw
definitions, namespaces/catalogs and pinned versions (MySQL 8.4.6; PostgreSQL
16.10 Debian 16.10-1.pgdg13+1). Partial visibility/default unknowns stay unknown.

Each native run exports current and deliberate drift, then drops only its owned
table and observes absence, a subsequent `SELECT 1` control and pool close/count.
A separate subprocess deliberately fails an assertion after object creation; its
real exit 1 and fixed failure marker are required, followed by independently
recorded cleanup/absence/control/pool evidence. That child is a failed assertion,
not a passing verification. Timeouts, unobserved absence or cleanup errors fail.
These controls do not prove physical DB identity, rollback ACK or global health.

Raw artifacts/logs stay outside the repository and analysis roots in new 0700
folders, 0600 files and owner-only executables, without upload. Public summaries
contain only fixed classifications/counts/exits. Neither supplied equality nor a
native owned derivative proves main.go's DB suitability, freshness, authorization,
complete schema knowledge or release acceptance. Fixed BLOCKERS, closureeligible0,
MAINT03/loader-producer limitations, historical testmain/expiry and Issue123's
human release track remain unchanged.
