package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/recoweft/goquent/orm/query"
	"log"
	"os"
	"time"

	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/migration"
	"github.com/recoweft/goquent/orm/operation"
)

type User struct {
	ID        int64      `db:"id,pk"`
	TenantID  int64      `db:"tenant_id"`
	Name      string     `db:"name"`
	Email     string     `db:"email"`
	DeletedAt *time.Time `db:"deleted_at,omitempty"`
}

func (User) TableName() string { return "users" }

func main() {
	mode := "plan"
	if len(os.Args) == 2 {
		mode = os.Args[1]
	} else if len(os.Args) != 1 {
		os.Exit(2)
	}
	os.Exit(run(mode))
}

// noDatabase has no driver or connection. Any unexpected dispatch fails closed.
type noDatabase struct{ calls int }

var errDatabaseDisabled = errors.New("EXAMPLE_DATABASE_DISABLED")

func (e *noDatabase) Exec(string, ...any) (sql.Result, error) {
	e.calls++
	return nil, errDatabaseDisabled
}
func (e *noDatabase) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	e.calls++
	return nil, errDatabaseDisabled
}
func (e *noDatabase) Query(string, ...any) (*sql.Rows, error) {
	e.calls++
	return nil, errDatabaseDisabled
}
func (e *noDatabase) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	e.calls++
	return nil, errDatabaseDisabled
}
func (e *noDatabase) QueryRow(string, ...any) *sql.Row {
	e.calls++
	panic(errDatabaseDisabled)
}
func (e *noDatabase) QueryRowContext(context.Context, string, ...any) *sql.Row {
	e.calls++
	panic(errDatabaseDisabled)
}

// denyOnly never grants permission and is used only by the refusal example.
type denyOnly struct{ calls int }

func (d *denyOnly) Authorize(_ context.Context, r query.AuthorizationRequest) (query.AuthorizationDecision, error) {
	d.calls++
	return r.Deny(), nil
}

func run(mode string) (code int) {
	defer func() {
		if recover() != nil {
			fmt.Fprintln(os.Stderr, "EXAMPLE_EXECUTION_REFUSED")
			code = 1
		}
	}()
	ctx := context.Background()
	executor := &noDatabase{}
	defer func() {
		if executor.calls != 0 {
			fmt.Fprintln(os.Stderr, "EXAMPLE_UNEXPECTED_DISPATCH")
			code = 1
		}
	}()
	db := orm.NewDBWithExecutor(executor, driver.MySQLDialect{})

	orm.ResetModelPolicies()
	if err := orm.Model(User{}).
		Table("users").
		TenantScoped("tenant_id").
		SoftDelete("deleted_at").
		RequiredFilter("tenant_id").
		PII("email").
		Register(); err != nil {
		log.Fatal("PUBLIC_OUTPUT: operation failed; details omitted")
	}

	m, err := manifest.Generate(manifest.Options{
		Dialect:     "mysql",
		GeneratedAt: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
		Schema: &migration.Schema{Tables: []migration.TableSchema{{
			Name: "users",
			Columns: []migration.ColumnSchema{
				{Name: "id", Type: "bigint", Nullable: false, NullableKnown: true},
				{Name: "tenant_id", Type: "bigint", Nullable: false, NullableKnown: true},
				{Name: "name", Type: "varchar(255)", Nullable: false, NullableKnown: true},
				{Name: "email", Type: "varchar(255)", Nullable: false, NullableKnown: true},
				{Name: "deleted_at", Type: "datetime", Nullable: true, NullableKnown: true},
			},
			Indexes: []migration.IndexSchema{{Name: "idx_users_tenant_id", Columns: []string{"tenant_id"}}},
		}}},
		Policies: orm.RegisteredTablePolicies(),
	})
	must("manifest", err)

	// These assertions are supplied by the trusted application, never by JSON.
	policies, err := query.NewPolicySet(orm.RegisteredTablePolicies()...)
	must("policy snapshot", err)
	schema, err := query.NewApplicationSchema(query.ApplicationSchemaInput{Database: "example", Dialect: "mysql", Tables: []query.ApplicationTable{{Table: "users", PlainTable: true, CompleteUniqueConstraints: true, Columns: []query.WriteKeyColumn{
		{Name: "id", DBType: "BIGINT", Bits: 64}, {Name: "tenant_id", DBType: "BIGINT", Bits: 64}, {Name: "name", DBType: "varchar(255)"}, {Name: "email", DBType: "varchar(255)"}, {Name: "deleted_at", DBType: "datetime", Nullable: true},
	}}}})
	must("application schema", err)
	execution, err := query.NewApplicationTenantContext(query.ExecutionContextInput{TenantPresent: true, CurrentTenant: int64(123)})
	must("application tenant", err)
	settings := query.NewSettings(policies, query.RiskConfig{}, execution).WithTenantPolicy("example", schema, false)
	db = db.WithOptions(orm.WithSettings(settings))
	orm.ResetModelPolicies()

	limit := int64(25)
	operationPlan, err := db.CompileOperation(ctx, operation.OperationSpec{
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
	})
	must("operation spec compile", err)

	printJSON("typed_operation", operationPlan)
	printJSON("manifest_summary", m)

	// Fixture-only ephemeral key. Production keys and current inputs belong to
	// the application; no key, scope, SQL or source artifact is printed.
	key := make([]byte, 32)
	_, err = rand.Read(key)
	must("fixture key", err)
	binding, err := query.NewBindingContext(query.BindingContextInput{
		Key: key, Scope: "example-only", Generation: "1", Target: "example", Dialect: "mysql",
	})
	must("binding", err)
	current := query.BindingCurrent{Settings: settings, BindingContext: binding}
	q := db.Table("users").Select("id", "name").Where("tenant_id", int64(123)).Limit(25)
	planned, err := q.Plan(ctx)
	must("read plan", err)
	printJSON("read_plan", planned)
	_, diagnostic, err := q.ValidateSelect(ctx, current, time.Now().Add(time.Minute))
	must("validate read", err)
	printJSON("validated_read_diagnostic", diagnostic)

	switch mode {
	case "plan":
		// No execution terminal is called on the default path.
	case "refuse-tenant":
		_, _, err := db.Table("users").Select("id").Limit(1).
			ValidateSelect(ctx, current, time.Now().Add(time.Minute))
		if !errors.Is(err, query.ErrBlockedOperation) {
			return 1
		}
		fmt.Fprintln(os.Stderr, "EXAMPLE_TENANT_REFUSED")
		return 3
	case "refuse-authorization":
		// A deny-only local adapter demonstrates refusal, not a production
		// authentication service or a statement/value approval protocol.
		provider := &denyOnly{}
		registration, err := query.NewAuthorization(provider, "fictional-subject")
		must("registration", err)
		high := query.RiskHigh
		settings = settings.WithAuthorization(registration).WithRiskConfig(query.RiskConfig{
			Rules: map[string]query.RiskRuleConfig{query.WarningLimitMissing: {Severity: &high}},
		})
		db = db.WithOptions(orm.WithSettings(settings))
		current.Settings = settings
		q = db.Table("users").Select("id").Where("tenant_id", int64(123)).RequireApproval("fixture intent only")
		handle, pending, err := q.ValidateSelect(ctx, current, time.Now().Add(time.Minute))
		must("pending candidate", err)
		if handle == nil || pending == nil || !pending.Blocked || provider.calls != 0 {
			return 1
		}
		printJSON("authorization_pending", pending)
		var rows []User
		err = q.ExecuteValidatedSelect(ctx, current, handle, &rows)
		if !errors.Is(err, query.ErrAuthorizationDenied) || provider.calls != 1 {
			return 1
		}
		if !errors.Is(q.ExecuteValidatedSelect(ctx, current, handle, &rows), query.ErrBindingConsumed) || provider.calls != 1 {
			return 1
		}
		fmt.Fprintln(os.Stderr, "EXAMPLE_AUTHORIZATION_DENIED_CONSUMED")
		return 4
	default:
		fmt.Fprintln(os.Stderr, "EXAMPLE_USAGE")
		return 2
	}
	return 0
}

func must(label string, err error) {
	if err != nil {
		log.Fatal("PUBLIC_OUTPUT: operation failed; details omitted")
	}
}

func printJSON(label string, v any) {
	var b []byte
	var err error
	switch p := v.(type) {
	case *query.QueryPlan:
		view, e := p.PublicView()
		err = e
		if err == nil {
			b, err = view.ToJSON()
		}
	case *manifest.Manifest:
		b, err = json.Marshal(p.PublicView())
	case *migration.MigrationPlan:
		view, e := p.PublicView()
		err = e
		if err == nil {
			b, err = view.ToJSON()
		}
	default:
		log.Fatal("PUBLIC_OUTPUT: unsupported example output")
	}
	if err != nil {
		log.Fatal("PUBLIC_OUTPUT: operation failed; details omitted")
	}

	fmt.Printf("== %s ==\n%s\n\n", label, b)
}
