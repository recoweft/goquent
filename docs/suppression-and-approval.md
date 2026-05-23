# Suppression and Approval

Goquent separates suppression from approval.

Suppression hides a specific suppressible warning when the team has a narrow reason. It should have
an owner and an expiration date when possible.

```go
plan, err := db.Table("users").
    Select("id", "name").
    Where("tenant_id", tenantID).
    SuppressWarning(
        orm.WarningLimitMissing,
        "bounded by tenant export job",
        orm.SuppressionOwner("data-platform"),
        orm.SuppressionExpiresAt(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)),
    ).
    Plan(ctx)
```

Inline suppressions are supported by the review CLI:

```go
// goquent:suppress LIMIT_MISSING reason="bounded tenant export" expires="2026-07-01" owner="data-platform"
err := db.Table("users").Where("tenant_id", tenantID).Get(&rows)
```

Config suppressions are supported by `goquent review --config`:

```json
{
  "suppressions": [
    {
      "code": "LIMIT_MISSING",
      "path": "internal/admin/export.go",
      "reason": "admin export is bounded by caller authorization and audit logging",
      "owner": "data-platform",
      "expires": "2026-08-01"
    }
  ]
}
```

Suppression cannot hide non-suppressible findings such as blocked updates or deletes without a
predicate.

Approval records that a risky operation is intentionally allowed:

```go
_, err := db.Table("users").
    Where("tenant_id", tenantID).
    RequireApproval("backfill inactive flag for tenant migration").
    Update(map[string]any{"inactive": true})
```

High and destructive risk require approval before execution. Blocked operations remain blocked.

Review guidance:

- Prefer fixing the query over suppressing a warning.
- Suppress only one code at a time.
- Include a reason that explains why the warning is acceptable.
- Use expiration dates for temporary exceptions.
- Use approval for intentionally risky execution, not for hiding noise.
