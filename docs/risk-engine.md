# RiskEngine Guide

The risk engine classifies structural database risk. It is deterministic and local; it does not
call the database and it does not make authorization decisions.

Built-in risk levels:

- `low`: no active structural warning.
- `medium`: review recommended, usually suppressible.
- `high`: explicit approval is required before execution.
- `destructive`: destructive DDL or similarly dangerous shape.
- `blocked`: execution must not proceed.

Common warning codes:

- `LIMIT_MISSING`: select list query has no limit.
- `SELECT_STAR_USED`: selected data is harder to review.
- `RAW_SQL_USED`: Goquent cannot fully inspect the query.
- `UPDATE_WITHOUT_WHERE`: blocked.
- `DELETE_WITHOUT_WHERE`: blocked.
- `BULK_UPDATE_DETECTED`: update predicate is not primary-key-like.
- `BULK_DELETE_DETECTED`: delete predicate is not primary-key-like.
- `DESTRUCTIVE_SQL_DETECTED`: destructive DDL token was detected.
- `WEAK_PREDICATE`: predicate such as `1=1`.

You can run the engine directly:

```go
result := orm.DefaultRiskEngine.CheckQuery(plan)
```

Broad write detection is conservative. Without table key metadata, only `id` and qualified `*.id`
predicates are treated as narrow. Foreign-key-looking columns such as `tenant_id` are not treated
as primary-key-like by name alone. When review has manifest metadata, complete primary-key or
unique-index predicates can be treated as narrow, including composite keys.

Risk rules can be customized for local policy:

```go
high := orm.RiskHigh
engine := orm.NewRiskEngine(orm.RiskConfig{
    Environment: "ci",
    Rules: map[string]orm.RiskRuleConfig{
        orm.WarningLimitMissing: {
            Severity: &high,
        },
    },
})
_ = engine
```

Do not use `RiskLow` as business approval. It only means Goquent did not find a risky database
shape.
