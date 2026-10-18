package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
)

func performanceTestDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, m, e := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if e != nil {
		t.Fatal("mock creation")
	}
	t.Cleanup(func() { db.Close() })
	return db, m
}
func TestPerformanceStructuralRejection(t *testing.T) {
	cases := map[string]func(*Query) *Query{
		"join":            func(q *Query) *Query { return q.Select("id").Join("other", "id", "=", "id") },
		"distinct":        func(q *Query) *Query { return q.Distinct("id") },
		"having":          func(q *Query) *Query { return q.Select("id").GroupBy("id").HavingRaw("secret()") },
		"order_raw":       func(q *Query) *Query { return q.Select("id").OrderByRaw("secret()") },
		"shared_lock":     func(q *Query) *Query { return q.Select("id").SharedLock() },
		"empty_in":        func(q *Query) *Query { return q.Select("id").WhereIn("id", []int{}) },
		"not_in":          func(q *Query) *Query { return q.Select("id").WhereNotIn("id", []int{1}) },
		"column":          func(q *Query) *Query { return q.Select("id").WhereColumn("id", "=", "other") },
		"offset":          func(q *Query) *Query { return q.Select("id").Offset(10001) },
		"too_many_args":   func(q *Query) *Query { return q.Select("id").WhereIn("id", make([]int, 1025)) },
		"too_large_value": func(q *Query) *Query { return q.Select("id").Where("id", strings.Repeat("x", 1<<20+1)) },
		"star":            func(q *Query) *Query { return q.Select("*") },
		"raw":             func(q *Query) *Query { return q.SelectRaw("secret()") },
		"where_raw":       func(q *Query) *Query { return q.Select("id").WhereRaw("secret()", nil) },
		"lock":            func(q *Query) *Query { return q.Select("id").LockForUpdate() },
		"function":        func(q *Query) *Query { return q.Select("secret()") },
		"group":           func(q *Query) *Query { return q.Select("id").GroupBy("id") },
		"no_limit":        func(q *Query) *Query { return q.Select("id").Limit(0) },
		"limit":           func(q *Query) *Query { return q.Select("id").Limit(10001) },
		"opaque_value":    func(q *Query) *Query { return q.Select("id").Where("id", struct{}{}) },
		"nan":             func(q *Query) *Query { return q.Select("id").Where("id", math.NaN()) },
	}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			db, m := performanceTestDB(t)
			q := f(New(db, "users", driver.MySQLDialect{}).Limit(10))
			if q.err == nil {
				probe := newSelectBuilder(q.dialect)
				q.builder.CopyStateToSelect(probe)
				if _, _, _, _, err := probe.BuildPerformanceSnapshot(); err == nil {
					t.Fatal("structural refusal relied on executor rejection")
				}
			}
			p, e := q.ExplainPerformance(t.Context(), PerformanceOptions{})
			if p != nil || e != ErrPerformanceUnsupported && e != ErrPerformanceBlocked {
				t.Fatalf("expected fixed refusal: %v", e)
			}
			if m.ExpectationsWereMet() != nil {
				t.Fatal("unexpected query")
			}
		})
	}
}
func TestPerformanceExternalExecutorAndPlanRemainOffline(t *testing.T) {
	db, m := performanceTestDB(t)
	q := New(db, "users", driver.MySQLDialect{}).Select("id").Where("id", 1).Limit(2)
	if _, e := q.Plan(t.Context()); e != nil {
		t.Fatal("plan")
	}
	if _, e := q.ExplainPerformance(t.Context(), PerformanceOptions{}); e != ErrPerformanceUnsupported {
		t.Fatal("custom driver not refused")
	}
	if m.ExpectationsWereMet() != nil {
		t.Fatal("unexpected query")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, e := q.ExplainPerformance(ctx, PerformanceOptions{}); e != ErrPerformanceCanceled {
		t.Fatal("cancel")
	}
}
func TestPerformanceFrozenShape(t *testing.T) {
	db, _ := performanceTestDB(t)
	q := New(db, "public.users", driver.PostgresDialect{}).Select("id").Where("id", int64(4)).OrderBy("id", "asc").Limit(2)
	b := newSelectBuilder(q.dialect)
	q.builder.CopyStateToSelect(b)
	sql, args, snap, shape, e := b.BuildPerformanceSnapshot()
	if e != nil || !strings.Contains(sql, "LIMIT") || len(args) != 1 || len(shape.Columns) != 3 || snap.Limit != 2 {
		t.Fatalf("shape: %v", e)
	}
	q.Where("id", 9).Limit(7)
	if args[0] != int64(4) || shape.Values["id"][0] != int64(4) || snap.Limit != 2 {
		t.Fatal("snapshot mutated")
	}
}
func TestPerformanceRowsFailures(t *testing.T) {
	for _, tc := range []string{"scan", "partial", "row_error", "close", "too_many", "too_large", "query", "cancel"} {
		t.Run(tc, func(t *testing.T) {
			db, m := performanceTestDB(t)
			ctx := t.Context()
			rows := sqlmock.NewRows([]string{"value"}).AddRow("ok")
			switch tc {
			case "scan":
				rows = sqlmock.NewRows([]string{"value"}).AddRow(nil)
			case "partial":
				rows.AddRow("bad").RowError(1, errors.New("PRIVATE_CANARY"))
			case "row_error":
				rows.RowError(0, errors.New("PRIVATE_CANARY"))
			case "close":
				rows.CloseError(errors.New("PRIVATE_CANARY"))
			case "too_many":
				rows.AddRow("extra")
			case "too_large":
				rows = sqlmock.NewRows([]string{"value"}).AddRow(strings.Repeat("x", 100))
			}
			if tc == "query" {
				m.ExpectQuery("fixed").WillReturnError(errors.New("PRIVATE_CANARY"))
			} else if tc == "cancel" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, time.Millisecond)
				defer cancel()
				m.ExpectQuery("fixed").WillDelayFor(20 * time.Millisecond).WillReturnRows(rows)
			} else {
				m.ExpectQuery("fixed").WillReturnRows(rows).RowsWillBeClosed()
			}
			data, e := performanceRows(ctx, db, "fixed", nil, 1, 8)
			if e == nil || data != nil || strings.Contains(fmt.Sprintf("%+v", e), "PRIVATE_CANARY") {
				t.Fatal("failure boundary")
			}
			if m.ExpectationsWereMet() != nil {
				t.Fatal("expectations")
			}
		})
	}
}
func TestPerformanceJSONAndPublicBoundary(t *testing.T) {
	valid := []byte(`[{"Plan":{"Node Type":"Index Scan","Index Name":"PRIVATE_CANARY","Plan Rows":2,"Total Cost":3,"Index Cond":"PRIVATE_CANARY"}}]`)
	ns, u, e := parsePerformance("postgres", valid)
	if e != nil || u || len(ns) != 1 {
		t.Fatal("valid parser")
	}
	p := PerformancePlan{Kind: PerformancePlanKind, Version: 1, Dialect: "postgres", ServerVersion: "PRIVATE_CANARY", Raw: valid, Nodes: ns, native: true}
	for _, s := range []string{fmt.Sprintf("%#v", p), fmt.Sprintf("%+v", &p), p.PublicView().String()} {
		if strings.Contains(s, "PRIVATE_CANARY") {
			t.Fatal("public leak")
		}
	}
	b, e := p.ToJSON()
	if e != nil || !strings.Contains(string(b), "PRIVATE_CANARY") {
		t.Fatal("source")
	}
	var decoded PerformancePlan
	if json.Unmarshal(b, &decoded) != nil || decoded.Native() {
		t.Fatal("source restored provenance")
	}
	view := p.PublicView()
	view.Dialect = "PRIVATE_CANARY"
	view.Nodes[0].Scan = "PRIVATE_CANARY"
	view.Nodes[0].Rows = math.Inf(1)
	if strings.Contains(view.String(), "PRIVATE_CANARY") {
		t.Fatal("tampered view leak")
	}
	if _, e := DecodePerformancePlanView([]byte(view.String())); e != nil {
		t.Fatal("view roundtrip")
	}
	for _, raw := range []string{`{"a":1,"a":2}`, `{} {}`, `{"a":1e999}`, `[{"Plan":{"Node Type":"Limit","Plan Rows":-1}}]`, `[{"Plan":{"Plan Rows":"NaN"}}]`, `[{"Plan":{"Total Cost":"1e999"}}]`} {
		if _, _, e := parsePerformance("postgres", []byte(raw)); e == nil {
			t.Fatal("invalid accepted")
		}
	}
	if _, u, e := parsePerformance("postgres", []byte(`[{"Plan":{"Node Type":"PRIVATE_CANARY"}}]`)); e != nil || !u {
		t.Fatal("unknown node")
	}
}

func performancePGMockCatalog(m sqlmock.Sqlmock, oid string) {
	m.ExpectExec("SET LOCAL search_path TO pg_catalog").WillReturnResult(sqlmock.NewResult(0, 0))
	m.ExpectQuery(performancePGTable).WithArgs("public", "users").WillReturnRows(sqlmock.NewRows([]string{"oid", "kind", "ok"}).AddRow(oid, "r", "true"))
	m.ExpectQuery(performancePGColumns).WithArgs(oid).WillReturnRows(sqlmock.NewRows([]string{"column", "type", "ok"}).AddRow("id", "20", "true"))
	rows := sqlmock.NewRows([]string{"operator", "left", "right", "ok"})
	for _, op := range []string{"=", "<>", "<", "<=", ">", ">="} {
		for _, l := range []string{"20", "21", "23"} {
			for _, r := range []string{"20", "21", "23"} {
				rows.AddRow(op, l, r, "true")
			}
		}
	}
	m.ExpectQuery(performancePGOperators).WillReturnRows(rows)
	m.ExpectQuery(performancePGIndexes).WithArgs(oid).WillReturnRows(sqlmock.NewRows([]string{"index", "ok"}).AddRow("12345", "true"))
}
func TestPerformanceCollectorLifecycle(t *testing.T) {
	for _, failure := range []string{"none", "version", "catalog", "operator", "rollback", "begin", "deadline"} {
		t.Run(failure, func(t *testing.T) {
			db, m := performanceTestDB(t)
			q := New(db, "public.users", driver.PostgresDialect{}).Select("id").Where("id", int64(7)).Limit(2)
			statement, args, snap, shape, e := q.builder.BuildPerformanceSnapshot()
			if e != nil {
				t.Fatal("shape")
			}
			p := newQueryPlan(OperationSelect, statement, args)
			appendSelectSnapshotMetadata(p, snap)
			q.finalizePlan(p)
			q.Where("id", int64(99)).Limit(9)
			if failure == "begin" {
				m.ExpectBegin().WillReturnError(errors.New("PRIVATE_CANARY"))
			} else {
				m.ExpectBegin()
				v := "16.10"
				if failure == "version" {
					v = "17.1"
				}
				expect := m.ExpectQuery("SELECT pg_catalog.current_setting('server_version'), pg_catalog.current_database()")
				if failure == "deadline" {
					expect.WillDelayFor(50 * time.Millisecond)
				}
				expect.WillReturnRows(sqlmock.NewRows([]string{"version", "database"}).AddRow(v, "PRIVATE_CANARY"))
				switch failure {
				case "catalog":
					m.ExpectExec("SET LOCAL search_path TO pg_catalog").WillReturnResult(sqlmock.NewResult(0, 0))
					m.ExpectQuery(performancePGTable).WithArgs("public", "users").WillReturnRows(sqlmock.NewRows([]string{"oid", "kind", "ok"}).AddRow("12", "v", "false"))
				case "operator":
					m.ExpectExec("SET LOCAL search_path TO pg_catalog").WillReturnResult(sqlmock.NewResult(0, 0))
					m.ExpectQuery(performancePGTable).WithArgs("public", "users").WillReturnRows(sqlmock.NewRows([]string{"oid", "kind", "ok"}).AddRow("12", "r", "true"))
					m.ExpectQuery(performancePGColumns).WithArgs("12").WillReturnRows(sqlmock.NewRows([]string{"column", "type", "ok"}).AddRow("id", "20", "false"))
				case "version", "deadline":
				default:
					performancePGMockCatalog(m, "12")
					m.ExpectQuery("EXPLAIN (ANALYZE FALSE, FORMAT JSON) " + statement).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"plan"}).AddRow(`[{"Plan":{"Node Type":"Seq Scan","Plan Rows":1,"Total Cost":2}}]`))
				}
				if failure == "rollback" {
					m.ExpectRollback().WillReturnError(errors.New("PRIVATE_CANARY"))
				} else {
					m.ExpectRollback()
				}
			}
			ctx := t.Context()
			if failure == "deadline" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 10*time.Millisecond)
				defer cancel()
			}
			conn, e := db.Conn(ctx)
			if e != nil {
				t.Fatal("connection")
			}
			result, e := performanceTransaction(ctx, conn, "postgres", p, shape)
			conn.Close()
			if failure == "none" {
				if e != nil || result == nil || result.Native() {
					t.Fatalf("collector: %v", e)
				}
			} else if e == nil {
				t.Fatal("failure swallowed")
			}
			if strings.Contains(fmt.Sprintf("%+v", e), "PRIVATE_CANARY") {
				t.Fatal("cause leak")
			}
			// database/sql may perform cancellation rollback asynchronously.
			deadline := time.Now().Add(time.Second)
			for m.ExpectationsWereMet() != nil && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if m.ExpectationsWereMet() != nil {
				t.Fatal("unmet lifecycle expectations")
			}
		})
	}
}

func TestPerformanceBudgetsAndUnknown(t *testing.T) {
	for _, tc := range []struct{ dialect, raw string }{
		{"mysql", `{"query_block":{"table":{"access_type":"ALL","rows_examined_per_scan":2,"cost_info":{"prefix_cost":"0.3"}}}}`},
		{"mysql", `{"query_block":{"table":{"access_type":"future"}}}`},
		{"postgres", `[{"Plan":{"Node Type":"Future","Plans":[{"Node Type":"Seq Scan","Plan Rows":0,"Total Cost":0}]}}]`},
	} {
		ns, _, e := parsePerformance(tc.dialect, []byte(tc.raw))
		if e != nil || len(ns) == 0 {
			t.Fatal("parser fixture")
		}
	}
	for _, raw := range []string{strings.Repeat("[", 34) + "0" + strings.Repeat("]", 34), "[" + strings.Repeat("0,", 4096) + "0]", strings.Repeat(" ", 1<<20) + "{}"} {
		if _, e := performanceJSON([]byte(raw)); e != ErrPerformanceLimit {
			t.Fatal("budget")
		}
	}
	for _, raw := range []string{`{"kind":"goquent.performance_plan_view","version":1,"version":1}`, `{"kind":"goquent.performance_plan_view","version":2}`, `{"kind":"goquent.performance_plan_view","version":1,"secret":"PRIVATE_CANARY"}`} {
		if _, e := DecodePerformancePlanView([]byte(raw)); e == nil {
			t.Fatal("view strict decoder")
		}
	}
	n := PerformanceNodeView{Scan: "PRIVATE_CANARY", RowsKnown: true, Rows: math.NaN()}
	if strings.Contains(fmt.Sprintf("%#v", n), "PRIVATE_CANARY") {
		t.Fatal("nested format")
	}
}

func TestPerformanceTenantGateBeforeDispatch(t *testing.T) {
	for _, auto := range []bool{false, true} {
		db, m := performanceTestDB(t)
		q := NewWithSettings(db, "users", driver.MySQLDialect{}, tenantTestSettings(t, "mysql", auto)).Select("id").Limit(2)
		if !auto {
			q.Where("tenant_id", 2)
		}
		_, e := q.ExplainPerformance(t.Context(), PerformanceOptions{})
		expected := ErrPerformanceBlocked
		if auto {
			expected = ErrPerformanceUnsupported
		}
		if e != expected {
			t.Fatalf("tenant gate: %v", e)
		}
		if m.ExpectationsWereMet() != nil {
			t.Fatal("dispatch")
		}
	}
}
func TestPerformanceAllowedPredicates(t *testing.T) {
	db, _ := performanceTestDB(t)
	q := New(db, "public.users", driver.PostgresDialect{}).Select("id").WhereGroup(func(g *Query) { g.Where("id", 1).OrWhereIn("id", []int{2, 3}) }).WhereBetween("id", 1, 5).WhereNotNull("id").Limit(10)
	_, args, _, _, e := q.builder.BuildPerformanceSnapshot()
	if e != nil || len(args) != 5 {
		t.Fatalf("finite predicates: %v", e)
	}
}

func TestPerformanceCatalogVisibilityRefusal(t *testing.T) {
	db, m := performanceTestDB(t)
	m.ExpectQuery(performanceMySQLVisibility).WithArgs("testdb", "testdb", "users").WillReturnRows(sqlmock.NewRows([]string{"visible"}).AddRow(0))
	q := New(db, "users", driver.MySQLDialect{}).Select("id").Limit(1)
	_, _, _, shape, e := q.builder.BuildPerformanceSnapshot()
	if e != nil {
		t.Fatal("shape")
	}
	if _, e := performanceMySQLCatalog(t.Context(), db, shape, "testdb"); e != ErrPerformanceUnsupported {
		t.Fatal("incomplete catalog accepted")
	}
	if m.ExpectationsWereMet() != nil {
		t.Fatal("unexpected target query")
	}
}

func TestPerformanceLiteralIdentifierRefusal(t *testing.T) {
	db, _ := performanceTestDB(t)
	q := New(db, "users", driver.MySQLDialect{}).Select("id").Limit(1)
	q.builder.WhereLiteral("users.id", 1)
	if _, _, _, _, e := q.builder.BuildPerformanceSnapshot(); e == nil {
		t.Fatal("literal identifier reinterpreted as a path")
	}
}
