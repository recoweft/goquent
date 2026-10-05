package tests

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/recoweft/goquent/orm"
)

type settingsExternalExecutor struct {
	*sql.Tx
	calls atomic.Int64
}

func (e *settingsExternalExecutor) QueryContext(ctx context.Context, sql string, args ...any) (*sql.Rows, error) {
	e.calls.Add(1)
	return e.Tx.QueryContext(ctx, sql, args...)
}

func TestSettingsDatabaseIsolationAndTransactions(t *testing.T) {
	for _, config := range []struct{ name, env, dsn string }{{orm.MySQL, "TEST_MYSQL_DSN", defaultMySQLTestDSN}, {orm.Postgres, "TEST_POSTGRES_DSN", defaultPostgresTestDSN}} {
		t.Run(config.name, func(t *testing.T) {
			dsn, explicit := lookupTestDSN(config.env, config.dsn)
			root := openTestDB(t, config.name, dsn, explicit)
			defer root.Close()
			var version string
			if err := root.SQLDB().QueryRow("SELECT version()").Scan(&version); err != nil {
				t.Fatal(err)
			}
			t.Logf("server: %s", version)
			const table = "gq_settings_rows"
			if _, err := root.SQLDB().Exec("CREATE TABLE " + table + " (id INTEGER PRIMARY KEY, tenant_a INTEGER, tenant_b INTEGER)"); err != nil {
				t.Fatal(err)
			}
			defer root.SQLDB().Exec("DROP TABLE " + table)
			if _, err := root.SQLDB().Exec("INSERT INTO " + table + " VALUES (1,1,10),(2,10,2)"); err != nil {
				t.Fatal(err)
			}
			var dbs []*orm.DB
			for i, col := range []string{"tenant_a", "tenant_b"} {
				p, err := orm.NewPolicySet(orm.TablePolicy{Table: table, TenantColumn: col, TenantMode: orm.PolicyModeBlock})
				if err != nil {
					t.Fatal(err)
				}
				c, err := orm.NewExecutionContext(orm.ExecutionContextInput{Source: "application-test", TenantPresent: true, CurrentTenant: int64(i + 1)})
				if err != nil {
					t.Fatal(err)
				}
				level := orm.RiskMedium
				if i == 0 {
					level = orm.RiskHigh
				}
				db, err := orm.OpenWithDriverOptions(config.name, dsn, orm.WithPolicySet(p), orm.WithRiskConfig(orm.RiskConfig{Rules: map[string]orm.RiskRuleConfig{orm.WarningLimitMissing: {Severity: &level}}}), orm.WithExecutionContext(c), orm.WithBoolScanPolicy(orm.BoolCompat))
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				dbs = append(dbs, db)
			}
			check := func(db *orm.DB, i int) error {
				input := db.Settings().ExecutionContext().Input()
				if input.Source != "application-test" || !input.TenantPresent || input.CurrentTenant != int64(i+1) {
					return fmt.Errorf("context not inherited: %+v", input)
				}
				col := []string{"tenant_a", "tenant_b"}[i]
				q := db.Table(table).Select("id").Where(col, int64(i+1)).WithContext(context.Background())
				plan, err := q.Plan(context.Background())
				if err != nil {
					return err
				}
				want := orm.RiskMedium
				if i == 0 {
					want = orm.RiskHigh
				}
				if plan.Blocked || plan.RiskLevel != want {
					return fmt.Errorf("mixed risk/policy: %s blocked=%v", plan.RiskLevel, plan.Blocked)
				}
				missing, err := db.Table(table).Select("id").Limit(1).Plan(context.Background())
				if err != nil || !missing.Blocked {
					return fmt.Errorf("missing tenant policy did not block: %v", err)
				}
				var row struct {
					ID int `db:"id"`
				}
				if err := q.Limit(1).First(&row); err != nil {
					return err
				}
				if row.ID != i+1 {
					return fmt.Errorf("wrong row: %d", row.ID)
				}
				n, err := db.Table(table).Where(col, int64(i+1)).WithContext(context.Background()).Count()
				if err != nil || n != 1 {
					return fmt.Errorf("count: %d %v", n, err)
				}
				// Generic scanning/raw approval retains BoolCompat; it does not establish tenant scope.
				active, err := orm.SelectOne[struct {
					Active bool `db:"active"`
				}](context.Background(), db.RequireRawApproval("test raw scalar scan"), "SELECT 1 AS active")
				if err != nil || !active.Active {
					return fmt.Errorf("bool scan: %v %v", active, err)
				}
				return nil
			}
			var wg sync.WaitGroup
			for i, db := range dbs {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for k := 0; k < 8; k++ {
						if err := check(db.Clone(), i); err != nil {
							t.Error(err)
							return
						}
						if err := db.TransactionContext(context.Background(), func(tx orm.Tx) error {
							if err := check(tx.DB, i); err != nil {
								return err
							}
							ext := &settingsExternalExecutor{Tx: tx.Tx.Tx}
							inherited := tx.WrapExecutor(ext)
							if err := inherited.Close(); err != nil {
								return err
							}
							if err := check(inherited, i); err != nil {
								return err
							}
							if ext.calls.Load() == 0 {
								return fmt.Errorf("custom executor not called")
							}
							standalone := orm.NewTxDB(tx.Tx.Tx, db.Dialect(), orm.WithSettings(db.Settings()))
							if err := check(standalone, i); err != nil {
								return err
							}
							if err := check(db.WrapTx(tx.Tx.Tx), i); err != nil {
								return err
							}
							return check(orm.NewDBWithExecutor(ext, db.Dialect(), orm.WithSettings(db.Settings())), i)
						}); err != nil {
							t.Error(err)
							return
						}
					}
				}()
			}
			wg.Wait()
		})
	}
}
