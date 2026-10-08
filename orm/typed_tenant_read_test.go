package orm_test

import (
	"errors"
	"reflect"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/operation"
	"github.com/recoweft/goquent/orm/query"
	"github.com/recoweft/goquent/tests/typedfixture"
)

func TestGeneratedTrustedTenantKey(t *testing.T) {
	for _, automatic := range []bool{false, true} {
		t.Run(map[bool]string{false: "manual", true: "automatic"}[automatic], func(t *testing.T) {
			std, mock, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer std.Close()
			schema, e := query.NewApplicationSchema(query.ApplicationSchemaInput{Database: "fixture", Dialect: "postgres", Tables: []query.ApplicationTable{{Table: "gq08_tenants", PlainTable: true, CompleteUniqueConstraints: true, Columns: []query.WriteKeyColumn{{Name: "id", DBType: "bigint", Bits: 64}, {Name: "tenant_id", DBType: "bigint", Bits: 64}, {Name: "active", DBType: "boolean"}}}}})
			if e != nil {
				t.Fatal(e)
			}
			policies, e := query.NewPolicySet(query.TablePolicy{Table: "gq08_tenants", TenantColumn: "tenant_id"})
			if e != nil {
				t.Fatal(e)
			}
			execution, e := query.NewApplicationTenantContext(query.ExecutionContextInput{CurrentTenant: int64(7), TenantPresent: true})
			if e != nil {
				t.Fatal(e)
			}
			settings := query.NewSettings(policies, query.RiskConfig{}, execution).WithTenantPolicy("fixture", schema, automatic)
			db := orm.NewDBWithExecutor(std, driver.PostgresDialect{}, orm.WithSettings(settings))
			repo := typedfixture.NewTenantRecordRepository(db)
			key := typedfixture.TenantRecordKey{ID: 0, TenantID: typedfixture.TenantRecordCurrentTenantKey()}
			p, v, e := repo.PlanFindByKey(t.Context(), key, typedfixture.TenantRecordRead{})
			if e != nil || p.Blocked {
				t.Fatal("typed tenant key refused")
			}
			spec := operation.OperationSpec{Version: 1, Operation: "select", Model: "gq08_tenants", Select: []string{"active", "id", "tenant_id"}, Filters: []operation.FilterSpec{{Field: "id", Op: "=", Value: int64(0)}, {Field: "tenant_id", Op: "=", ValueRef: "current_tenant"}}}
			want, wv, e := db.CompileOperationWithDiagnostics(t.Context(), spec, operation.Options{Manifest: typedfixture.TenantManifest()})
			if e != nil || p.SQL != want.SQL || !reflect.DeepEqual(p.Params, want.Params) || !reflect.DeepEqual(v, wv) {
				t.Fatal("trusted dynamic parity lost")
			}
			mock.ExpectQuery(regexp.QuoteMeta(p.SQL)).WillReturnRows(sqlmock.NewRows([]string{"active", "id", "tenant_id"}).AddRow(true, 0, 7))
			row, e := repo.FindByKey(t.Context(), key, typedfixture.TenantRecordRead{})
			if e != nil || row.ID != 0 || row.TenantID != 7 || !row.Active {
				t.Fatal("trusted key read failed")
			}
			bad := typedfixture.TenantRecordKey{ID: 0}
			if _, e = repo.FindByKey(t.Context(), bad, typedfixture.TenantRecordRead{}); e == nil {
				t.Fatal("missing tenant marker accepted")
			}
			without := orm.NewDBWithExecutor(std, driver.PostgresDialect{}, orm.WithSettings(settings.WithExecutionContext(query.ExecutionContext{})))
			if _, e = typedfixture.NewTenantRecordRepository(without).FindByKey(t.Context(), key, typedfixture.TenantRecordRead{}); !errors.Is(e, operation.ErrReservedBinding) {
				t.Fatal("missing application context accepted")
			}
			replacement := query.Settings{}
			if _, e = orm.SelectOperationBy[struct{ ID int64 }](t.Context(), without, spec, operation.Options{Manifest: typedfixture.TenantManifest(), Settings: &replacement, Values: map[string]any{"current_tenant": int64(7)}}); !errors.Is(e, operation.ErrReservedBinding) {
				t.Fatal("caller tenant/settings override accepted")
			}
			if e = mock.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
