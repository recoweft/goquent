# Contribution Guidelines

Goquent is a Go ORM library, query builder, and database review tool. Its current
layout is not an application-specific Onion Architecture under `internal/`.

## Packages and dependencies

```text
cmd/goquent/               CLI: review, migration, manifest, OperationSpec, MCP
orm/                       Public DB/Executor, generic CRUD, scopes and helpers
orm/query/                 Public query DSL, plans, risk and policy checks
orm/internal/querybuilder/ Internal SQL builders, dialects and snapshots
orm/internal/stringutil/   Internal naming helpers
orm/driver/                database/sql connections, transactions and dialects
orm/model/, conv/, scanner/ Model mapping, conversion and row scanning
orm/review/, migration/    Static review and migration planning
orm/manifest/, operation/  Schema context and read-only OperationSpec compilation
orm/mcp/                   Read-only context and planning server
tests/                     Public API and MySQL/PostgreSQL integration tests
examples/, docs/, specs/   Runnable examples, usage guides and design contracts
```

- Keep reusable public APIs under `orm/` and its public subpackages. Implementation
  details belong under the relevant `internal/` boundary; do not expose internal
  builder types through public signatures or callbacks.
- Lower-level packages must not import the root `orm` facade or CLI. Keep SQL
  generation and structural inspection separate from connections and execution.
  Preserve Go's acyclic package dependencies and inject compatible executors.
- The SQL builder is in this repository; do not restore the external
  `goquent-query-builder` module dependency.
- `go.mod` is the dependency inventory: production drivers are
  `github.com/go-sql-driver/mysql` and `github.com/lib/pq`; tests also use
  `github.com/DATA-DOG/go-sqlmock`. `filippo.io/edwards25519` is indirect.
  Prefer the standard library. Explain additional dependencies and update
  `go.mod`/`go.sum` with relevant tests; a TODO alone is not dependency review.

## ORM and review contracts

- MySQL and PostgreSQL are both implemented and supported. Preserve their distinct
  placeholder, identifier, conflict, RETURNING and affected-row semantics.
- Read [the API inventory and contracts](specs/goquent_ai_contracts_v3.md) before
  changing an execution or inspection path. It distinguishes current behavior from
  requirements assigned to later GQ-AI issues. Strict safety profiles described
  there are requirements, not a currently available runtime option.
- Preserve `sql.DB`, `sql.Tx`, custom `Executor`, driver registration, struct/map
  scanning, generic helpers and bool compatibility. External executor wrappers do
  not imply ownership or the ability to start another transaction.
- Plans must not execute SQL. Explicit live schema reads are separate operations.
  A low risk level, `precise`, predicate column name, reason or suppression is not
  authorization or proof that all conditions were checked.
- Generic CRUD/RETURNING and scoped destination helpers share the Query pipeline;
  see docs/planned-query-execution.md. PR3 adds private compound preflight and Raw binding, a source-breaking Row
  wrapper, and Strict refusal of opaque recipes/non-nil nested scopes. Preserve
  the explicit support limits; statement gates do not prove arbitrary callbacks
  or complete collection replacement safe.
- AI/MCP workflows must remain read-only for database execution. Migration apply
  remains a human-controlled deployment operation.
- Use `OpenWithDriver(orm.MySQL, dsn)` or `OpenWithDriver(orm.Postgres, dsn)`.
  The legacy `Open(dsn)` selects MySQL. `db.Model(value)` creates a query;
  package-level `orm.Model(value)` declares policy. Use
  `db.Transaction(func(tx orm.Tx) error { ... })` or its context-aware variant.
- Minimize unnecessary reflection and preserve connection pooling. Keep any
  `unsafe` use narrowly justified. Preserve documented error identities and
  `errors.Is`/`errors.As` behavior; do not claim every driver error is normalized.

## Toolchain and validation

- `go.mod` declares Go `1.25.0` and preferred toolchain `go1.26.4`. Both GitHub
  workflows bootstrap Go `1.25.9`; Go's toolchain selection may download/use
  `go1.26.4`. Record `go version` when reporting tests.
- Run `go test ./...`. Run `make test-integration` for the repository's MySQL 8
  and PostgreSQL 16 container-backed suite. Explicit `TEST_MYSQL_DSN` and
  `TEST_POSTGRES_DSN` make connection failures fail rather than skip. Report skips,
  failures and unavailable environments separately from passing packages.
- Follow `.github/PULL_REQUEST_TEMPLATE/goquent-database-review.md`: run the review
  CLI and relevant manifest verification. Report existing findings honestly;
  partial/unsupported analysis is not a successful safety proof.
- Documentation publishing uses `make docs` on main. Do not regenerate unrelated
  API documentation for a hand-written specification change.
- Keep source comments and new documentation in English and run `gofmt` on changed
  Go files. Preserve existing benchmark coverage; performance targets require
  measured evidence and are not present guarantees.
- Do not add intentionally failing tests for future safety requirements. Record
  missing guarantees in the contracts and add executable regression coverage in
  the issue implementing the guarantee.
