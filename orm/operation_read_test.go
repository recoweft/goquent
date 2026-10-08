package orm_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/operation"
	"github.com/recoweft/goquent/orm/query"
	"github.com/recoweft/goquent/tests/typedfixture"
)

func TestGeneratedReadPlanAndDispatch(t *testing.T) {
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			std, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer std.Close()
			var d driver.Dialect = driver.MySQLDialect{}
			if dialect == "postgres" {
				d = driver.PostgresDialect{}
			}
			db := orm.NewDBWithExecutor(std, d, orm.WithSettings(query.Settings{}))
			limit := int64(0)
			spec := operation.OperationSpec{Version: 1, Operation: "select", Model: "gq08_records", Select: []string{"segment", "id"}, Filters: []operation.FilterSpec{{Field: "id", Op: "=", Value: int64(9007199254740993)}, {Field: "segment", Op: "in", Value: []any{int32(0), int32(2)}}}, OrderBy: []operation.OrderSpec{{Field: "id", Direction: "desc"}}, Limit: &limit}
			want, dv, err := db.CompileOperationWithDiagnostics(t.Context(), spec, operation.Options{Manifest: typedfixture.Manifest(dialect)})
			if err != nil {
				t.Fatal(err)
			}
			var got *orm.QueryPlan
			var view operation.DiagnosticView
			if dialect == "mysql" {
				c := typedfixture.MySQLRecordColumns()
				got, view, err = typedfixture.NewMySQLRecordRepository(db).PlanIdentity(t.Context(), typedfixture.MySQLRecordRead{Filters: []typedfixture.MySQLRecordPredicate{c.ID.Eq(9007199254740993), c.Segment.In(0, 2)}, OrderBy: []typedfixture.MySQLRecordOrder{c.ID.Desc()}, Limit: &limit})
			} else {
				c := typedfixture.PostgresRecordColumns()
				got, view, err = typedfixture.NewPostgresRecordRepository(db).PlanIdentity(t.Context(), typedfixture.PostgresRecordRead{Filters: []typedfixture.PostgresRecordPredicate{c.ID.Eq(9007199254740993), c.Segment.In(0, 2)}, OrderBy: []typedfixture.PostgresRecordOrder{c.ID.Desc()}, Limit: &limit})
			}
			if err != nil || !reflect.DeepEqual(got.Params, want.Params) || got.SQL != want.SQL || !reflect.DeepEqual(got.WhereTree, want.WhereTree) || !reflect.DeepEqual(got.Warnings, want.Warnings) || got.Blocked != want.Blocked || !reflect.DeepEqual(view, dv) {
				t.Fatal("generated/dynamic planning diverged")
			}
			mock.ExpectQuery(regexp.QuoteMeta(want.SQL)).WithArgs(int64(9007199254740993), int32(0), int32(2)).WillReturnRows(sqlmock.NewRows([]string{"segment", "id"}))
			// A caller option cannot replace the DB dialect/settings or executor.
			_, err = orm.SelectOperationBy[struct {
				ID      int64
				Segment int32
			}](t.Context(), db, spec, operation.Options{Manifest: typedfixture.Manifest(dialect), Dialect: driver.PostgresDialect{}})
			if err != nil {
				t.Fatal("read dispatch refused")
			}
			if err = mock.ExpectationsWereMet(); err != nil {
				t.Fatal("dispatch did not match original arguments")
			}
		})
	}
}
func TestGeneratedRuntimeConstraintsAndOmission(t *testing.T) {
	db := orm.NewDBWithExecutor(nil, driver.PostgresDialect{}, orm.WithSettings(query.Settings{}))
	r := typedfixture.NewPostgresRecordRepository(db)
	c := typedfixture.PostgresRecordColumns()
	for _, tc := range []struct {
		name     string
		filter   typedfixture.PostgresRecordPredicate
		blocked  bool
		sentinel error
	}{
		{"enum", c.Status.Eq(typedfixture.PostgresRecordStatusValue("not-a-member")), false, operation.ErrTypeMismatch},
		{"decimal-mismatch", c.Balance.Eq(json.Number("1.12345")), false, operation.ErrTypeMismatch},
		{"decimal-private-domain", c.Balance.Eq(json.Number("9007199254740993.1234")), true, nil},
		{"date", c.Happened.Eq("2024-02-30"), false, operation.ErrTypeMismatch},
		{"null", c.Note.IsNull(), false, nil},
		{"zero-reference", typedfixture.PostgresRecordPredicate{}, false, operation.ErrInvalidFilter},
		{"empty-IN", c.ID.In(), false, operation.ErrInvalidFilter},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, _, err := r.PlanIdentity(t.Context(), typedfixture.PostgresRecordRead{Filters: []typedfixture.PostgresRecordPredicate{tc.filter}})
			if tc.sentinel != nil {
				if !errors.Is(err, tc.sentinel) {
					t.Fatal("wrong runtime refusal")
				}
			} else if err != nil || p.Blocked != tc.blocked {
				t.Fatal("unknown/valid determination changed")
			}
		})
	}
	canary := "private-source-canary"
	values := []any{typedfixture.PostgresRecordRow{Note: sql.Null[string]{V: canary, Valid: true}}, typedfixture.PostgresRecordRead{AccessReason: canary}, c.Status.Eq(typedfixture.PostgresRecordStatusValue(canary)), typedfixture.PostgresRecordStatusValue(canary), typedfixture.PostgresRecordIDKey(9007199254740993), r, c}
	for _, v := range values {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		display := fmt.Sprintf("%s %v %+v %#v", data, v, v, v)
		if strings.Contains(display, canary) || strings.Contains(display, "9007199254740993") || strings.Contains(display, "fictional-schema") {
			t.Fatal("generated display leaked source")
		}
	}
}
func TestOperationReadRefusesBeforeExecutor(t *testing.T) {
	for _, strict := range []bool{false, true} {
		t.Run(fmt.Sprint(strict), func(t *testing.T) {
			std, mock, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer std.Close()
			settings := query.Settings{}
			if strict {
				schema, err := query.NewApplicationSchema(query.ApplicationSchemaInput{Database: "fixture", Dialect: "postgres", Tables: []query.ApplicationTable{{Table: "gq08_records", PlainTable: true, Columns: []query.WriteKeyColumn{{Name: "id", DBType: "bigint", Bits: 64}}}}})
				if err != nil {
					t.Fatal(err)
				}
				settings = query.Settings{}.WithTenantPolicy("fixture", schema, false)
			}
			db := orm.NewDBWithExecutor(std, driver.PostgresDialect{}, orm.WithSettings(settings))
			m := typedfixture.Manifest("postgres")
			s := operation.OperationSpec{Operation: "select", Model: "gq08_records", Select: []string{"id"}}
			cases := []struct {
				s operation.OperationSpec
				m *manifest.Manifest
			}{{s, m}}
			unknown := typedfixture.Manifest("postgres")
			unknown.Tables[0].Columns[0].TypeSource = ""
			cases[0].m = unknown
			bad := s
			bad.Filters = []operation.FilterSpec{{Field: "id", Op: "=", Value: 1.5}}
			cases = append(cases, struct {
				s operation.OperationSpec
				m *manifest.Manifest
			}{bad, m})
			for _, tc := range cases {
				_, e = orm.SelectOperationBy[struct{ ID int64 }](t.Context(), db, tc.s, operation.Options{Manifest: tc.m})
				if e == nil {
					t.Fatal("refusal dispatched")
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			_, e = orm.SelectOperationBy[struct{ ID int64 }](ctx, db, s, operation.Options{Manifest: m})
			if e == nil {
				t.Fatal("cancellation accepted")
			}
			if e = mock.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestOperationReadScannerPolicyAndErrors(t *testing.T) {
	type row struct {
		Active   bool             `db:"active"`
		Flag     sql.NullBool     `db:"flag"`
		Note     sql.Null[string] `db:"note"`
		Balance  string           `db:"balance"`
		Happened time.Time        `db:"happened"`
	}
	for _, policy := range []orm.BoolScanPolicy{orm.BoolStrict, orm.BoolCompat, orm.BoolLenient} {
		t.Run(fmt.Sprint(policy), func(t *testing.T) {
			std, mock, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer std.Close()
			db := orm.NewDBWithExecutor(std, driver.PostgresDialect{}, orm.WithBoolScanPolicy(policy), orm.WithSettings(query.Settings{}))
			spec := operation.OperationSpec{Operation: "select", Model: "gq08_records", Select: []string{"active", "flag", "note", "balance", "happened"}}
			opts := operation.Options{Manifest: typedfixture.Manifest("postgres")}
			p, e := db.CompileOperation(t.Context(), spec, opts)
			if e != nil {
				t.Fatal(e)
			}
			now := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
			mock.ExpectQuery(regexp.QuoteMeta(p.SQL)).WillReturnRows(sqlmock.NewRows(spec.Select).AddRow(true, false, nil, []byte("999999999999.1234"), now))
			rows, e := orm.SelectOperationBy[row](t.Context(), db, spec, opts)
			if e != nil || len(rows) != 1 || !rows[0].Active || !rows[0].Flag.Valid || rows[0].Flag.Bool || rows[0].Note.Valid || rows[0].Balance != "999999999999.1234" || !rows[0].Happened.Equal(now) {
				t.Fatal("generic scanner behavior changed")
			}
			mock.ExpectQuery(regexp.QuoteMeta(p.SQL)).WillReturnRows(sqlmock.NewRows(spec.Select).AddRow("bad", nil, nil, "1", now))
			if _, e = orm.SelectOperationBy[row](t.Context(), db, spec, opts); e == nil {
				t.Fatal("invalid bool accepted")
			}
			mock.ExpectQuery(regexp.QuoteMeta(p.SQL)).WillReturnRows(sqlmock.NewRows(spec.Select).AddRow("2", nil, nil, "1", now))
			lenientRows, lenientErr := orm.SelectOperationBy[row](t.Context(), db, spec, opts)
			if policy == orm.BoolLenient {
				if lenientErr != nil || len(lenientRows) != 1 || !lenientRows[0].Active {
					t.Fatal("lenient bool path changed")
				}
			} else if lenientErr == nil {
				t.Fatal("strict/compat bool path widened")
			}
			failure := errors.New("fixture query failure")
			mock.ExpectQuery(regexp.QuoteMeta(p.SQL)).WillReturnError(failure)
			if _, e = orm.SelectOperationBy[row](t.Context(), db, spec, opts); !errors.Is(e, failure) {
				t.Fatal("execution error identity lost")
			}
			if e = mock.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestGeneratedClockScannerRejectsLossyConversion(t *testing.T) {
	var value typedfixture.PostgresRecordTimeText
	if err := value.Scan(time.Date(0, time.January, 1, 12, 34, 56, 123456000, time.UTC)); err != nil || value != "12:34:56.123456" {
		t.Fatal("clock precision changed")
	}
	if err := value.Scan(time.Date(0, time.January, 2, 0, 0, 0, 0, time.UTC)); err == nil {
		t.Fatal("24-hour driver result silently normalized")
	}
	if err := value.Scan(float64(1)); err == nil {
		t.Fatal("unsupported result converted")
	}
}
