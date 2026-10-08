package orm_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/operation"
	"github.com/recoweft/goquent/orm/query"
	"github.com/recoweft/goquent/tests/typedfixture"
)

func TestGeneratedPatchPlanAndDispatch(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			std, mock, e := sqlmock.New()
			if e != nil {
				t.Fatal("mock unavailable")
			}
			defer std.Close()
			var d driver.Dialect = driver.MySQLDialect{}
			if dialect == "postgres" {
				d = driver.PostgresDialect{}
			}
			db := orm.NewDBWithExecutor(std, d, orm.WithSettings(query.Settings{}))
			s := operation.UpdateSpec{Version: 1, Model: "gq08_records", Filters: []operation.FilterSpec{{Field: "id", Op: "=", Value: int64(9007199254740993)}, {Field: "segment", Op: "=", Value: int32(0)}}, Assignments: []operation.UpdateAssignment{{Column: "active", State: operation.UpdateValue, Value: false}, {Column: "balance", State: operation.UpdateValue, Value: json.Number("-0")}, {Column: "flag", State: operation.UpdateNull}, {Column: "note", State: operation.UpdateValue, Value: ""}}}
			opts := operation.Options{Manifest: typedfixture.Manifest(dialect)}
			want, view, e := db.CompileUpdateWithDiagnostics(t.Context(), s, opts)
			if e != nil {
				t.Fatal("source compile failed")
			}
			var got *orm.QueryPlan
			var dv operation.DiagnosticView
			var exec func() (sql.Result, error)
			if dialect == "mysql" {
				c := typedfixture.MySQLRecordColumns()
				r := typedfixture.NewMySQLRecordRepository(db)
				key := typedfixture.MySQLRecordKey{ID: 9007199254740993, Segment: 0}
				patch := typedfixture.MySQLRecordPatch{Active: c.Active.Set(false), Balance: c.Balance.Set(json.Number("-0")), Flag: c.Flag.SetNull(), Note: c.Note.Set("")}
				got, dv, e = r.PlanUpdateByKey(t.Context(), key, patch, typedfixture.MySQLRecordUpdate{})
				exec = func() (sql.Result, error) {
					return r.UpdateByKey(t.Context(), key, patch, typedfixture.MySQLRecordUpdate{})
				}
			} else {
				c := typedfixture.PostgresRecordColumns()
				r := typedfixture.NewPostgresRecordRepository(db)
				key := typedfixture.PostgresRecordKey{ID: 9007199254740993, Segment: 0}
				patch := typedfixture.PostgresRecordPatch{Active: c.Active.Set(false), Balance: c.Balance.Set(json.Number("-0")), Flag: c.Flag.SetNull(), Note: c.Note.Set("")}
				got, dv, e = r.PlanUpdateByKey(t.Context(), key, patch, typedfixture.PostgresRecordUpdate{})
				exec = func() (sql.Result, error) {
					return r.UpdateByKey(t.Context(), key, patch, typedfixture.PostgresRecordUpdate{})
				}
			}
			if e != nil || got.SQL != want.SQL || !reflect.DeepEqual(got.Params, want.Params) || !reflect.DeepEqual(got.WhereTree, want.WhereTree) || !reflect.DeepEqual(dv, view) {
				t.Fatal("typed/source update diverged")
			}
			generic, e := orm.PlanUpdate(t.Context(), db, struct {
				Active  bool        `db:"active"`
				Balance json.Number `db:"balance"`
				Flag    any         `db:"flag"`
				Note    string      `db:"note"`
				ID      int64       `db:"id,pk"`
				Segment int32       `db:"segment,pk"`
			}{Balance: json.Number("-0"), ID: 9007199254740993}, orm.Table("gq08_records"), orm.PK("id", "segment"), orm.WherePK())
			if e != nil || generic.SQL != want.SQL || !reflect.DeepEqual(generic.Params, want.Params) || !reflect.DeepEqual(generic.WhereTree, want.WhereTree) || !reflect.DeepEqual(generic.Warnings, want.Warnings) {
				t.Fatalf("generic parity SQL=%t args=%t tree=%t warnings=%t", generic.SQL == want.SQL, reflect.DeepEqual(generic.Params, want.Params), reflect.DeepEqual(generic.WhereTree, want.WhereTree), reflect.DeepEqual(generic.Warnings, want.Warnings))
			}
			mock.ExpectExec(regexp.QuoteMeta(want.SQL)).WithArgs(false, "-0", nil, "", int64(9007199254740993), int32(0)).WillReturnResult(sqlmock.NewResult(0, 1))
			if _, e = exec(); e != nil {
				t.Fatal("typed dispatch failed")
			}
			if mock.ExpectationsWereMet() != nil {
				t.Fatal("unexpected dispatch")
			}
		})
	}
}

func TestGeneratedPatchRefusalAndOmission(t *testing.T) {
	std, mock, e := sqlmock.New()
	if e != nil {
		t.Fatal("mock unavailable")
	}
	defer std.Close()
	db := orm.NewDBWithExecutor(std, driver.PostgresDialect{}, orm.WithSettings(query.Settings{}))
	r := typedfixture.NewPostgresRecordRepository(db)
	c := typedfixture.PostgresRecordColumns()
	key := typedfixture.PostgresRecordKey{ID: 1}
	for _, patch := range []typedfixture.PostgresRecordPatch{{}, {Balance: c.Balance.Set(json.Number("1.20"))}, {Status: c.Status.Set(typedfixture.PostgresRecordStatusValue("bad"))}, {Note: typedfixture.PostgresRecordNoteColumn{}.Set("x")}} {
		if _, e := r.UpdateByKey(t.Context(), key, patch, typedfixture.PostgresRecordUpdate{}); e == nil {
			t.Fatal("refused patch executed")
		}
	}
	patch := typedfixture.PostgresRecordPatch{Note: c.Note.Set("private-patch-canary")}
	for _, v := range []any{patch, patch.Note, typedfixture.PostgresRecordUpdate{AccessReason: "private-patch-canary"}} {
		b, e := json.Marshal(v)
		if e != nil || strings.Contains(fmt.Sprintf("%s %v %+v %#v", b, v, v, v), "private-patch-canary") {
			t.Fatal("patch display leaked")
		}
	}
	patch.Note = typedfixture.PostgresRecordNotePatch{}
	if _, _, e := r.PlanUpdateByKey(t.Context(), key, patch, typedfixture.PostgresRecordUpdate{}); !errors.Is(e, operation.ErrEmptyPatch) {
		t.Fatal("cleared patch was not empty")
	}
	if mock.ExpectationsWereMet() != nil {
		t.Fatal("refusal used executor")
	}
}

func TestUpdateReturningBoolPolicyAndRefusal(t *testing.T) {
	for _, policy := range []orm.BoolScanPolicy{orm.BoolStrict, orm.BoolCompat, orm.BoolLenient} {
		std, mock, e := sqlmock.New()
		if e != nil {
			t.Fatal("mock unavailable")
		}
		defer std.Close()
		db := orm.NewDBWithExecutor(std, driver.PostgresDialect{}, orm.WithSettings(query.Settings{}), orm.WithBoolScanPolicy(policy))
		s := operation.UpdateSpec{Version: 1, Model: "gq08_records", Filters: []operation.FilterSpec{{Field: "id", Op: "=", Value: int64(1)}, {Field: "segment", Op: "=", Value: int32(0)}}, Assignments: []operation.UpdateAssignment{{Column: "active", State: operation.UpdateValue, Value: false}}, Returning: []string{"active", "flag"}}
		opts := operation.Options{Manifest: typedfixture.Manifest("postgres")}
		p, _, e := db.CompileUpdateWithDiagnostics(t.Context(), s, opts)
		if e != nil {
			t.Fatal("returning plan failed")
		}
		type row struct {
			Active bool
			Flag   sql.NullBool
		}
		mock.ExpectQuery(regexp.QuoteMeta(p.SQL)).WithArgs(false, int64(1), int32(0)).WillReturnRows(sqlmock.NewRows(s.Returning).AddRow(false, nil))
		v, e := orm.UpdateOperationReturningBy[row](t.Context(), db, s, opts)
		if e != nil || v.Active || v.Flag.Valid {
			t.Fatal("returning bool/null changed")
		}
		mock.ExpectQuery(regexp.QuoteMeta(p.SQL)).WillReturnRows(sqlmock.NewRows(s.Returning).AddRow("2", nil))
		v, e = orm.UpdateOperationReturningBy[row](t.Context(), db, s, opts)
		if policy == orm.BoolLenient {
			if e != nil || !v.Active {
				t.Fatal("lenient bool changed")
			}
		} else if e == nil {
			t.Fatal("bool policy widened")
		}
		s.Returning = []string{"hidden"}
		if _, e = orm.UpdateOperationReturningBy[row](t.Context(), db, s, opts); e == nil {
			t.Fatal("forbidden returning executed")
		}
		if mock.ExpectationsWereMet() != nil {
			t.Fatal("returning dispatch mismatch")
		}
	}
	std, mock, e := sqlmock.New()
	if e != nil {
		t.Fatal("mock unavailable")
	}
	defer std.Close()
	db := orm.NewDBWithExecutor(std, driver.MySQLDialect{}, orm.WithSettings(query.Settings{}))
	r := typedfixture.NewMySQLRecordRepository(db)
	if _, e = r.UpdateIdentityByKey(t.Context(), typedfixture.MySQLRecordKey{ID: 1}, typedfixture.MySQLRecordPatch{Active: typedfixture.MySQLRecordColumns().Active.Set(false)}, typedfixture.MySQLRecordUpdate{}); e == nil {
		t.Fatal("MySQL returning executed")
	}
	if mock.ExpectationsWereMet() != nil {
		t.Fatal("refusal used executor")
	}
}

func TestGeneratedTenantPatchCurrentBinding(t *testing.T) {
	for _, auto := range []bool{false, true} {
		std, mock, e := sqlmock.New()
		if e != nil {
			t.Fatal("mock unavailable")
		}
		defer std.Close()
		cols := []query.WriteKeyColumn{{Name: "id", DBType: "bigint", Bits: 64}, {Name: "tenant_id", DBType: "bigint", Bits: 64}}
		schema, e := query.NewApplicationSchema(query.ApplicationSchemaInput{Database: "fixture", Dialect: "postgres", Tables: []query.ApplicationTable{{Table: "gq08_tenants", PlainTable: true, Columns: append(append([]query.WriteKeyColumn(nil), cols...), query.WriteKeyColumn{Name: "active", DBType: "boolean"}), CompleteUniqueConstraints: true, Constraints: []query.WriteKeyConstraint{{Kind: "primary", AllRows: true, Valid: true, NotDeferrable: true, Columns: cols}}}}})
		if e != nil {
			t.Fatal("schema refused")
		}
		policies, e := query.NewPolicySet(query.TablePolicy{Table: "gq08_tenants", TenantColumn: "tenant_id"})
		if e != nil {
			t.Fatal("policy refused")
		}
		ctx, e := query.NewApplicationTenantContext(query.ExecutionContextInput{CurrentTenant: int64(7), TenantPresent: true})
		if e != nil {
			t.Fatal("context refused")
		}
		settings := query.NewSettings(policies, query.RiskConfig{}, ctx).WithTenantPolicy("fixture", schema, auto)
		db := orm.NewDBWithExecutor(std, driver.PostgresDialect{}, orm.WithSettings(settings))
		r := typedfixture.NewTenantRecordRepository(db)
		patch := typedfixture.TenantRecordPatch{Active: typedfixture.TenantRecordColumns().Active.Set(false)}
		key := typedfixture.TenantRecordKey{ID: 1, TenantID: typedfixture.TenantRecordCurrentTenantKey()}
		p, _, e := r.PlanUpdateByKey(t.Context(), key, patch, typedfixture.TenantRecordUpdate{})
		if e != nil || p.Blocked {
			t.Fatal("tenant update plan refused")
		}
		mock.ExpectExec(regexp.QuoteMeta(p.SQL)).WillReturnResult(sqlmock.NewResult(0, 1))
		if _, e = r.UpdateByKey(t.Context(), key, patch, typedfixture.TenantRecordUpdate{}); e != nil {
			t.Fatal("tenant update failed")
		}
		key.TenantID = typedfixture.TenantRecordTenantIDKey{}
		if _, e = r.UpdateByKey(t.Context(), key, patch, typedfixture.TenantRecordUpdate{}); e == nil {
			t.Fatal("zero tenant marker accepted")
		}
		if mock.ExpectationsWereMet() != nil {
			t.Fatal("tenant dispatch mismatch")
		}
	}
}
