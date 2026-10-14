package tests

import (
	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/query"
	"github.com/recoweft/goquent/tests/typedfixture"
	"testing"
)

func TestGeneratedTenantPatchDatabase(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var root *orm.DB
			var d driver.Dialect = driver.MySQLDialect{}
			if dialect == "mysql" {
				root = setupDB(t)
			} else {
				root = setupPgDB(t)
				d = driver.PostgresDialect{}
			}
			defer root.Close()
			std := root.SQLDB()
			if _, e := std.Exec("CREATE TABLE gq08_tenants (id BIGINT NOT NULL,tenant_id BIGINT NOT NULL,active BOOLEAN NOT NULL,PRIMARY KEY(id,tenant_id))"); e != nil {
				t.Fatal("tenant table setup failed")
			}
			defer std.Exec("DROP TABLE gq08_tenants")
			if _, e := std.Exec("INSERT INTO gq08_tenants VALUES (1,7,true),(1,8,true)"); e != nil {
				t.Fatal("tenant fixture setup failed")
			}
			for _, automatic := range []bool{false, true} {
				cols := []query.WriteKeyColumn{{Name: "id", DBType: "bigint", Bits: 64}, {Name: "tenant_id", DBType: "bigint", Bits: 64}, {Name: "active", DBType: "boolean"}}
				if dialect == "mysql" {
					cols[0].DBType = "BIGINT"
					cols[1].DBType = "BIGINT"
				}
				schema, e := query.NewApplicationSchema(query.ApplicationSchemaInput{Database: "fixture", Dialect: dialect, Tables: []query.ApplicationTable{{Table: "gq08_tenants", PlainTable: true, Columns: cols, CompleteUniqueConstraints: true, Constraints: []query.WriteKeyConstraint{{Kind: "primary", Columns: cols[:2], AllRows: true, Valid: true, NotDeferrable: true}}}}})
				if e != nil {
					t.Fatal("tenant schema failed")
				}
				policies, e := query.NewPolicySet(query.TablePolicy{Table: "gq08_tenants", TenantColumn: "tenant_id"})
				if e != nil {
					t.Fatal("tenant policy failed")
				}
				context, e := query.NewApplicationTenantContext(query.ExecutionContextInput{CurrentTenant: int64(7), TenantPresent: true})
				if e != nil {
					t.Fatal("tenant context failed")
				}
				settings := query.NewSettings(policies, query.RiskConfig{}, context).WithTenantPolicy("fixture", schema, automatic)
				counted := &genericDatabaseExecutor{Executor: std}
				db := orm.NewDBWithExecutor(counted, d, orm.WithSettings(settings))
				if dialect == "mysql" {
					r := typedfixture.NewMySQLTenantRecordRepository(db)
					key := typedfixture.MySQLTenantRecordKey{ID: 1, TenantID: typedfixture.MySQLTenantRecordCurrentTenantKey()}
					patch := typedfixture.MySQLTenantRecordPatch{Active: typedfixture.MySQLTenantRecordColumns().Active.Set(false)}
					if _, _, e = r.PlanUpdateByKey(t.Context(), key, patch, typedfixture.MySQLTenantRecordUpdate{}); e != nil || counted.calls.Load() != 0 {
						t.Fatal("tenant plan failed or dispatched")
					}
					if _, e = r.UpdateByKey(t.Context(), key, patch, typedfixture.MySQLTenantRecordUpdate{}); e != nil {
						t.Fatal("tenant patch failed")
					}
					key.TenantID = typedfixture.MySQLTenantRecordTenantIDKey{}
					if _, e = r.UpdateByKey(t.Context(), key, patch, typedfixture.MySQLTenantRecordUpdate{}); e == nil || counted.calls.Load() != 1 {
						t.Fatal("zero tenant marker dispatched")
					}
				} else {
					r := typedfixture.NewTenantRecordRepository(db)
					key := typedfixture.TenantRecordKey{ID: 1, TenantID: typedfixture.TenantRecordCurrentTenantKey()}
					patch := typedfixture.TenantRecordPatch{Active: typedfixture.TenantRecordColumns().Active.Set(false)}
					if _, _, e = r.PlanUpdateByKey(t.Context(), key, patch, typedfixture.TenantRecordUpdate{}); e != nil || counted.calls.Load() != 0 {
						t.Fatal("tenant plan failed or dispatched")
					}
					if _, e = r.UpdateByKey(t.Context(), key, patch, typedfixture.TenantRecordUpdate{}); e != nil {
						t.Fatal("tenant patch failed")
					}
					key.TenantID = typedfixture.TenantRecordTenantIDKey{}
					if _, e = r.UpdateByKey(t.Context(), key, patch, typedfixture.TenantRecordUpdate{}); e == nil || counted.calls.Load() != 1 {
						t.Fatal("zero tenant marker dispatched")
					}
				}
				var own, other bool
				if std.QueryRow("SELECT active FROM gq08_tenants WHERE id=1 AND tenant_id=7").Scan(&own) != nil || std.QueryRow("SELECT active FROM gq08_tenants WHERE id=1 AND tenant_id=8").Scan(&other) != nil || own || !other {
					t.Fatal("tenant isolation result changed")
				}
			}
		})
	}
}
