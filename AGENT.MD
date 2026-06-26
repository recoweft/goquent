# Contribution Guidelines

This repository follows the Onion Architecture in Go. The high-level directory layout is as follows:

```
./cmd/        - Entry points
./internal/   - Application code
    ├── domain        - Business logic and type definitions
    ├── usecase       - Application use cases
    ├── infrastructure- Implementations of repositories or external systems
    └── interface     - HTTP handlers or other adapters
```

## Layer Responsibilities

- **Domain**: Define entities, value objects and repository interfaces. This layer contains pure business logic and must not depend on any other layer.
- **Usecase**: Describe application-specific workflows. Usecase packages depend only on domain interfaces.
- **Infrastructure**: Provide concrete implementations for DB access or external API calls. Infrastructure packages implement domain repository interfaces.
- **Interface**: Handle input/output (HTTP, CLI, etc.) and orchestrate use cases. Interfaces depend on usecases.

## Dependency Rules

- Dependencies point only from outer layers to inner layers.
- The domain layer is independent of usecase, infrastructure or interface layers.
- Use the Dependency Inversion Principle. Expose interfaces in inner layers and implement them in outer layers.
- Where applicable, inject dependencies via constructors or a dedicated DI mechanism.

This repository adheres to standard Go directory practices. Place new packages under `internal/` unless they are meant for external consumption.

## ORM Development Rules

The `orm` package implements a lightweight ORM using the internal SQL query
builder under `orm/internal/querybuilder`.
Follow these guidelines when extending or modifying this package:

- Design for MySQL support first, but keep the driver abstraction ready for
  future PostgreSQL support.
- Directory layout under `orm/`:

```
driver   - connection handling and SQL dialect interface
model    - struct to table/column mapping logic
query    - wrapper around the internal query builder and scanning
scanner  - fast conversion from `sql.Rows` to structs or maps
```

- Only `database/sql` and `github.com/go-sql-driver/mysql` may be used as
  dependencies. Add a TODO comment before introducing any other library.
- Optimize connection pooling via `sql.DB` settings and minimize reflection.
  `unsafe` may be used if necessary but keep readability in mind.
- API example:

```go
orm, _ := orm.Open(orm.MySQL, dsn)
user := new(User)
err := orm.Model(user).Where("id", 1).First(user)

var row map[string]any
err = orm.Table("users").Where("id", 1).FirstMap(&row)

var rows []map[string]any
err = orm.Table("users").Where("age", ">", 20).GetMaps(&rows)
```

- Support struct scanning, `map[string]any`, and `[]map[string]any`.
- Transactions should use `orm.Transaction(func(tx orm.Tx) error { ... })`.
- Provide end-to-end tests in `tests/` using a MySQL 8 container. `go test ./...`
  must pass with Go 1.25 or higher.
- Document usage, benchmark results (`go test -bench`), and PostgreSQL
  extension plans in `README.md`. All comments must be in English.
- Wrap SQL errors with package-level variables such as `orm.ErrNotFound` or
  `orm.ErrConflict`.
- Include benchmarks `BenchmarkScannerStruct` and `BenchmarkScannerMap` aiming
  for roughly 1.5x the speed of GORM's scanning.
