# Goquent AI-safe ORM Roadmap / Spec & Tasks v2

作成日: 2026-04-24  
状態: Draft v2  
対象: Goquent を「AI が生成した DB コードを安全にレビュー・検証・制御できる ORM」に進化させるための仕様・タスク整理  
変更理由: 実際に Goquent を使うコーディングエージェント視点の「得 / 損」レビューを反映し、実装負荷・誤検知・manifest stale・静的解析限界を仕様に組み込む。

---

## 0. v2 での主要変更

v1 の方向性は維持する。Goquent の独自ポジションは引き続き **AI-safe ORM**、つまり **AI が生成した DB コードをレビュー可能・説明可能・制御可能にする ORM** とする。

ただし v2 では、以下を重要仕様として追加・修正する。

### 0.1 追加した仕様

- `Manifest` に stale detection を追加する
  - schema / generated code / manifest / 実 DB のズレを検知する。
  - AI が古い manifest を信じてしまうリスクを減らす。
- warning suppression と approval reason を早期フェーズに移動する
  - 誤検知・過検知による review fatigue を避ける。
  - suppress には理由、対象範囲、有効期限を持たせる。
- `RiskLow` の意味を明確化する
  - `RiskLow` は「業務的に安全」ではない。
  - あくまで「DB 操作の形として低リスク」という意味に限定する。
- static review の限界を明文化する
  - precise / partial / unsupported の三段階に分ける。
  - 復元できない動的 query は、無理に確定的に判断せず fallback warning を出す。
- `OperationSpec` の MVP を狭くする
  - 最初は read-only `select / filter / order / limit` に限定する。
  - join / aggregate / subquery / mutation は後段に送る。
- MCP は現時点の scope を read-only に限定する
  - 初期 MCP は context provider / review tool provider に限定する。
  - DB write、migration apply、raw SQL execution は初期実装では提供しない。

### 0.2 フェーズ順の修正

v1 では `Manifest` を `Review CLI` より前に置いていたが、v2 では以下の順に変更する。

```text
MVP-1: QueryPlan
MVP-2: RiskEngine + Approval/Suppression
MVP-3: Minimal Policy DSL: tenant / soft delete / PII
MVP-4: goquent review
MVP-5: MigrationPlan / Migration Review
MVP-6: Manifest + Stale Detection
MVP-7: OperationSpec MVP: read-only select only
MVP-8: MCP Server: read-only context/review tools
```

理由は、`goquent review` が早く見える方が、人間にも AI にも Goquent の価値が伝わりやすいため。Manifest は重要だが、古い manifest が危険になるため、stale detection とセットで少し後に置く。

### 0.3 Documentation status taxonomy

This roadmap contains current features, human-controlled features, future ideas, prohibited
boundaries, and non-normative API sketches.

README and user-facing docs must follow this rule:

- Current features may be documented as product capabilities only when implemented, tested, and represented by examples.
- Human-controlled features, such as migration apply and raw SQL escape hatches, must not be included in AI or MCP workflows.
- Future or optional features must not appear in the README main path.
- Prohibited actions may be documented only as safety boundaries.
- API sketches are not public API until the implementation matches them.

---

### 0.4 Implementation audit (GQ-AI-01/PR1)

Audited against main `8c41e211336c97f9f33c950b3ebc302df05b90e3` for
[Issue #52](https://github.com/recoweft/goquent/issues/52).
The task checkboxes below are historical planning lists, not a live completion
ledger. This audit and [contracts v3](goquent_ai_contracts_v3.md) distinguish the
current implementation from required future guarantees. “Implemented” applies
only to the named feature, not its entire phase or all database entry points.
Test execution outcomes are recorded separately in the PR/report.

| Phase / feature | Status | Source/test evidence and remaining gap |
| --- | --- | --- |
| 1: QueryPlan structs, SQL/params, JSON/text output, non-executing DSL SELECT/INSERT/UPDATE/DELETE plans | Implemented | `orm/query/plan.go`, `query.go`, `plan_test.go`; plan snapshot and recording-executor assertions exist |
| 1: Every execution uses the same complete plan | Partial | Query terminals generally plan/gate; generic writes in `orm/write.go` bypass QueryPlan; PostgreSQL InsertGetId omits gate, scoped RETURNING appends SQL after checking. GQ-AI-04/05 |
| 1: Semantic metadata for all expressions/subqueries and versioned executable plans | Partial | Builder snapshots and flat PredicateRef exist; no complete condition proof or operation/version/executor binding. GQ-AI-02/05 |
| 2: Built-in risk rules, block checks, reason/expiry approval, inline/API suppression | Implemented | `orm/query/risk.go`, `suppression.go`, `risk_test.go`; structural heuristics only, not external authorization |
| 2: Configurable review rules/suppressions | Implemented | `orm/review/config.go`, `suppression_lint.go`, CLI config handling/tests; JSON review config exists, the unified YAML draft in section 9 does not |
| 2: Fail-closed Strict profile and trusted high-risk external permission | Unimplemented | Gate consumes mutable plan fields/reason; no unified Strict profile/provider. GQ-AI-03/05/13 |
| 3: Policy declaration, registry, soft-delete defaults, PII and required-column diagnostics | Implemented | `orm/policy.go`, `orm/query/policy.go`, corresponding policy tests; policy applicability excludes INSERT and raw operations |
| 3: Tenant value/condition proofs, DB-isolated policy and uniform enforcement | Partial | Global registry and column-presence checks; values/OR/NOT/conflict branches not proved. GQ-AI-02/03/04 |
| 4: CLI review of Go/SQL/plan JSON, pretty/JSON/GitHub output, manifest/precision gates | Implemented | `orm/review/*`, `cmd/goquent/main.go` and tests; current-input manifest gate rejects missing/unverified context |
| 4: Type-aware exhaustive static review | Partial | `orm/review/go.go` matches AST method/chain shapes, not complete receiver identity/data flow; plan JSON may retain supplied warnings. GQ-AI-05/09 |
| 4: Baseline adoption and SARIF | Unimplemented | No implemented baseline flag; `TestReviewCommandRejectsBadFormat` rejects SARIF. These remain optional future features, not new PR1 work |
| 5: Migration SQL/step plans, schema diff, risk, dry-run/apply, live schema/status/drift/backfill guidance | Implemented | `orm/migration/{plan,parser,schema,schema_read,status,drift,review_mode}.go`, tests and CLI; apply is human-controlled |
| 5: Complete dialect/schema semantics and verified preflight evidence | Partial | Supported parser/schema shapes and textual guidance do not prove arbitrary migration/data effects or evidence authenticity. GQ-AI-11 |
| 6: Manifest v1, schema/policy/code/database fingerprints, load/validate/verify | Implemented | `orm/manifest/*`, tests, `examples/ai-safe-orm/goquent.manifest.json`; verification compares supplied inputs |
| 6: Uniform freshness/trust and public-data sanitization | Partial | Review fresh gate exists; OperationSpec accepts absent attached verification; serialization is not a universally redacted public view. GQ-AI-03/05/06/07/11 |
| 7: Narrow read-only OperationSpec, schema, manifest fields/filter/PII/ref checks and compiler | Implemented | `orm/operation/{spec,schema}.go`, `spec_test.go`, CLI/example; unknown value_ref rejected |
| 7: Complete typed values, trusted tenant binding and structured diagnostics | Partial | Field/operator/ref presence checks are not full type or tenant-equality proof. GQ-AI-02/03/07 |
| 8: Read-only MCP resources/tools/prompts and exposure allowlists | Implemented | `orm/mcp/{server,jsonrpc}.go`, tests; fixture/skeleton tools return content; explain_query is not live EXPLAIN |
| 8: Sensitive metadata masking across all public output | Unimplemented | Manifest/plan/reason/evidence output is not a separate sanitized non-executable representation. GQ-AI-06 |
| 9: Guides, example, playbook and PR review template | Implemented | `docs/`, `examples/ai-safe-orm/`, `.github/PULL_REQUEST_TEMPLATE/goquent-database-review.md` |
| 9: Examples/release gates proving all new contracts | Partial | Existing examples show MVP paths; Strict, CRUD parity and new case guarantees require GQ-AI-14 after 01–13 |
| Cross-cutting typed-column/update code generation | Unimplemented | Generic helpers/model metadata exist, not the typed column API sketches. GQ-AI-08 |
| Cross-cutting idempotency, hooks, nested writes and row expectations | Partial | `orm/{write,idempotency,transaction_hooks,nested_write}.go` and tests implement recipes; no unified replay/cardinality/atomicity operation contract. GQ-AI-04/10 |
| Cross-cutting measured performance plans and query observation | Unimplemented | QueryPlan has EstimatedRows/UsesIndex fields, not a general live EXPLAIN/observation pipeline. GQ-AI-12 |

MySQL and PostgreSQL are both supported now; PostgreSQL is not a future extension.
The actual repository is an ORM/library and CLI (`orm/`, `cmd/goquent/`, `tests/`),
not an application with domain/usecase layers under a root `internal/`. The builder
is internal to `orm`. `go.mod` declares Go 1.25.0 with toolchain go1.26.4;
CI bootstraps 1.25.9 and runs both MySQL 8 and PostgreSQL 16. See the corrected
[AGENT.MD](../AGENT.MD) for dependencies and development rules.

PR1 adds documentation only. PR2 shared fixtures wait for PR1's main merge;
new runtime safety tests belong to their owning issue. Existing unchecked lists,
reason-based approvals and API sketches must not be advertised as current Strict
guarantees. Compatibility changes and common case IDs are tracked in contracts v3.

## 1. Vision

Goquent は、単なる CRUD ORM ではなく、**AI が生成した DB 操作を、型安全・説明可能・検証可能・承認可能にする ORM** を目指す。

中心となる価値は次の通り。

```text
SQL を隠す ORM ではなく、SQL を AI と人間の両方に説明する ORM。
AI に DB 操作を任せる ORM ではなく、AI の DB 操作を制御する ORM。
CRUD を楽にする ORM ではなく、DB 変更を安全にレビュー可能にする ORM。
```

### 1.1 Tagline

```text
The ORM that makes AI-generated database code reviewable.
```

日本語では以下。

```text
AI が生成した DB コードを、レビュー可能にする ORM。
```

### 1.2 v2 Positioning

v2 では、Goquent を以下のように位置づける。

```text
Goquent is an AI-safe ORM for Go.
It makes database operations explainable, policy-aware, reviewable, approval-gated, and staleness-aware.
```

特に `staleness-aware` を追加する。AI は機械可読な manifest を強く信じるため、Goquent は「その manifest が今も正しいか」まで扱う必要がある。

---

## 2. Design Principles

### 2.1 Plan-first

Goquent は、クエリやマイグレーションを即実行する前に、必ず `Plan` として可視化できるようにする。

```go
plan, err := query.Plan(ctx)
```

`Exec` は最終段階であり、開発時・レビュー時・CI では `Plan` を中心に扱う。

### 2.2 SQL-visible

ORM の DSL だけではなく、最終的に生成される SQL、パラメータ、対象テーブル、対象カラム、JOIN、WHERE 条件、LIMIT、リスクを見えるようにする。

### 2.3 Policy-aware

アプリケーション固有の DB ルールを ORM が理解する。

例:

- tenant scope
- soft delete
- PII
- required filter
- forbidden column selection
- destructive migration guard
- raw SQL guard

### 2.4 Deterministic first, AI second

Goquent の安全性は LLM に依存しない。  
AI は補助役であり、Goquent Core / Plan / Policy / Review は deterministic に動作する。

### 2.5 Human-reviewable

AI がコードを書く頻度が増えても、人間は最終的なレビュー責任を持つ。  
Goquent はレビュー時に「何を見るべきか」を明確に提示する。

### 2.6 Staleness-aware

Goquent は、manifest や generated code が古くなっている可能性を前提に設計する。

```text
古い manifest は、manifest がない状態より危険な場合がある。
```

したがって、manifest は生成するだけでなく、検証できなければならない。

### 2.7 Suppressible but accountable

warning は抑制できる必要がある。ただし、抑制は無言で行わせない。

抑制には原則として以下を要求する。

- warning code
- reason
- scope
- optional expiration
- optional owner / reviewer

### 2.8 Risk means structural DB risk, not business correctness

`RiskLow` は「安全」を意味しない。  
`RiskLow` は、Goquent が見える範囲において **DB 操作の形として低リスク** という意味に限定する。

業務的に危険な操作は `RiskLow` でもあり得る。

例:

```text
SELECT id FROM users WHERE id = ?
```

この query は形として低リスクでも、「その user をこの処理で取得してよいか」という業務判断までは Goquent には分からない。

### 2.9 Static analysis should not pretend to be complete

`goquent review` は Go の動的な query 組み立てを完全には復元できない。

そのため、review 結果には analysis precision を持たせる。

```text
precise:     Goquent query chain を復元でき、QueryPlan を生成できた
partial:     一部は復元できたが、条件分岐や helper により不完全
unsupported: 動的すぎて復元できない
```

unsupported を「問題なし」と扱ってはいけない。

### 2.10 MCP current scope is read-only

The current MCP scope is read-only. It exists to provide schema, manifest, policy, and review
context to AI coding agents.

Current MCP tools must not:

- perform DB writes
- apply migrations
- execute raw SQL
- grant approval for destructive operations
- bypass stale manifest checks

README must not imply that MCP write tools are available or planned as part of the current
workflow. Any future write-capable MCP tool would require a separate safety specification and
explicit opt-in design.

### 2.11 Raw SQL is an escape hatch

Raw SQL is allowed only as a human-controlled escape hatch when Goquent's structured API cannot
express the operation adequately.

Raw SQL must:

- produce a QueryPlan or review artifact when possible
- emit `RAW_SQL_USED`
- include a reason when required by configuration
- be included in PR review output

Raw SQL execution is not part of the AI/MCP workflow.

---

## 3. Non-goals

このロードマップでは、以下を主目的にしない。

- 自然言語から直接 SQL を実行する機能
- AI に本番 DB 操作を自律実行させる機能
- GORM / ent / sqlc / Bun の単純な代替
- すべての RDBMS 方言に初期段階から完全対応すること
- 本番 runtime で LLM を必須依存にすること
- 「SQL を完全に隠す」こと
- static analysis で Go の全 query 構築パターンを完全復元すること
- `RiskLow` を業務的安全性の保証として扱うこと
- OperationSpec を巨大な独自 query 言語にすること

---

## 4. Core Concepts

### 4.1 QueryPlan

クエリを実行前に説明する構造体。

```go
type QueryPlan struct {
    Operation         OperationType
    SQL               string
    Params            []any
    Tables            []TableRef
    Columns           []ColumnRef
    Joins             []JoinRef
    Predicates        []PredicateRef
    Limit             *int64
    Offset            *int64
    EstimatedRows     *int64
    UsesIndex          *bool
    RiskLevel          RiskLevel
    Warnings           []Warning
    RequiredApproval   bool
    Approval           *Approval
    AnalysisPrecision  AnalysisPrecision
    Metadata           map[string]any
}
```

### 4.2 MigrationPlan

schema diff や migration SQL を実行前に説明する構造体。

```go
type MigrationPlan struct {
    Steps             []MigrationStep
    RiskLevel         RiskLevel
    Warnings          []Warning
    RequiredApproval  bool
    Approval          *Approval
    SQL               []string
    Preflight         []PreflightCheck
    Metadata          map[string]any
}
```

### 4.3 RiskLevel

`RiskLevel` は DB 操作の形に対するリスク分類であり、業務的安全性の保証ではない。

```go
type RiskLevel string

const (
    RiskLow         RiskLevel = "low"
    RiskMedium      RiskLevel = "medium"
    RiskHigh        RiskLevel = "high"
    RiskDestructive RiskLevel = "destructive"
    RiskBlocked     RiskLevel = "blocked"
)
```

### 4.4 Warning

warning は、AI と人間が共通語として扱える問題単位。

```go
type Warning struct {
    Code          string
    Level         RiskLevel
    Message       string
    Location      *SourceLocation
    Hint          string
    Evidence      []Evidence
    Suppressible  bool
    RequiresReason bool
}
```

例:

```text
TENANT_FILTER_MISSING
SOFT_DELETE_FILTER_MISSING
PII_COLUMN_SELECTED
UPDATE_WITHOUT_WHERE
DELETE_WITHOUT_WHERE
SELECT_STAR_USED
LIMIT_MISSING
RAW_SQL_USED
DESTRUCTIVE_MIGRATION
MANIFEST_STALE
STATIC_REVIEW_PARTIAL
STATIC_REVIEW_UNSUPPORTED
SUPPRESSION_EXPIRED
```

### 4.5 AnalysisPrecision

`goquent review` や静的解析が、どの程度確実に query を復元できたかを表す。

```go
type AnalysisPrecision string

const (
    AnalysisPrecise     AnalysisPrecision = "precise"
    AnalysisPartial     AnalysisPrecision = "partial"
    AnalysisUnsupported AnalysisPrecision = "unsupported"
)
```

意味:

| Precision | 意味 | 代表例 |
|---|---|---|
| `precise` | QueryPlan を生成できた | 単純な fluent chain |
| `partial` | 一部の table / operation / where だけ分かった | helper wrapper、条件分岐 |
| `unsupported` | 静的に復元できない | interface 越し、関数ポインタ、複雑な動的構築 |

### 4.6 Approval

危険操作を実行・適用するための明示的な承認情報。

```go
type Approval struct {
    Reason     string
    Scope      string
    CreatedBy  string
    CreatedAt  time.Time
    ExpiresAt  *time.Time
}
```

approval は「警告を消す」ものではない。  
approval は、危険性を認識したうえで実行を許可する情報である。

### 4.7 Suppression

誤検知や正当な例外を抑制するための情報。

```go
type Suppression struct {
    Code       string
    Reason     string
    Scope      SuppressionScope
    Location   *SourceLocation
    ExpiresAt  *time.Time
    Owner      string
}
```

suppression は原則として review 出力から warning を隠す。ただし JSON 出力には suppressed findings として残す option を持たせる。

### 4.8 Policy

アプリケーション固有の安全ルール。

```go
type Policy interface {
    Name() string
    CheckQuery(plan *QueryPlan) []Warning
    CheckMigration(plan *MigrationPlan) []Warning
}
```

### 4.9 Manifest

AI や外部ツールが Goquent の schema / relation / policy を読むための JSON 形式のメタデータ。  
v2 では、manifest は freshness / fingerprint を含む。

```go
type Manifest struct {
    Version                  string
    GeneratedAt              time.Time
    GeneratorVersion         string
    Dialect                  string
    SchemaFingerprint        string
    PolicyFingerprint        string
    GeneratedCodeFingerprint string
    DatabaseFingerprint      *string
    Tables                   []ManifestTable
    Policies                 []ManifestPolicy
    Verification             ManifestVerification
}
```

### 4.10 ReviewReport

`goquent review` が出力するレビュー結果。

```go
type ReviewReport struct {
    Findings           []Finding
    SuppressedFindings []Finding
    Summary            ReviewSummary
    ManifestStatus     *ManifestStatus
}
```

### 4.11 Finding

review 用の問題単位。

```go
type Finding struct {
    Code              string
    Level             RiskLevel
    Message           string
    Location          *SourceLocation
    Hint              string
    Evidence          []Evidence
    AnalysisPrecision AnalysisPrecision
    Suppressed         bool
    Suppression        *Suppression
}
```

---

## 5. Risk Classification

### 5.1 Important Boundary

`RiskLevel` は **structural DB risk** を表す。  
業務要件の正しさ、認可、ユーザー意図、法務上の妥当性を完全に保証するものではない。

| RiskLevel | 意味 |
|---|---|
| `low` | DB 操作の形として通常は低リスク |
| `medium` | 事故につながる可能性があり、レビュー対象 |
| `high` | 本番事故につながりやすく、CI fail / approval 候補 |
| `destructive` | データ損失・不可逆変更の可能性が高い |
| `blocked` | Goquent が実行拒否すべき操作 |

### 5.2 Low

通常の読み取りや、安全性の高い単一行操作。

例:

- primary key 指定の `SELECT`
- `LIMIT` 付きの list query
- required filter を満たした tenant-scoped query

注意:

```text
Low は「業務的に安全」を意味しない。
```

### 5.3 Medium

問題になり得るが、即ブロックするほどではない操作。

例:

- `LIMIT` のない `SELECT`
- PII column の選択
- index 利用が不明な query
- 複数テーブル JOIN
- bulk insert
- static analysis が partial

### 5.4 High

本番で事故になりやすい操作。

例:

- `UPDATE` / `DELETE` の WHERE が弱い
- tenant-scoped table で tenant filter がない
- soft delete table で deleted filter がない
- full scan の可能性が高い
- raw SQL の使用
- static analysis が unsupported で、対象が write operation の可能性を持つ

### 5.5 Destructive

データ損失や不可逆変更につながる操作。

例:

- `DROP TABLE`
- `DROP COLUMN`
- `TRUNCATE`
- nullable から non-nullable への変更 without default / backfill
- column type の縮小
- irreversible migration

### 5.6 Blocked

Goquent が実行を拒否すべき操作。

例:

- `DELETE FROM users` without WHERE
- tenant-scoped table に対する tenant filter なしの destructive operation
- policy により forbidden とされた PII export
- explicit approval なしの destructive migration
- expired approval による destructive operation

---

## 6. Warning Suppression / Approval Design

### 6.1 Suppression と Approval の違い

| 項目 | Suppression | Approval |
|---|---|---|
| 目的 | 誤検知・正当な例外を review noise から除外する | 危険操作の実行を明示的に許可する |
| 対象 | warning / finding | high / destructive operation |
| 出力 | pretty では隠してもよい。JSON では残せる | plan / audit に必ず残す |
| 理由 | 必須 | 必須 |
| 有効期限 | 推奨 | destructive では推奨 |
| blocked を解除できるか | できない | 原則できない。設定された approval-required のみ解除可能 |

### 6.2 Inline suppression

Go source では comment による suppression を許可する。

```go
// goquent:suppress PII_COLUMN_SELECTED reason="admin export requires email" expires="2026-07-01"
query := db.From(User).Select(user.ID, user.Email)
```

SQL migration では以下。

```sql
-- goquent:suppress DESTRUCTIVE_MIGRATION reason="legacy column unused for 90 days" expires="2026-07-01"
ALTER TABLE users DROP COLUMN legacy_id;
```

### 6.3 Config suppression

広い範囲の suppression は config に書ける。ただし、範囲が広い suppression ほど有効期限を必須にする。

```yaml
suppressions:
  - code: LIMIT_MISSING
    path: internal/admin/**/*.go
    reason: admin screens intentionally allow unbounded export after auth check
    expires: 2026-07-01
    owner: platform-team
```

### 6.4 Non-suppressible warnings

以下は初期設定では suppress 不可とする。

```text
DELETE_WITHOUT_WHERE
UPDATE_WITHOUT_WHERE
DESTRUCTIVE_MIGRATION
TENANT_FILTER_MISSING in enforce/block mode
MANIFEST_STALE when require_fresh_manifest is true
```

### 6.5 Tasks

- [ ] `Suppression` 構造体を定義する
- [ ] inline suppression parser を実装する
- [ ] SQL comment suppression parser を実装する
- [ ] config suppression loader を実装する
- [ ] suppression reason 必須チェックを実装する
- [ ] suppression expiration チェックを実装する
- [ ] expired suppression に `SUPPRESSION_EXPIRED` warning を出す
- [ ] non-suppressible warning の設定を実装する
- [ ] suppressed findings を JSON 出力に含める option を実装する
- [ ] approval と suppression の違いを docs に明記する

---

# 7. Phase Roadmap

---

## Phase 1: QueryPlan / Dry-run / SQL Visibility

### Goal

すべての query を実行前に説明可能にする。  
Goquent の AI-safe 化の土台を作る。

### Spec

#### 1. `Plan(ctx)` API を追加する

```go
plan, err := db.
    From(User).
    Where(user.ID.Eq(id)).
    Select(user.ID, user.Email).
    Plan(ctx)
```

#### 2. `Plan` は実行しない

`Plan(ctx)` は DB に対する write を発生させない。  
必要に応じて `EXPLAIN` など read-only な introspection は別 API に分離する。

```go
plan, err := query.Plan(ctx)       // SQL generation only
analysis, err := plan.Analyze(ctx) // optional DB introspection
```

#### 3. `QueryPlan` に最低限含める情報

- operation type: select / insert / update / delete / raw
- generated SQL
- bind parameters
- target tables
- selected columns
- predicates
- joins
- limit / offset
- warnings
- risk level
- required approval flag
- analysis precision

#### 4. `Exec` 前にも内部的に `Plan` を通す

```text
Build Query
  -> Build QueryPlan
  -> Run Risk Checks
  -> Check Approval Requirement
  -> Execute SQL
```

### API Sketch

```go
type Query interface {
    Plan(ctx context.Context) (*QueryPlan, error)
    Exec(ctx context.Context) (Result, error)
}
```

```go
func (p *QueryPlan) String() string
func (p *QueryPlan) ToJSON() ([]byte, error)
func (p *QueryPlan) RequiresApproval() bool
```

### Tasks

#### Core

- [ ] `OperationType` を定義する
- [ ] `QueryPlan` 構造体を定義する
- [ ] `Warning` 構造体を定義する
- [ ] `AnalysisPrecision` を定義する
- [ ] `SourceLocation` 構造体を定義する
- [ ] select query の `Plan(ctx)` を実装する
- [ ] insert query の `Plan(ctx)` を実装する
- [ ] update query の `Plan(ctx)` を実装する
- [ ] delete query の `Plan(ctx)` を実装する
- [ ] raw SQL query の `Plan(ctx)` を実装する
- [ ] SQL と params を `QueryPlan` に格納する
- [ ] table / column / predicate / join metadata を `QueryPlan` に格納する

#### Safety

- [ ] `Exec(ctx)` の前に必ず `Plan(ctx)` が生成されるようにする
- [ ] `Plan(ctx)` が write を発生させないことをテストする
- [ ] raw SQL の plan には `RAW_SQL_USED` warning を付与する

#### Formatting

- [ ] `QueryPlan.String()` を実装する
- [ ] `QueryPlan.ToJSON()` を実装する
- [ ] CLI 用の pretty print format を実装する

#### Tests

- [ ] select plan snapshot test
- [ ] insert plan snapshot test
- [ ] update plan snapshot test
- [ ] delete plan snapshot test
- [ ] raw SQL plan snapshot test
- [ ] params ordering test
- [ ] SQL injection regression test

### Acceptance Criteria

- [ ] 主要な query が `Plan(ctx)` で SQL と params を出せる
- [ ] `Plan(ctx)` は DB write を発生させない
- [ ] `Exec(ctx)` は内部的に plan generation を通る
- [ ] plan の JSON 出力ができる
- [ ] snapshot test により SQL の変更が検知できる

---

## Phase 2: RiskEngine / Basic Guardrails / Approval / Suppression MVP

### Goal

AI や人間が書いた query の危険度を deterministic に分類する。  
同時に、誤検知運用のための suppression と、危険操作のための approval reason を早期に入れる。

### Spec

#### 1. `RiskEngine` を追加する

```go
type RiskEngine interface {
    CheckQuery(plan *QueryPlan) RiskResult
}
```

```go
type RiskResult struct {
    Level             RiskLevel
    Warnings          []Warning
    RequiredApproval  bool
    Blocked           bool
}
```

#### 2. Built-in rules

最初に入れる built-in rules:

- `UPDATE_WITHOUT_WHERE`
- `DELETE_WITHOUT_WHERE`
- `LIMIT_MISSING`
- `SELECT_STAR_USED`
- `RAW_SQL_USED`
- `BULK_UPDATE_DETECTED`
- `BULK_DELETE_DETECTED`
- `DESTRUCTIVE_SQL_DETECTED`

#### 3. Risk level aggregation

複数 warning がある場合、最も高い risk level を `QueryPlan.RiskLevel` に反映する。

```text
Low < Medium < High < Destructive < Blocked
```

#### 4. Approval requirement

High 以上、または設定で approval required とされた warning がある場合、`RequiredApproval` を true にする。

#### 5. Approval reason

危険操作には reason を持たせる。

```go
result, err := query.
    RequireApproval("bulk update verified by operator").
    Exec(ctx)
```

#### 6. Suppression MVP

warning の一部は suppress できる。ただし、reason は必須とする。

```go
// goquent:suppress LIMIT_MISSING reason="batch job intentionally scans all active accounts" expires="2026-07-01"
query := db.From(Account).Where(account.Status.Eq("active")).Select(account.ID)
```

### API Sketch

```go
plan, err := query.Plan(ctx)
if plan.RequiredApproval {
    return err
}
```

```go
result, err := query.
    RequireApproval("bulk update verified by operator").
    Exec(ctx)
```

### Tasks

#### Core

- [ ] `RiskEngine` interface を定義する
- [ ] default risk engine を実装する
- [ ] risk level ordering を実装する
- [ ] warning aggregation を実装する
- [ ] `QueryPlan.RiskLevel` に結果を反映する
- [ ] `QueryPlan.RequiredApproval` に結果を反映する
- [ ] `RiskLow` の意味を docs に明記する

#### Built-in Rules

- [ ] update without where rule
- [ ] delete without where rule
- [ ] select star rule
- [ ] missing limit rule
- [ ] raw SQL rule
- [ ] dangerous SQL token rule: drop / truncate / alter
- [ ] weak predicate rule: `WHERE 1=1`
- [ ] bulk update heuristic
- [ ] bulk delete heuristic

#### Approval

- [ ] `Approval` 構造体を定義する
- [ ] `RequireApproval(reason string)` API を追加する
- [ ] approval reason を plan metadata に保存する
- [ ] approval reason が空なら error にする
- [ ] blocked operation は approval 付きでも実行不可にする
- [ ] destructive operation は config により approval 必須にする
- [ ] approval の JSON 出力を実装する

#### Suppression MVP

- [ ] `Suppression` 構造体を定義する
- [ ] suppressible / non-suppressible を warning ごとに設定できるようにする
- [ ] inline suppression parser を実装する
- [ ] suppression reason 必須チェックを実装する
- [ ] suppression expires parser を実装する
- [ ] expired suppression warning を実装する
- [ ] non-suppressible warning が suppress されようとした場合に warning を出す

#### Config

- [ ] risk rule の enable / disable 設定を追加する
- [ ] warning code ごとの severity override を追加する
- [ ] warning code ごとの suppressible 設定を追加する
- [ ] environment ごとの設定を追加する: local / test / staging / production

#### Tests

- [ ] risk rule unit tests
- [ ] risk aggregation tests
- [ ] approval required tests
- [ ] approval reason required tests
- [ ] blocked operation tests
- [ ] config override tests
- [ ] suppression parser tests
- [ ] expired suppression tests
- [ ] non-suppressible warning tests

### Acceptance Criteria

- [ ] 危険 query に warning が出る
- [ ] risk level が query plan に反映される
- [ ] approval required の query を判定できる
- [ ] blocked operation を実行拒否できる
- [ ] suppressible warning を reason 付きで抑制できる
- [ ] non-suppressible warning は抑制できない
- [ ] risk engine は LLM なしで deterministic に動く

---

## Phase 3: Minimal Policy DSL: Tenant / Soft Delete / PII

### Goal

アプリケーション固有の DB 安全ルールを Goquent が理解し、query plan に反映する。  
MVP では tenant / soft delete / PII に限定する。

### Spec

#### 1. Model policy declaration

モデルごとに policy を宣言できるようにする。

```go
goquent.Model(User{}).
    Table("users").
    TenantScoped(user.TenantID).
    SoftDelete(user.DeletedAt).
    PII(user.Email, user.PhoneNumber).
    RequiredFilter(user.TenantID)
```

#### 2. Tenant scope

tenant-scoped table では、指定された tenant column に対する filter が必要。

```go
db.From(User).Select(user.ID, user.Email)
```

上記は次の warning または error になる。

```text
TENANT_FILTER_MISSING: users is tenant-scoped but tenant_id filter is missing.
```

#### 3. Soft delete

soft delete column が定義された model では、default で deleted row を除外する。

```go
db.From(User).Select(user.ID)
```

内部的には以下を追加する。

```sql
WHERE users.deleted_at IS NULL
```

明示的に含める場合:

```go
db.From(User).WithDeleted()
```

#### 4. PII

PII column の select / export に warning を付与する。

```go
db.From(User).Select(user.Email)
```

```text
PII_COLUMN_SELECTED: users.email is marked as PII.
```

正当な用途では access reason を指定できる。

```go
query := db.From(User).
    Select(user.ID, user.Email).
    AccessReason("customer support lookup")
```

`AccessReason` は warning を自動で消すものではない。  
review 出力に根拠として残す。

#### 5. Required filter

特定 column に filter が必要であることを宣言できる。

```go
RequiredFilter(user.OrganizationID)
```

#### 6. Policy mode

policy ごとに mode を指定できる。

```go
PolicyModeWarn
PolicyModeEnforce
PolicyModeBlock
```

### API Sketch

```go
type ModelPolicyBuilder[T any] struct {}

func Model[T any]() *ModelPolicyBuilder[T]
func (b *ModelPolicyBuilder[T]) TenantScoped(column Column[T]) *ModelPolicyBuilder[T]
func (b *ModelPolicyBuilder[T]) SoftDelete(column Column[T]) *ModelPolicyBuilder[T]
func (b *ModelPolicyBuilder[T]) PII(columns ...Column[T]) *ModelPolicyBuilder[T]
func (b *ModelPolicyBuilder[T]) RequiredFilter(columns ...Column[T]) *ModelPolicyBuilder[T]
```

### Tasks

#### Core

- [ ] model metadata registry を実装する
- [ ] table metadata を実装する
- [ ] column metadata を実装する
- [ ] policy metadata を実装する
- [ ] model registration API を実装する

#### Tenant Scope

- [ ] `TenantScoped(column)` を実装する
- [ ] tenant filter detection を実装する
- [ ] missing tenant filter warning を実装する
- [ ] tenant filter missing 時の enforce mode を実装する
- [ ] current tenant injection API を検討する

#### Soft Delete

- [ ] `SoftDelete(column)` を実装する
- [ ] default soft delete predicate injection を実装する
- [ ] `WithDeleted()` を実装する
- [ ] `OnlyDeleted()` を実装する
- [ ] delete を soft delete update に変換する option を実装するか検討する

#### PII

- [ ] `PII(columns...)` を実装する
- [ ] PII select warning を実装する
- [ ] PII export warning を実装する
- [ ] `AccessReason(reason string)` API を実装する
- [ ] empty access reason を reject する
- [ ] access reason を QueryPlan / ReviewReport に出力する

#### Required Filter

- [ ] `RequiredFilter(columns...)` を実装する
- [ ] missing required filter detection を実装する
- [ ] compound required filter を検討する

#### Policy Mode

- [ ] warn / enforce / block mode を実装する
- [ ] environment ごとの policy mode override を実装する
- [ ] policy violation を QueryPlan に反映する

#### Tests

- [ ] tenant-scoped select test
- [ ] tenant-scoped update test
- [ ] tenant-scoped delete test
- [ ] soft delete default filter test
- [ ] with deleted test
- [ ] PII warning test
- [ ] PII access reason test
- [ ] required filter test
- [ ] policy mode test

### Acceptance Criteria

- [ ] model に tenant scope を宣言できる
- [ ] tenant filter 漏れを検知できる
- [ ] soft delete filter が default で追加される
- [ ] PII column selection を warning にできる
- [ ] PII access reason を出力できる
- [ ] policy violation を warning / error / block に切り替えられる

---

## Phase 4: Review CLI / CI Integration

### Goal

AI が生成したコードや migration を、Goquent が CLI / CI でレビューできるようにする。  
最初の killer feature として `goquent review` を提供する。

### Spec

#### 1. Review command

```bash
goquent review
```

出力例:

```text
Database Review

[High] users query is missing tenant_id filter
  file: internal/user/repository.go:42
  precision: precise
  hint: add Where(user.TenantID.Eq(currentTenantID))

[Medium] PII column selected: users.email
  file: internal/report/export.go:18
  precision: precise
  access_reason: customer support lookup
  hint: avoid selecting PII or provide a narrower access path

[Medium] Could not fully reconstruct Goquent query chain
  file: internal/order/service.go:88
  precision: partial
  code: STATIC_REVIEW_PARTIAL
  hint: run query.Plan(ctx) in tests or simplify the query construction
```

#### 2. Review targets

- Go source code
- generated QueryPlan JSON
- migration files
- raw SQL files
- manifest

#### 3. Static review precision

Go source review では、必ず `AnalysisPrecision` を持つ。

```text
precise     -> exact QueryPlan generated
partial     -> partial metadata extracted
unsupported -> fallback warning only
```

#### 4. CI behavior

```bash
goquent review --fail-on high
```

Exit code:

- 0: no findings above threshold
- 1: findings above threshold
- 2: configuration / parse error
- 3: stale manifest when fresh manifest is required

#### 5. Output formats

- pretty text
- JSON
- GitHub Actions annotation
- SARIF, optional

#### 6. Baseline support

既存プロジェクトに導入しやすくするため、baseline file を検討する。

```bash
goquent review --write-baseline .goquent/review-baseline.json
```

baseline は新規 warning を見逃すためではなく、段階的導入のための逃げ道とする。

#### 7. Reserved and optional CLI capabilities

A CLI flag is a current documented feature only when it is implemented, validated by tests, and
shown in examples.

- `goquent review --config` currently loads JSON review configuration (manifest defaults, risk/precision gates, rule overrides and suppressions); see section 0.4. This is not the unified YAML configuration draft in section 9.
- `goquent review --write-baseline` is optional/future and must not appear in the README main path until its adoption rules are specified.
- SARIF output is optional/future unless implemented and tested. Even when implemented, it belongs in CI docs rather than the README primary path.

### Tasks

#### Core

- [ ] `ReviewReport` 構造体を定義する
- [ ] `Finding` 構造体を定義する
- [ ] review target discovery を実装する
- [ ] query plan review を実装する
- [ ] raw SQL review を実装する
- [ ] suppression application を実装する
- [ ] suppressed findings の JSON 出力を実装する

#### Go Source Review

- [ ] Go AST parser integration を実装する
- [ ] Goquent query chain detection を実装する
- [ ] source location mapping を実装する
- [ ] Plan 生成が可能な static pattern を抽出する
- [ ] precise / partial / unsupported を判定する
- [ ] static extraction できない場合の fallback warning を実装する
- [ ] unsupported pattern を docs に明記する

#### CLI

- [ ] `goquent review` を実装する
- [ ] `goquent review --fail-on medium|high|destructive` を実装する
- [ ] `goquent review --format json` を実装する
- [ ] `goquent review --format github` を実装する
- [ ] `goquent review --config goquent.yaml` を実装する
- [ ] `goquent review --show-suppressed` を実装する
- [ ] `goquent review --write-baseline` を検討する

#### CI

- [ ] GitHub Actions example を作成する
- [ ] CI 用 exit code behavior を実装する
- [ ] baseline file を検討する
- [ ] suppress annotation を検討する
- [ ] stale manifest fail behavior を設計する

#### Tests

- [ ] review CLI snapshot tests
- [ ] fail-on threshold tests
- [ ] source location tests
- [ ] precise static analysis tests
- [ ] partial static analysis tests
- [ ] unsupported static analysis tests
- [ ] raw SQL review tests
- [ ] suppression tests
- [ ] baseline tests

### Acceptance Criteria

- [ ] `goquent review` が query の問題を出せる
- [ ] source location を表示できる
- [ ] analysis precision を表示できる
- [ ] CI で high risk 以上を fail にできる
- [ ] JSON output を他ツールが読める
- [ ] suppression により誤検知を運用できる
- [ ] unsupported static pattern を「安全」と誤認させない
- [ ] AI 生成コードのレビューに使える実用的な出力になる

---

## Phase 5: MigrationPlan / Migration Risk Review

### Goal

AI が生成した migration、または schema diff から作られた migration を、実行前に安全性評価できるようにする。

### Spec

#### 1. MigrationPlan generation

```bash
goquent migrate plan
```

または Go API:

```go
plan, err := migrator.Plan(ctx)
```

#### 2. Migration step classification

各 migration step を分類する。

```go
type MigrationStepType string

const (
    AddTable        MigrationStepType = "add_table"
    DropTable       MigrationStepType = "drop_table"
    AddColumn       MigrationStepType = "add_column"
    DropColumn      MigrationStepType = "drop_column"
    RenameColumn    MigrationStepType = "rename_column"
    AlterColumnType MigrationStepType = "alter_column_type"
    AddIndex        MigrationStepType = "add_index"
    DropIndex       MigrationStepType = "drop_index"
)
```

#### 3. Risk rules

- add nullable column: Low
- add non-null column with default: Medium
- add non-null column without default: High
- drop column: Destructive
- drop table: Destructive
- type expansion: Medium
- type narrowing: Destructive
- rename column: Medium / High
- add index concurrently unsupported: Medium / High
- drop index: Medium

#### 4. Preflight suggestions

Destructive / High migration には preflight suggestion を出す。

例:

```text
Drop column users.legacy_id
Risk: Destructive
Suggested preflight:
  - confirm no application code references users.legacy_id
  - confirm the column has been unused for 30 days
  - backup table before migration
  - deploy code that stops writing this column before dropping it
```

#### 5. Approval requirement

Destructive migration は explicit approval がない限り実行不可。

```bash
goquent migrate apply --approve "drop legacy_id after 30 days no usage"
```

#### 6. Migration apply is human-controlled

`goquent migrate apply` is a human-controlled deployment command, not an AI-agent workflow and not
an MCP tool.

The README primary migration path should emphasize:

1. `goquent migrate plan`
2. dry-run
3. risk review
4. approval reason for high/destructive changes
5. preflight checks
6. human-controlled apply, if the project uses Goquent for application

AI agents may prepare or review migration artifacts, but must not run migration apply.

### API Sketch

```go
plan, err := migrator.Plan(ctx)
fmt.Println(plan.RiskLevel)
fmt.Println(plan.RequiredApproval)
```

### Tasks

#### Core

- [ ] `MigrationPlan` 構造体を定義する
- [ ] `MigrationStep` 構造体を定義する
- [ ] migration SQL parser または structured migration builder を選定する
- [ ] migration step extractor を実装する
- [ ] migration risk engine を実装する
- [ ] migration approval を query approval と統一する

#### Schema Diff

- [ ] current schema introspection を実装する
- [ ] desired schema representation を定義する
- [ ] schema diff engine を実装する
- [ ] diff から migration steps を生成する

#### Risk Rules

- [ ] add table rule
- [ ] drop table rule
- [ ] add column rule
- [ ] drop column rule
- [ ] rename column rule
- [ ] alter column type rule
- [ ] add index rule
- [ ] drop index rule
- [ ] nullable change rule
- [ ] default change rule

#### CLI

- [ ] `goquent migrate plan` を実装する
- [ ] `goquent migrate dry-run` を実装する
- [ ] `goquent migrate apply` を実装する
- [ ] `goquent migrate apply --approve` を実装する
- [ ] JSON output option を実装する
- [ ] `goquent review` から migration review を呼べるようにする

#### Tests

- [ ] migration step classification tests
- [ ] destructive migration tests
- [ ] approval required tests
- [ ] schema diff tests
- [ ] CLI snapshot tests
- [ ] suppression / approval interaction tests

### Acceptance Criteria

- [ ] migration を実行前に plan として出力できる
- [ ] migration step ごとの risk を判定できる
- [ ] destructive migration を approval なしでブロックできる
- [ ] migration plan を JSON 出力できる
- [ ] CI で migration risk を検査できる

---

## Phase 6: Manifest / AI-readable Schema Export / Stale Detection

### Goal

AI エディタ、MCP server、レビュー CLI が Goquent の schema / relation / policy を安全に参照できるようにする。  
ただし、manifest は古くなると危険なので、freshness verification を必須設計にする。

### Spec

#### 1. Manifest output

```bash
goquent manifest --format json
```

#### 2. Manifest contents

最低限含めるもの:

- tables
- models
- columns
- primary keys
- foreign keys
- indexes
- relations
- nullable
- default values
- generated columns
- enum values
- PII flag
- tenant scope
- soft delete
- required filters
- policy modes
- query examples
- generated timestamp
- generator version
- schema fingerprint
- policy fingerprint
- generated code fingerprint
- optional database fingerprint

#### 3. Manifest example

```json
{
  "version": "1",
  "generated_at": "2026-04-24T00:00:00Z",
  "generator_version": "0.1.0",
  "dialect": "postgres",
  "schema_fingerprint": "sha256:...",
  "policy_fingerprint": "sha256:...",
  "generated_code_fingerprint": "sha256:...",
  "database_fingerprint": "sha256:...",
  "tables": [
    {
      "name": "users",
      "model": "User",
      "columns": [
        {
          "name": "id",
          "type": "uuid",
          "primary": true,
          "nullable": false
        },
        {
          "name": "email",
          "type": "text",
          "pii": true,
          "nullable": false
        },
        {
          "name": "tenant_id",
          "type": "uuid",
          "required_filter": true,
          "nullable": false
        },
        {
          "name": "deleted_at",
          "type": "timestamp",
          "soft_delete": true,
          "nullable": true
        }
      ],
      "policies": [
        {
          "type": "tenant_scope",
          "column": "tenant_id",
          "mode": "enforce"
        },
        {
          "type": "soft_delete",
          "column": "deleted_at",
          "mode": "enforce"
        }
      ]
    }
  ],
  "verification": {
    "fresh": true,
    "checked_at": "2026-04-24T00:00:00Z",
    "checks": [
      {
        "name": "generated_code",
        "status": "ok"
      },
      {
        "name": "policy",
        "status": "ok"
      },
      {
        "name": "database",
        "status": "ok"
      }
    ]
  }
}
```

#### 4. Manifest schema

Manifest 自体の JSON Schema も提供する。

```bash
goquent manifest schema
```

#### 5. Stale detection commands

```bash
goquent manifest verify
```

```bash
goquent manifest diff --against-db
```

```bash
goquent doctor
```

#### 6. Stale detection policy

`goquent review` や `goquent mcp` は、manifest が stale の場合に明示的な warning を出す。

```text
MANIFEST_STALE: manifest does not match generated code or database schema.
```

設定により、stale manifest を CI fail にできる。

```yaml
manifest:
  require_fresh: true
```

#### 7. Fingerprint status

Generated-code and policy fingerprints are part of the core stale-detection model when manifest
verification is implemented.

Database fingerprint is optional because it requires live database access. If no database
connection is configured, manifest verification must report that the database check was not
performed. Whether that is acceptable is controlled by configuration.

README should describe this as stale manifest detection, not as a guarantee that the live database
always matches.

### Tasks

#### Core

- [ ] manifest structure を定義する
- [ ] manifest versioning policy を定義する
- [ ] model registry から manifest を生成する
- [ ] DB introspection から manifest を生成する option を検討する
- [ ] manifest JSON output を実装する
- [ ] manifest pretty output を実装する

#### Fingerprint / Freshness

- [ ] schema fingerprint を定義する
- [ ] policy fingerprint を定義する
- [ ] generated code fingerprint を定義する
- [ ] database fingerprint を定義する
- [ ] `goquent manifest verify` を実装する
- [ ] `goquent manifest diff --against-db` を実装する
- [ ] stale manifest warning を実装する
- [ ] stale manifest CI fail option を実装する

#### Content

- [ ] tables output
- [ ] columns output
- [ ] relations output
- [ ] indexes output
- [ ] policies output
- [ ] PII output
- [ ] tenant scope output
- [ ] soft delete output
- [ ] required filter output
- [ ] query examples output

#### Schema

- [ ] manifest JSON Schema を作成する
- [ ] manifest schema validation を実装する
- [ ] manifest version compatibility test を作成する

#### CLI

- [ ] `goquent manifest` を実装する
- [ ] `goquent manifest --format json` を実装する
- [ ] `goquent manifest --format pretty` を実装する
- [ ] `goquent manifest schema` を実装する
- [ ] `goquent manifest verify` を実装する
- [ ] `goquent doctor` に manifest check を入れる

#### Tests

- [ ] manifest snapshot tests
- [ ] manifest schema validation tests
- [ ] policy metadata output tests
- [ ] relation metadata output tests
- [ ] stale generated code test
- [ ] stale policy test
- [ ] stale database schema test
- [ ] CI fail-on-stale test

### Acceptance Criteria

- [ ] Goquent schema / policy を JSON manifest として出力できる
- [ ] manifest が JSON Schema で検証できる
- [ ] manifest freshness を検証できる
- [ ] stale manifest を review / MCP が警告できる
- [ ] stale manifest を CI fail にできる
- [ ] AI や外部ツールが schema と policy を読める
- [ ] manifest output が stable で snapshot test 可能

---

## Phase 7: OperationSpec MVP / Structured AI Interface

### Goal

AI が自由文 SQL を直接生成するのではなく、構造化された intent を Goquent に渡し、Goquent が安全に QueryPlan を生成できるようにする。  
MVP では read-only select に限定し、独自 query 言語として肥大化させない。

### Spec

#### 1. Current OperationSpec scope

The current/MVP OperationSpec is intentionally narrow:

- read-only `select` only
- single model only
- explicit select fields
- filter
- order by
- limit

The following are not current OperationSpec features and must not be documented as current README
capabilities:

- insert
- update
- delete
- join
- aggregate
- group by
- having
- subquery
- raw SQL
- CTE

These are future design topics, not current product capabilities. Mutation OperationSpec requires
a separate safety design and must not be conflated with AI autonomous DB execution.

#### 2. OperationSpec

```go
type OperationSpec struct {
    Operation string       `json:"operation"`
    Model     string       `json:"model"`
    Select    []string     `json:"select,omitempty"`
    Filters   []FilterSpec `json:"filters,omitempty"`
    OrderBy   []OrderSpec  `json:"order_by,omitempty"`
    Limit     *int64       `json:"limit,omitempty"`
}
```

JSON example:

```json
{
  "operation": "select",
  "model": "Order",
  "select": ["id", "total", "created_at"],
  "filters": [
    {
      "field": "tenant_id",
      "op": "=",
      "value_ref": "current_tenant"
    },
    {
      "field": "created_at",
      "op": ">=",
      "value_ref": "start_date"
    }
  ],
  "order_by": [
    {
      "field": "created_at",
      "direction": "desc"
    }
  ],
  "limit": 100
}
```

#### 3. Validation

OperationSpec は manifest / policy に対して検証する。

- 存在しない model は拒否
- 存在しない field は拒否
- forbidden field は拒否
- required filter がない場合は warning / error
- PII field は warning / access reason required
- limit がない list query は warning
- stale manifest の場合は compile を拒否または warning にする

#### 4. Compilation

```text
OperationSpec
  -> Validate with Manifest
  -> Verify Manifest Freshness
  -> Apply Policy
  -> Compile to QueryPlan
  -> RiskEngine
  -> Human Approval if needed
  -> Exec only if explicitly requested by Go code, not by AI tool default
```

### Tasks

#### Core

- [ ] `OperationSpec` を定義する
- [ ] MVP JSON Schema を作成する
- [ ] supported operation を `select` に限定する
- [ ] manifest-based validator を実装する
- [ ] manifest freshness check を組み込む
- [ ] policy validator を実装する
- [ ] OperationSpec から QueryPlan への compiler を実装する
- [ ] unsupported operation handling を実装する

#### Safety

- [ ] unknown model rejection
- [ ] unknown field rejection
- [ ] forbidden field rejection
- [ ] required filter validation
- [ ] PII validation
- [ ] missing limit warning
- [ ] stale manifest behavior test
- [ ] insert / update / delete rejection in MVP
- [ ] join / aggregate / subquery rejection in MVP

#### Tests

- [ ] valid select spec test
- [ ] invalid model test
- [ ] invalid field test
- [ ] missing tenant filter test
- [ ] PII field test
- [ ] stale manifest test
- [ ] unsupported mutation test
- [ ] unsupported join test
- [ ] compilation snapshot test

### Acceptance Criteria

- [ ] AI が構造化された read-only select OperationSpec を渡せる
- [ ] OperationSpec は manifest / policy により検証される
- [ ] valid spec から QueryPlan を作れる
- [ ] invalid / unsafe spec を拒否できる
- [ ] MVP が巨大な別 query 言語になっていない
- [ ] 自由文 SQL 直接実行より安全な interface になる

---

## Phase 8: MCP Server / AI Tooling

### Goal

AI エディタやコーディングエージェントが Goquent を安全な DB context provider / tool provider として利用できるようにする。  
現時点の MCP scope は read-only とし、AI に実行権限を渡さない。

### Spec

#### 1. Command

```bash
goquent mcp
```

#### 2. Resources

MCP resources として以下を公開する。

```text
goquent://schema
goquent://manifest
goquent://models
goquent://relations
goquent://policies
goquent://migrations
goquent://query-examples
goquent://review-rules
goquent://manifest-status
```

#### 3. Tools

MCP tools として以下を公開する。

```text
get_schema
get_manifest
get_manifest_status
explain_query
review_query
review_migration
generate_query_plan
compile_operation_spec
propose_repository_method
generate_test_fixture
```

#### 4. Prompts

MCP prompts として以下を提供する。

```text
add_repository_method
review_database_change
write_safe_migration
debug_slow_query
explain_query_plan
```

MCP prompts may help draft or review code changes, but they do not write files, apply migrations,
execute SQL, or approve destructive operations.

Prefer names such as:

- `draft_repository_method_guidance`
- `review_database_change`
- `propose_migration_plan`
- `review_migration_risk`

Avoid names such as `write_safe_migration` unless the docs explicitly state that the prompt only
produces reviewable text and never applies migrations.

#### 5. Safety

MCP server は現時点では read-only に限定する。

- DB write をしない
- migration apply をしない
- raw SQL execution をしない
- destructive operation は plan / review のみ
- stale manifest の場合は manifest resource に status を含める
- write tools は初期実装では存在しない

### Tasks

#### Core

- [ ] MCP server entrypoint を実装する
- [ ] current MCP scope を read-only に限定する
- [ ] manifest resource を実装する
- [ ] manifest status resource を実装する
- [ ] schema resource を実装する
- [ ] policy resource を実装する
- [ ] query examples resource を実装する
- [ ] review rules resource を実装する

#### Tools

- [ ] `get_schema` tool
- [ ] `get_manifest` tool
- [ ] `get_manifest_status` tool
- [ ] `explain_query` tool
- [ ] `review_query` tool
- [ ] `review_migration` tool
- [ ] `generate_query_plan` tool
- [ ] `compile_operation_spec` tool
- [ ] `generate_test_fixture` tool

#### Prompts

- [ ] `add_repository_method` prompt
- [ ] `review_database_change` prompt
- [ ] `write_safe_migration` prompt
- [ ] `debug_slow_query` prompt
- [ ] `explain_query_plan` prompt

#### Safety

- [ ] write tools を初期実装では提供しない
- [ ] config で MCP 公開範囲を制御する
- [ ] sensitive metadata masking を検討する
- [ ] stale manifest warning を MCP resource に含める
- [ ] OperationSpec compile tool が read-only select MVP だけを受けることを保証する

#### Tests

- [ ] MCP resource snapshot tests
- [ ] MCP tool tests
- [ ] OperationSpec compile tool tests
- [ ] read-only safety tests
- [ ] stale manifest MCP tests

### Acceptance Criteria

- [ ] AI エディタが Goquent manifest を読める
- [ ] AI エディタが query / migration review を tool として呼べる
- [ ] MCP server は初期状態で DB write しない
- [ ] stale manifest 状態を AI に伝えられる
- [ ] AI が Goquent の policy を理解したコード生成をしやすくなる

---

## Phase 9: Developer Experience / Documentation / Examples

### Goal

人間が使っても便利で、AI が使っても迷わない documentation と examples を整備する。

### Spec

#### 1. Docs structure

```text
docs/
  vision.md
  query-plan.md
  risk-engine.md
  suppression-and-approval.md
  policy-dsl.md
  static-review-limits.md
  migration-plan.md
  manifest.md
  manifest-stale-detection.md
  review-cli.md
  operation-spec.md
  mcp.md
  examples.md
```

#### 2. Example projects

- simple CRUD
- tenant-scoped SaaS
- soft delete
- PII warning
- suppression / approval example
- migration review
- manifest verify
- CI review
- MCP integration

#### 3. AI instruction templates

AI に Goquent を使わせるための instruction を用意する。

```text
When writing database code with Goquent:
1. Prefer QueryPlan before Exec.
2. Never bypass tenant policy.
3. Do not use raw SQL unless explicitly required.
4. For migrations, always run goquent migrate plan.
5. Include review output in PR comments.
6. Treat RiskLow as low structural DB risk, not business approval.
7. Do not trust manifest unless goquent manifest verify passes.
8. If goquent review says partial or unsupported, do not claim the query is safe.
```

### Tasks

- [ ] vision document を作成する
- [ ] QueryPlan guide を作成する
- [ ] RiskEngine guide を作成する
- [ ] Suppression / Approval guide を作成する
- [ ] Policy DSL guide を作成する
- [ ] Static Review Limits guide を作成する
- [ ] MigrationPlan guide を作成する
- [ ] Manifest guide を作成する
- [ ] Manifest Stale Detection guide を作成する
- [ ] Review CLI guide を作成する
- [ ] OperationSpec guide を作成する
- [ ] MCP guide を作成する
- [ ] example project を作成する
- [ ] AI instruction template を作成する
- [ ] PR review template を作成する

### Acceptance Criteria

- [ ] 新規ユーザーが Goquent の独自価値を理解できる
- [ ] AI に渡せる instruction がある
- [ ] CI 導入までの手順がある
- [ ] manifest freshness の重要性が説明されている
- [ ] static review の限界が説明されている
- [ ] example が実装と同期している

---

# 8. Suggested MVP Path

最短で独自ポジションを示すなら、以下の順がよい。

```text
MVP-1: QueryPlan
MVP-2: RiskEngine + Approval/Suppression MVP
MVP-3: Policy DSL for tenant / soft delete / PII
MVP-4: goquent review
MVP-5: MigrationPlan / Migration Review
MVP-6: Manifest export + stale detection
MVP-7: OperationSpec read-only select MVP
MVP-8: MCP server read-only
```

特に最初の公開デモは以下が強い。

```bash
goquent review
```

出力例:

```text
Database Review

[Blocked] DELETE FROM users without WHERE
  file: internal/user/repository.go:42
  precision: precise
  hint: add a specific predicate. This finding is not suppressible.

[High] Missing tenant_id filter on tenant-scoped table users
  file: internal/user/repository.go:77
  precision: precise
  hint: add Where(user.TenantID.Eq(currentTenantID))

[Medium] PII column selected: users.email
  file: internal/report/export.go:18
  precision: precise
  access_reason: customer support export
  hint: provide a narrower query or document access reason

[Medium] Could not fully reconstruct Goquent query chain
  file: internal/order/service.go:88
  precision: partial
  code: STATIC_REVIEW_PARTIAL
  hint: add a test that snapshots query.Plan(ctx)
```

このデモは、Goquent が「便利な ORM」ではなく「AI 生成 DB コードの安全境界」であることを分かりやすく示せる。

---

# 9. Configuration Draft

```yaml
# goquent.yaml
version: 1

environment: development

risk:
  fail_on: high
  rules:
    UPDATE_WITHOUT_WHERE:
      severity: blocked
      suppressible: false
    DELETE_WITHOUT_WHERE:
      severity: blocked
      suppressible: false
    SELECT_STAR_USED:
      severity: medium
      suppressible: true
    LIMIT_MISSING:
      severity: medium
      suppressible: true
    RAW_SQL_USED:
      severity: high
      suppressible: true
      require_reason: true
    PII_COLUMN_SELECTED:
      severity: medium
      suppressible: true
      require_reason: true
    TENANT_FILTER_MISSING:
      severity: high
      suppressible: false
    SOFT_DELETE_FILTER_MISSING:
      severity: medium
      suppressible: true
    MANIFEST_STALE:
      severity: high
      suppressible: false
    STATIC_REVIEW_PARTIAL:
      severity: medium
      suppressible: true
    STATIC_REVIEW_UNSUPPORTED:
      severity: high
      suppressible: true

policies:
  tenant_scope:
    mode: enforce
  soft_delete:
    mode: enforce
  pii:
    mode: warn
    require_access_reason: true

manifest:
  path: .goquent/manifest.json
  require_fresh: true
  verify:
    generated_code: true
    policy: true
    database: optional

review:
  include:
    - "internal/**/*.go"
    - "migrations/**/*.sql"
  exclude:
    - "vendor/**"
  output: pretty
  show_suppressed: false
  static_analysis:
    precise_patterns: true
    warn_on_partial: true
    warn_on_unsupported: true

suppressions:
  - code: LIMIT_MISSING
    path: internal/admin/**/*.go
    reason: admin screens intentionally allow unbounded export after auth check
    expires: 2026-07-01
    owner: platform-team

mcp:
  enabled: true
  mode: read_only
  expose:
    manifest: true
    manifest_status: true
    policies: true
    query_examples: true
    migrations: false
  allow_write_tools: false
```

---

# 10. Non-normative API Direction

The examples in this section are design sketches. They are not public API unless the
implementation and examples in the repository match them exactly.

README, docs, and examples must use only implemented public API. Do not copy this section into
user-facing docs as current usage.

## 10.1 Plan-first query

```go
query := db.
    From(User).
    Where(user.TenantID.Eq(currentTenantID)).
    Where(user.ID.Eq(id)).
    Select(user.ID, user.Email)

plan, err := query.Plan(ctx)
if err != nil {
    return err
}

if plan.RiskLevel >= goquent.RiskHigh {
    return fmt.Errorf("unsafe query: %v", plan.Warnings)
}

user, err := query.First(ctx)
```

## 10.2 Policy declaration

```go
goquent.Model(User{}).
    Table("users").
    PrimaryKey(user.ID).
    TenantScoped(user.TenantID).
    SoftDelete(user.DeletedAt).
    PII(user.Email, user.PhoneNumber).
    RequiredFilter(user.TenantID)
```

## 10.3 Approval for risky operation

```go
result, err := db.
    Delete(User).
    Where(user.LastLoginAt.Lt(cutoff)).
    RequireApproval("delete inactive users older than 2 years after audit").
    Exec(ctx)
```

## 10.4 PII access reason

```go
query := db.
    From(User).
    Select(user.ID, user.Email).
    AccessReason("customer support lookup")

plan, err := query.Plan(ctx)
```

## 10.5 Migration review

```bash
goquent migrate plan --format pretty
```

```text
Migration Plan

[Low] Add column users.display_name text nullable

[Destructive] Drop column users.legacy_id
  Requires approval: true
  Suggested preflight:
    - confirm no app code references users.legacy_id
    - confirm no writes for 30 days
    - backup users table
```

## 10.6 Manifest verification

```bash
goquent manifest verify
```

```text
Manifest Verification

[OK] generated code fingerprint matches
[OK] policy fingerprint matches
[WARN] database fingerprint not checked; no database connection configured
```

## 10.7 OperationSpec MVP

```json
{
  "operation": "select",
  "model": "User",
  "select": ["id", "email"],
  "filters": [
    {
      "field": "tenant_id",
      "op": "=",
      "value_ref": "current_tenant"
    }
  ],
  "order_by": [
    {
      "field": "created_at",
      "direction": "desc"
    }
  ],
  "limit": 100
}
```

---

# 11. Open Design Questions

現時点で決め切らなくてよいが、実装前に検討したい点。

## 11.1 SQL dialect

- MySQL and PostgreSQL are already implemented; preserve both dialects.
- SQLite remains outside the current support contract.
- Future semantic risk rules must preserve dialect-specific limits and compare
  equivalent operation meaning rather than requiring identical SQL (contracts v3).

## 11.2 Query metadata extraction

- query builder 内部 AST から metadata を取るか
- SQL parser から metadata を取るか
- raw SQL はどこまで解析するか

## 11.3 Policy enforcement timing

- query build 時に error にするか
- Plan 時に warning / error にするか
- Exec 時に最終 block するか

推奨:

```text
Build: lightweight validation
Plan: full policy/risk evaluation
Exec: final enforcement
```

## 11.4 Approval design

- approval token を型で表すか
- reason string で十分か
- production では external approval system と連携するか
- approval に期限を必須にするか

## 11.5 Suppression design

- inline comment の構文をどうするか
- config suppression の範囲が広すぎる場合にどう警告するか
- suppression の期限切れを CI fail にするか

## 11.6 Static review precision

`goquent review` が Go source から query chain をどこまで正確に復元するか。

初期段階では以下でよい。

```text
known static patterns: precise plan
unknown dynamic patterns: partial or unsupported warning
```

## 11.7 Manifest fingerprint source

- schema fingerprint を model registry 由来にするか
- generated code 由来にするか
- live DB introspection 由来にするか
- それらを別々に保持するか

推奨:

```text
別々に保持する。
ズレが発生したときに、どこが stale なのか分かる方がよい。
```

## 11.8 AI interface maturity

- OperationSpec をどの程度表現力豊かにするか
- join / aggregate / subquery をいつ対応するか
- LLM からの spec に対する validation をどこまで厳しくするか

推奨:

```text
MVP は read-only select のみに限定する。
表現力より安全性・検証可能性を優先する。
```

---

# 12. Recommended First Implementation Slice

最初の実装単位としては、以下がよい。

## Slice 1: QueryPlan minimal

- [ ] `QueryPlan` struct
- [ ] `Plan(ctx)` for select
- [ ] SQL / params / table / columns output
- [ ] JSON output
- [ ] snapshot tests

## Slice 2: Basic RiskEngine

- [ ] `RiskLevel`
- [ ] `Warning`
- [ ] `SELECT_STAR_USED`
- [ ] `LIMIT_MISSING`
- [ ] `RAW_SQL_USED`
- [ ] risk aggregation
- [ ] docs: `RiskLow` is structural, not business-safe

## Slice 3: Update/Delete guard + Approval

- [ ] `UPDATE_WITHOUT_WHERE`
- [ ] `DELETE_WITHOUT_WHERE`
- [ ] `RequiredApproval`
- [ ] `RequireApproval(reason string)`
- [ ] blocked operation handling

## Slice 4: Suppression MVP

- [ ] inline suppression comment parser
- [ ] reason required
- [ ] expires support
- [ ] non-suppressible warnings
- [ ] `--show-suppressed`

## Slice 5: Tenant policy minimal

- [ ] model policy registry
- [ ] `TenantScoped(column)`
- [ ] tenant filter detection
- [ ] warning / block mode

## Slice 6: CLI demo

- [ ] `goquent review` minimal command
- [ ] pretty output
- [ ] JSON output
- [ ] source location
- [ ] analysis precision
- [ ] one example project

## Slice 7: Manifest freshness later

- [ ] manifest generation
- [ ] fingerprinting
- [ ] `goquent manifest verify`
- [ ] stale warning
- [ ] MCP resource later

---

# 13. Success Metrics

Goquent がこの方向に進んでいるかを測る指標。

## Developer Experience

- query plan を見れば、ORM DSL を読まなくても SQL の意図が分かる
- policy violation が実行前に分かる
- migration risk が CI で検知できる
- 人間が PR review で見るべき箇所が明確になる
- warning suppression により review fatigue を抑えられる
- `RiskLow` を過信しない docs / output になっている

## AI Compatibility

- AI が manifest から schema / policy を理解できる
- AI が stale manifest を信じてよいか判断できる
- AI が Goquent の禁止事項を回避した query を生成しやすい
- AI が作った migration に対し、Goquent が deterministic に警告できる
- MCP 経由で query review / migration review を呼べる
- static analysis が partial / unsupported の場合に、AI が安全だと誤認しない

## Safety

- tenant filter 漏れを検知できる
- soft delete 漏れを検知できる
- destructive migration を approval なしで防げる
- raw SQL の使用を明示的に扱える
- high risk query を CI で fail できる
- stale manifest を検知できる
- suppress できない warning を抑制できない

## Maintenance Cost Control

- OperationSpec が別の巨大 query 言語になっていない
- MCP が初期状態で write 権限を持たない
- suppression に reason / expires がある
- unsupported static patterns を無理に解析しない
- manifest freshness check が CI に組み込み可能

---

# 14. Positioning Statement

Goquent は、既存 ORM と「書きやすさ」だけで競争しない。  
Goquent の独自性は、AI コーディング時代における DB 操作の安全境界になることにある。

```text
Goquent is an AI-safe ORM for Go.
It makes database operations explainable, policy-aware, reviewable, approval-gated, and staleness-aware.
```

この方向なら、人間が書くコードにも価値があり、AI が書くコードにはさらに価値が出る。

ただし、v2 では以下を明確な境界として扱う。

```text
Goquent は DB 操作の形を検証する。
Goquent は業務判断そのものを完全には検証しない。

Goquent は AI に context と review tool を与える。
Goquent は AI に初期状態で DB 実行権限を与えない。

Goquent は static analysis を行う。
Goquent は解析できない動的 query を「安全」とは言わない。

Goquent は manifest を生成する。
Goquent は manifest が古い可能性を検証する。
```
