package orm_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/operation"
	"github.com/recoweft/goquent/orm/query"
	"github.com/recoweft/goquent/tests/typedfixture"
)

type patchCounter struct {
	orm.Executor
	calls int
}

func (e *patchCounter) Exec(s string, a ...any) (sql.Result, error) {
	e.calls++
	return e.Executor.Exec(s, a...)
}
func (e *patchCounter) ExecContext(c context.Context, s string, a ...any) (sql.Result, error) {
	e.calls++
	return e.Executor.ExecContext(c, s, a...)
}
func (e *patchCounter) Query(s string, a ...any) (*sql.Rows, error) {
	e.calls++
	return e.Executor.Query(s, a...)
}
func (e *patchCounter) QueryContext(c context.Context, s string, a ...any) (*sql.Rows, error) {
	e.calls++
	return e.Executor.QueryContext(c, s, a...)
}
func (e *patchCounter) QueryRow(s string, a ...any) *sql.Row {
	e.calls++
	return e.Executor.QueryRow(s, a...)
}
func (e *patchCounter) QueryRowContext(c context.Context, s string, a ...any) *sql.Row {
	e.calls++
	return e.Executor.QueryRowContext(c, s, a...)
}

func TestUpdateRefusedExecutorAndTrustedSettings(t *testing.T) {
	std, _, e := sqlmock.New()
	if e != nil {
		t.Fatal("mock unavailable")
	}
	defer std.Close()
	counted := &patchCounter{Executor: std}
	policies, e := query.NewPolicySet(query.TablePolicy{Table: "gq08_records", ImmutableColumns: []string{"active"}})
	if e != nil {
		t.Fatal("policy failed")
	}
	settings := query.Settings{}.WithPolicySet(policies)
	db := orm.NewDBWithExecutor(counted, driver.PostgresDialect{}, orm.WithSettings(settings))
	replacement := query.Settings{}
	opts := operation.Options{Manifest: typedfixture.Manifest("postgres"), Settings: &replacement, Dialect: driver.MySQLDialect{}}
	for _, assignment := range []operation.UpdateAssignment{
		{Column: "active", State: operation.UpdateValue, Value: false},
		{Column: "note", State: operation.UpdateNull, ValuePresent: true},
		{Column: "computed", State: operation.UpdateUnchanged},
		{Column: "balance", State: operation.UpdateValue, Value: json.Number("1.20")},
	} {
		s := operation.UpdateSpec{Version: 1, Model: "gq08_records", Assignments: []operation.UpdateAssignment{assignment}, Filters: []operation.FilterSpec{{Field: "id", Op: "=", Value: int64(1)}, {Field: "segment", Op: "=", Value: int32(0)}}}
		_, _, _ = db.CompileUpdateWithDiagnostics(t.Context(), s, opts)
		if counted.calls != 0 {
			t.Fatal("planning dispatched")
		}
		if _, e := orm.UpdateOperationBy(t.Context(), db, s, opts); e == nil || counted.calls != 0 {
			t.Fatal("refusal/settings override dispatched")
		}
	}
	// An absent key cannot turn into a write authorization or unconditional update.
	s := operation.UpdateSpec{Version: 1, Model: "gq08_records", Assignments: []operation.UpdateAssignment{{Column: "note", State: operation.UpdateValue, Value: ""}}}
	if _, e := orm.UpdateOperationBy(t.Context(), db, s, opts); e == nil || counted.calls != 0 {
		t.Fatal("unconditional update dispatched")
	}
}

// DB-scoped compilation and immediate dispatch retain decoded RETURNING positions.
func TestUpdateReturningDBDiagnosticPosition(t *testing.T) {
	std, _, err := sqlmock.New()
	if err != nil {
		t.Fatal("mock unavailable")
	}
	defer std.Close()
	counted := &patchCounter{Executor: std}
	db := orm.NewDBWithExecutor(counted, driver.PostgresDialect{}, orm.WithSettings(query.Settings{}))
	opts := operation.Options{Manifest: typedfixture.Manifest("postgres")}
	var s operation.UpdateSpec
	if json.Unmarshal([]byte(`{"version":1,"model":"gq08_records","filters":[{"field":"id","op":"=","value":1},{"field":"segment","op":"=","value":0}],"assignments":[{"column":"active","state":"value","value":false}],"returning":["active","flag"]}`), &s) != nil {
		t.Fatal("decode failed")
	}
	p, view, err := db.CompileUpdateWithDiagnostics(t.Context(), s, opts)
	if err != nil || p == nil || view.Outcome != "compiled" || counted.calls != 0 {
		t.Fatal("plan refused or dispatched")
	}
	for _, d := range view.Diagnostics {
		if d.Location.Section == "select" {
			t.Fatal("synthetic SELECT location")
		}
	}
	s.Returning[1] = "hidden"
	p, view, err = db.CompileUpdateWithDiagnostics(t.Context(), s, opts)
	if p != nil || !errors.Is(err, operation.ErrForbiddenField) || counted.calls != 0 || view.Outcome != "rejected" {
		t.Fatal("compile refusal changed")
	}
	first := view.Diagnostics[0]
	loc := first.Location
	if first.Code != "OPERATION_FIELD_FORBIDDEN" || first.Status != "refused" || first.Ordinal != 1 || loc.Source != "spec" || loc.Section != "returning" || !loc.IndexKnown || loc.Index != 1 || loc.Origin != "json" {
		t.Fatal("DB refusal location changed")
	}
	if _, err := orm.UpdateOperationReturningBy[struct{ Active bool }](t.Context(), db, s, opts); !errors.Is(err, operation.ErrForbiddenField) || counted.calls != 0 {
		t.Fatal("refused returning dispatched")
	}
}
