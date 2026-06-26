package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/migration"
	"github.com/recoweft/goquent/orm/operation"
)

type User struct {
	ID        int64      `db:"id,pk"`
	TenantID  string     `db:"tenant_id"`
	Name      string     `db:"name"`
	Email     string     `db:"email"`
	DeletedAt *time.Time `db:"deleted_at,omitempty"`
}

func (User) TableName() string { return "users" }

func main() {
	ctx := context.Background()
	db := orm.NewDB(nil, driver.MySQLDialect{})

	orm.ResetModelPolicies()
	if err := orm.Model(User{}).
		Table("users").
		TenantScoped("tenant_id").
		SoftDelete("deleted_at").
		RequiredFilter("tenant_id").
		PII("email").
		Register(); err != nil {
		log.Fatal(err)
	}

	simpleInsert, err := db.Table("users").
		PlanInsert(ctx, map[string]any{
			"tenant_id": "tenant_123",
			"name":      "Alice",
			"email":     "alice@example.test",
		})
	must("simple insert plan", err)

	tenantRead, err := db.Table("users").
		Select("id", "name").
		Where("tenant_id", "tenant_123").
		OrderBy("id", "asc").
		Limit(100).
		Plan(ctx)
	must("tenant read plan", err)

	piiRead, err := db.Table("users").
		Select("id", "email").
		Where("tenant_id", "tenant_123").
		AccessReason("support ticket TICKET-123").
		Limit(1).
		Plan(ctx)
	must("PII read plan", err)

	suppressedExport, err := db.Table("users").
		Select("id", "name").
		Where("tenant_id", "tenant_123").
		SuppressWarning(
			orm.WarningLimitMissing,
			"tenant export streams through an application-level cursor",
			orm.SuppressionOwner("data-platform"),
			orm.SuppressionExpiresAt(time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)),
		).
		Plan(ctx)
	must("suppressed export plan", err)

	approvedUpdate, err := db.Table("audit_logs").
		WhereRaw("1 = 1", map[string]any{}).
		RequireApproval("one-time audit log classification backfill").
		PlanUpdate(ctx, map[string]any{"category": "import"})
	must("approved update plan", err)

	migrationPlan, err := migration.PlanSQL(`ALTER TABLE users DROP COLUMN legacy_email;`)
	must("migration plan", err)

	m, err := manifest.Generate(manifest.Options{
		Dialect:     "mysql",
		GeneratedAt: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
		Schema: &migration.Schema{Tables: []migration.TableSchema{{
			Name: "users",
			Columns: []migration.ColumnSchema{
				{Name: "id", Type: "bigint", Nullable: false},
				{Name: "tenant_id", Type: "varchar(64)", Nullable: false},
				{Name: "name", Type: "varchar(255)", Nullable: false},
				{Name: "email", Type: "varchar(255)", Nullable: false},
				{Name: "deleted_at", Type: "timestamp", Nullable: true},
			},
			Indexes: []migration.IndexSchema{{Name: "idx_users_tenant_id", Columns: []string{"tenant_id"}}},
		}}},
		Policies: orm.RegisteredTablePolicies(),
	})
	must("manifest", err)

	orm.ResetModelPolicies()

	limit := int64(25)
	operationPlan, err := operation.Compile(ctx, operation.OperationSpec{
		Operation: "select",
		Model:     "users",
		Select:    []string{"id", "name"},
		Filters: []operation.FilterSpec{{
			Field:    "tenant_id",
			Op:       "=",
			ValueRef: "current_tenant",
		}},
		OrderBy: []operation.OrderSpec{{Field: "id", Direction: "asc"}},
		Limit:   &limit,
	}, operation.Options{
		Manifest: m,
		Values:   map[string]any{"current_tenant": "tenant_123"},
	})
	must("operation spec compile", err)

	printJSON("simple_insert", simpleInsert)
	printJSON("tenant_read", tenantRead)
	printJSON("pii_read", piiRead)
	printJSON("suppressed_export", suppressedExport)
	printJSON("approved_update", approvedUpdate)
	printJSON("migration_plan", migrationPlan)
	printJSON("operation_spec_plan", operationPlan)
}

func must(label string, err error) {
	if err != nil {
		log.Fatalf("%s: %v", label, err)
	}
}

func printJSON(label string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("== %s ==\n%s\n\n", label, b)
}
