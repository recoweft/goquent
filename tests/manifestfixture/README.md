# Finite manifest evidence companion

This companion uses the current `examples/ai-safe-orm/schema.json` and
`policies.json`. `users_generated.go` is actual typed repository generator output;
the example's handwritten `main.go` is **not** generated code. The example does
not call this repository. Its original manifest and schema/policy-only stage stay
unchanged, including their two skipped axes. Companion success does not discharge
that example's missing evidence or any release blocker.

## Independent expectations

From the repository root, explicit maintenance regeneration is:

```sh
go run ./tests/manifestfixture/generate
go run ./cmd/goquent manifest check-repositories --config tests/manifestfixture/repositories.json
```

The maintenance command calls `manifest.GenerateRepositorySkeleton` with the
example manifest, typed mode, package `manifestfixture`, table `users`, row
`UserRow` and repository `UserRepository`. It saves that generator input separately
from the expected verification manifests. `repositories.json` checks the same
options against committed source without writing anything. The two committed
verification manifests bind the canonical example schema/policy, the generated
file, and the respective declared database artifact. Code hashing includes the
literal relative path `tests/manifestfixture/users_generated.go`; use the repo
root (or the runner's equivalent private tree). Verification never regenerates
expected schema/policy/code fingerprints. Review regeneration as an intentional
expectation change, never as a prerequisite for a stale check.

`declaration.go` defines DDL and its expected reader representation without a DB
connection. `mysql-database.json` and `postgres-database.json` are separately
reviewable serializations, not exported live snapshots. Columns/index follow the
example; the companion table/index names identify the isolated fixture. MySQL
uses `varchar(n)` and `timestamp`, PostgreSQL's reader uses `character varying`
and `timestamp without time zone`. Index raw definitions, null ordering, catalog,
namespace, coverage and source labels are deliberately different. No current
schema names, types, definitions or unknowns are normalized away.

These finite expectations pin MySQL 8.4.6 and PostgreSQL
16.10 (Debian 16.10-1.pgdg13+1), including the version strings that the reader
hashes. A different server version is a mismatch, not automatically accepted.
Changing an expectation requires independent DDL/catalog reasoning and review.
Column/default visibility remains partial/unknown where reported by the reader;
fingerprint equality does not turn those coverage fields into complete knowledge.

## Run separately scoped evidence

```sh
python3 scripts/release_validation/run.py --output /tmp/goquent-manifest-new \
  --stages manifest,manifest-evidence
# With the existing three disposable fixture DSNs explicitly exported:
python3 scripts/release_validation/run.py --output /tmp/goquent-manifest-native-new \
  --native-disposable-fixture --stages manifest-native
```

`manifest-evidence` is a nonconnecting **supplied snapshot** regression matrix.
`manifest-native` additionally requires the existing Compose fixture, explicit
opt-in and all three exact fixture DSNs. It runs `go test -json -p 1 -count=1
./tests/manifestfixture -run '^TestManifestEvidenceNative$' -args
-manifest-evidence-output OUTPUT` sequentially for MySQL then PostgreSQL. No new
service is provisioned. The native test allocates a random owned table and index,
instantiates the declared expected DB schema **before any live read**, and binds
only that database fingerprint on a copy of the committed expected manifest.
Code/schema/policy expectations are never refreshed. It creates only its owned
objects, uses `migration.ReadSchema` through a read-only surface, exports current,
then adds a canary column and separately exports drift. Cleanup drops only the
successfully created owned table. No production or pre-existing business object
is changed. Ordinary tests skip this collection without the opt-in/three DSNs.

For each dialect the runner invokes the built CLI (not `go run` exit wrapping):

```sh
goquent manifest verify --format json --manifest EXPECTED_MANIFEST \
  --schema examples/ai-safe-orm/schema.json \
  --policy examples/ai-safe-orm/policies.json \
  --code tests/manifestfixture/users_generated.go \
  --against-db --database-schema CURRENT_SCHEMA
```

The native expected manifest and current/drift schema files live only in the
protected output directory. The CLI compares supplied inputs; `--against-db`
does not connect. The runner first checks actual repository regeneration, requires
both native test leaves when selected, and requires exactly four unique `ok`
checks with real exit 0 for each positive. Exit 0 alone cannot pass a case.
`cases.json` retains each case's actual exit, check statuses and verification
classification separately from its regression outcome. `commands.jsonl` maps
actual commands to exits/raw logs; `provenance.json` records input origins.

| Case, per dialect | Real CLI exit | Verification classification |
| --- | --- | --- |
| four-axis | 0, all four `ok` | pass within that stage's supplied/native scope |
| code-drift | 1, generated_code stale | fail |
| database-drift | 1, database stale | fail; native uses a second live export |
| schema-content / policy-content | 1, respective axis stale | fail despite retained expected hash |
| missing-code | 1, generated_code stale | fail |
| database-disabled | 0, database skipped | skip |
| missing-database with against-db | 2, no checks | fail |
| invalid-manifest / unreadable-input | 2, no checks | fail |
| missing-fingerprints, both optional inputs absent | 0, both optional axes skipped | skip |
| missing-stored-code, current supplied | 1, generated_code stale | fail |

A successful negative regression means the expected refusal was observed; the
verification itself remains fail/skip. Both optional fingerprints absent may
skip; one absent is stale. Schema/policy also require Validate/content agreement,
so the optional skip rule does not generalize to them. Native not selected is
not-run; missing prerequisites without invocation is unverified; explicit skip
is skip; invoked failing commands retain fail and their real exit.

Raw source, schema, names, paths, SQL, version strings, errors and DSNs are private
local evidence: new repo-external mode-0700 directory, mode-0600 data files, no
upload. Public summaries expose only fixed stage/classification, counts and exits.
Canary cases check source/type/policy/path/version values do not appear in CLI
output. The runner tests the malformed/duplicate/incomplete-view boundaries and
actual CLI cases. CLI/MCP remain read-only; no runtime, API, wire, semantic version,
hash algorithm, threshold or baseline changes are made.

Agreement is not physical DB identity, live freshness, revocation, commit success
or business authorization. Generated code fingerprint equality is not business
permission. Fixed BLOCKERS and human release review remain in force; broader
native performance and the historical testmain/expiry causes remain unverified.
