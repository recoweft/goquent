# Policy DSL Guide

Policy metadata lets Goquent review application-specific safety rules. Policies can be registered
from Go code or supplied to manifest generation as JSON.

```go
err := orm.Model(User{}).
    Table("users").
    TenantScoped("tenant_id").
    SoftDelete("deleted_at").
    RequiredFilter("tenant_id").
    PII("email").
    Register()
```

Policy checks:

- Tenant scope: queries should include the tenant column.
- Soft delete: select, update, and delete operations get a default `deleted_at IS NULL` filter.
- Required filters: configured columns must be present in predicates.
- PII: selecting configured columns emits a warning and should include an access reason.

Registered policies are checked for every table visible in a `QueryPlan`, including joined tables.
When more than one policy table requires the same predicate column, qualify the predicate so the
scope is unambiguous:

```go
err := orm.RegisterTablePolicy(orm.TablePolicy{
    Table:        "users",
    TenantColumn: "tenant_id",
    TenantMode:   orm.PolicyModeBlock,
})
if err != nil {
    return err
}
err = orm.RegisterTablePolicy(orm.TablePolicy{
    Table:        "memberships",
    TenantColumn: "tenant_id",
    TenantMode:   orm.PolicyModeBlock,
})
if err != nil {
    return err
}

plan, err := db.Table("users").
    Select("users.id").
    Join("memberships", "users.id", "=", "memberships.user_id").
    Where("users.tenant_id", tenantID).
    Where("memberships.tenant_id", tenantID).
    Limit(50).
    Plan(ctx)
```

Use `RequiredFilter` for parent scopes that must always be visible alongside tenant scope:

```go
err := orm.Model(FilingCase{}).
    Table("filing_cases").
    TenantScoped("tenant_id", orm.PolicyModeBlock).
    RequiredFilter("client_company_id", "workplace_id").
    PolicyMode(orm.PolicyModeBlock).
    Register()
```

Soft delete helpers:

```go
active, _ := db.Table("users").Select("id").Plan(ctx)
all, _ := db.Table("users").WithDeleted().Select("id").Plan(ctx)
deleted, _ := db.Table("users").OnlyDeleted().Select("id").Plan(ctx)
_, _, _ = active, all, deleted
```

PII access reason:

```go
plan, err := db.Table("users").
    Select("id", "email").
    Where("tenant_id", tenantID).
    AccessReason("support ticket TICKET-123").
    Limit(1).
    Plan(ctx)
```

Policy modes are `warn`, `enforce`, and `block`. `enforce` raises missing policy predicates to high
risk. `block` prevents execution.

For AI workflows, export policy metadata into a manifest and require AI-generated operations to
compile against that manifest.
