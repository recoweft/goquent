# GQ-AI-02/PR1 validation record

Environment: `/home/murai/github/goquent`, WSL2, Ubuntu 24.04.4,
Go `go1.26.4 linux/amd64`. Baseline main:
`0d08f5233e55015aac6e5b2eb626e8d92c6c3202` (merged #56).
MySQL 8.4.6 and PostgreSQL 16.10 were running as the repository's Compose services
on localhost ports 3306/5432. Shared DB commands ran serially.

## Measured results

| Command | Result |
| --- | --- |
| `go test ./orm/query ./orm/internal/querybuilder/... ./orm/internal/valuecopy ./tests/contracts ./orm/review ./orm/manifest ./orm/operation ./orm/mcp ./cmd/goquent -count=1` | PASS on the final implementation; no DB-dependent test claimed by this command |
| `TEST_MYSQL_DSN='root:password@tcp(127.0.0.1:3306)/testdb?parseTime=true' TEST_POSTGRES_DSN='postgres://postgres:password@127.0.0.1:5432/testdb?sslmode=disable' go test ./tests -run '^TestPredicateTreeDatabaseSemantics$' -count=1 -v` | PASS in both DBs, including empty-IN rejection and nested UPDATE/DELETE in external sql.Tx |
| `go test ./... -json` | Final run FAIL: 861 pass, 2 fail, 1 skip test/subtest events. Both failures are the TransactionHooks column-order mismatch below. `TestOpenWithRegisteredDriver` skips without TEST_DB_DSN. Earlier development all-package runs passed; this final failure is not hidden by them. |
| `TEST_DB_DSN='root:password@tcp(127.0.0.1:3306)/testdb?parseTime=true' GOFLAGS=-json make test-integration` | Final run FAIL: 863 pass, 1 fail, 0 skip test/subtest events. Both DBs and registered custom driver actually ran. Only `TestRunTransactionWithHooksCommitsAuditHook` failed. Earlier integration runs passed; this final failure remains reported. |
| `go run ./cmd/goquent review --fail-on high ./...` | Exit 1: 228 Medium, 44 High, 9 Destructive, 11 Blocked findings; precision 225 precise / 33 partial / 34 unsupported. These include existing examples/tests and the new integration fixture's explicit DDL/seed/cleanup, plus partial reconstruction of new test helpers. No suppression or safety claim. |
| `go run ./cmd/goquent manifest verify --manifest examples/ai-safe-orm/goquent.manifest.json --schema examples/ai-safe-orm/schema.json --policy examples/ai-safe-orm/policies.json` | Exit 0; schema and policy match, aggregate fresh=true. generated_code and database fingerprints absent: both skipped/unverified. |
| `git diff --check` | PASS |

The five existing example JSON files were unchanged. Their baseline hashes,
loading and semantic review/diagnostic transport pass in `tests/contracts`.
Development test compilation errors and a sandbox cache-write failure were
corrected/retried; they are not counted as final successful test results.

## TransactionHooks investigation (unresolved, not fixed here)

The failing SQL lists `name,id`; sqlmock expects `id,name`. Both hook tests call
the generic `Insert` path, whose `orm/write.go:buildInsertStatement` ranges over
`meta.FieldsByName` (a map). That path and `orm/transaction_hooks.go` /
`orm/transaction_hooks_test.go` are byte-for-byte unchanged from main. They do not
use the condition-tree SELECT/write-plan pipeline.

To compare without changing the working branch, a Go `-overlay` replaced all
modified existing Go files with `git show origin/main:<path>` and hid newly added
Go files. It was run from the same repository directory.

| Command | Commit hook | Rollback hook |
| --- | --- | --- |
| `go test -overlay=/tmp/gq-pr1-baseline-overlay.json ./orm -run '^TestRunTransactionWithHooks' -count=20 -json` (main source) | 17 pass, 3 fail | 20 pass |
| `go test ./orm -run '^TestRunTransactionWithHooks' -count=20 -json` (PR1 source) | 15 pass, 5 fail | 18 pass, 2 fail |

All observed failures have the same INSERT column-order mismatch. The rollback
failure shares the unchanged Insert/expectation path; it was observed on this
branch, but not in the 20 baseline trials. The sample counts are not evidence of a
statistically established frequency change. This PR does not fix, skip or relax
those tests. CI results must be reported independently against its actual head.

## Limits and review status

The tests establish the bounded representation and SQL/parameter/result behavior
in [predicate-tree.md](predicate-tree.md), not PR2 cardinality or Strict safety.
Raw expressions, custom mutable values, JOIN semantics, subqueries and UNION
branches retain explicit analysis limits. Snapshots are not executable-plan
attestations. No benchmark speedup or broad performance guarantee was measured.

The PR body records its head SHA, GitHub CI URLs/status, and actual Issue/PR links.
Implementation/test evidence, PR creation and merge status are separate. No merge
or Issue closure is performed by this work.
