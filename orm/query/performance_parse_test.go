package query

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
)

func TestPerformanceUnknownTableChildRegression(t *testing.T) {
	raw := []byte(`{"query_block":{"table":{"access_type":"ALL","rows_examined_per_scan":1,"cost_info":{"prefix_cost":"1"},"future_child":{"table":{"access_type":"future"}}}}}`)
	nodes, unknown, err := parsePerformance("mysql", raw)
	if err != nil || !unknown || len(nodes) != 1 {
		t.Fatalf("unknown child: unknown=%t nodes=%d error=%v", unknown, len(nodes), err)
	}
	assertPerformanceProjection(t, "mysql", raw, true)
}

// These inputs were captured from the native bounded terminal on MySQL 8.4.6
// and PostgreSQL 16.10, using the dedicated integer-only fixture. Mutations below
// are synthetic parser tests, not evidence that a server emitted an extension.
func performanceFixture(t *testing.T, dialect string) []byte {
	t.Helper()
	name := "mysql-8.4.json"
	if dialect == "postgres" {
		name = "postgres-16.json"
	}
	b, e := os.ReadFile("testdata/performance/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func performanceFixtureMutation(t *testing.T, dialect, path string, value any) []byte {
	t.Helper()
	root, e := performanceJSON(performanceFixture(t, dialect))
	if e != nil {
		t.Fatal(e)
	}
	parts := strings.Split(path, "/")
	parent := root
	for _, key := range parts[:len(parts)-1] {
		switch v := parent.(type) {
		case map[string]any:
			parent = v[key]
		case []any:
			i, e := strconv.Atoi(key)
			if e != nil {
				t.Fatal(e)
			}
			parent = v[i]
		default:
			t.Fatal("invalid fixture path")
		}
	}
	parent.(map[string]any)[parts[len(parts)-1]] = value
	b, e := json.Marshal(root)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func assertPerformanceProjection(t *testing.T, dialect string, raw []byte, wantUnknown bool) {
	t.Helper()
	nodes, unknown, e := parsePerformance(dialect, raw)
	if e != nil || unknown != wantUnknown || len(nodes) == 0 {
		t.Fatalf("parser: unknown=%t nodes=%d error=%v", unknown, len(nodes), e)
	}
	if nodes[0].Rows == nil || nodes[0].Cost == nil {
		t.Fatal("lost known partial estimates")
	}
	p := PerformancePlan{Kind: PerformancePlanKind, Version: 1, Dialect: dialect, Raw: raw, Nodes: nodes, Unknown: unknown}
	source, e := p.ToJSON()
	if e != nil {
		t.Fatal(e)
	}
	var decoded PerformancePlan
	if e := json.Unmarshal(source, &decoded); e != nil || decoded.Unknown != wantUnknown || decoded.Native() {
		t.Fatal("source roundtrip/provenance")
	}
	for _, plan := range []PerformancePlan{p, decoded} {
		v := plan.PublicView()
		if v.Unknown != wantUnknown || !v.Nodes[0].RowsKnown || !v.Nodes[0].CostKnown {
			t.Fatal("public partial evidence")
		}
		public, e := json.Marshal(v)
		if e != nil {
			t.Fatal(e)
		}
		roundtrip, e := DecodePerformancePlanView(public)
		if e != nil || roundtrip.Unknown != wantUnknown {
			t.Fatal("public unknown roundtrip")
		}
		for _, output := range []string{string(public), plan.String(), fmt.Sprintf("%#v", plan), fmt.Sprintf("%+v", &plan)} {
			if strings.Contains(output, "PRIVATE_CANARY") {
				t.Fatal("extension leaked through public projection")
			}
		}
	}
}

func TestPerformanceNativeJSONFixtures(t *testing.T) {
	for _, d := range []string{"mysql", "postgres"} {
		t.Run(d, func(t *testing.T) { assertPerformanceProjection(t, d, performanceFixture(t, d), false) })
	}
}

func TestPerformanceUnknownAtEveryLevel(t *testing.T) {
	for _, tc := range []struct{ dialect, path string }{
		{"mysql", "future"},
		{"mysql", "query_block/future"},
		{"mysql", "query_block/cost_info/future"},
		{"mysql", "query_block/ordering_operation/future"},
		{"mysql", "query_block/ordering_operation/table/future"},
		{"mysql", "query_block/ordering_operation/table/cost_info/future"},
		{"postgres", "0/future"},
		{"postgres", "0/Plan/future"},
		{"postgres", "0/Plan/Plans/0/future"},
		{"postgres", "0/Plan/Plans/0/Plans/0/future"},
	} {
		t.Run(tc.dialect+"/"+tc.path, func(t *testing.T) {
			for _, extension := range []any{"PRIVATE_CANARY", map[string]any{"PRIVATE_CANARY": []any{map[string]any{"table": map[string]any{"access_type": "PRIVATE_CANARY"}}}}} {
				raw := performanceFixtureMutation(t, tc.dialect, tc.path, extension)
				assertPerformanceProjection(t, tc.dialect, raw, true)
			}
		})
	}
	for _, tc := range []struct{ dialect, path string }{
		{"mysql", "query_block/ordering_operation/table/access_type"},
		{"postgres", "0/Plan/Plans/0/Plans/0/Node Type"},
		{"postgres", "0/Plan/Plans/0/Parent Relationship"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			assertPerformanceProjection(t, tc.dialect, performanceFixtureMutation(t, tc.dialect, tc.path, "PRIVATE_CANARY"), true)
		})
	}
}

func TestPerformanceKnownFieldTypes(t *testing.T) {
	for _, tc := range []struct {
		dialect, path string
		bad           any
	}{
		{"mysql", "query_block", []any{}},
		{"mysql", "query_block/select_id", "PRIVATE_CANARY"},
		{"mysql", "query_block/cost_info", []any{}},
		{"mysql", "query_block/ordering_operation", true},
		{"mysql", "query_block/ordering_operation/using_filesort", "false"},
		{"mysql", "query_block/ordering_operation/table", []any{}},
		{"mysql", "query_block/ordering_operation/table/access_type", map[string]any{}},
		{"mysql", "query_block/ordering_operation/table/key", []any{}},
		{"mysql", "query_block/ordering_operation/table/used_columns", []any{map[string]any{"PRIVATE_CANARY": true}}},
		{"mysql", "query_block/ordering_operation/table/used_key_parts", "id"},
		{"mysql", "query_block/ordering_operation/table/attached_condition", map[string]any{}},
		{"mysql", "query_block/ordering_operation/table/cost_info", []any{}},
		{"mysql", "query_block/ordering_operation/table/cost_info/prefix_cost", true},
		{"mysql", "query_block/ordering_operation/table/cost_info/data_read_per_join", map[string]any{}},
		{"mysql", "query_block/nested_loop", []any{true}},
		{"postgres", "0/Plan", []any{}},
		{"postgres", "0/Plan/Node Type", 42},
		{"postgres", "0/Plan/Parallel Aware", "false"},
		{"postgres", "0/Plan/Async Capable", map[string]any{}},
		{"postgres", "0/Plan/Plans", map[string]any{}},
		{"postgres", "0/Plan/Plans", []any{42}},
		{"postgres", "0/Plan/Plans/0/Sort Key", []any{map[string]any{"PRIVATE_CANARY": true}}},
		{"postgres", "0/Plan/Plans/0/Plans/0/Filter", []any{}},
		{"postgres", "0/Plan/Index Name", 42},
		{"postgres", "0/Plan/Output", []any{false}},
	} {
		t.Run(tc.dialect+"/"+tc.path, func(t *testing.T) {
			for _, bad := range []any{tc.bad, nil} {
				assertPerformanceFixedResult(t, tc.dialect, performanceFixtureMutation(t, tc.dialect, tc.path, bad))
			}
		})
	}
}

func assertPerformanceFixedResult(t *testing.T, dialect string, raw []byte) {
	t.Helper()
	nodes, unknown, e := parsePerformance(dialect, raw)
	if e != ErrPerformanceResult || !unknown || nodes != nil || fmt.Sprintf("%+v", e) != "PERFORMANCE_RESULT" {
		t.Fatal("malformed known field did not produce fixed failure")
	}
}

func TestPerformanceAuxiliaryNumbers(t *testing.T) {
	for _, tc := range []struct{ dialect, path string }{
		{"mysql", "query_block/cost_info/query_cost"},
		{"mysql", "query_block/ordering_operation/table/cost_info/read_cost"},
		{"mysql", "query_block/ordering_operation/table/cost_info/eval_cost"},
		{"mysql", "query_block/ordering_operation/table/cost_info/prefix_cost"},
		{"mysql", "query_block/ordering_operation/table/cost_info/data_read_per_join"},
		{"mysql", "query_block/ordering_operation/table/key_length"},
		{"mysql", "query_block/ordering_operation/table/filtered"},
		{"mysql", "query_block/ordering_operation/table/rows_produced_per_join"},
		{"mysql", "query_block/ordering_operation/table/rows_examined_per_scan"},
		{"mysql", "query_block/select_id"},
		{"postgres", "0/Plan/Startup Cost"},
		{"postgres", "0/Plan/Total Cost"},
		{"postgres", "0/Plan/Plan Rows"},
		{"postgres", "0/Plan/Plan Width"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			for _, bad := range []any{nil, "NaN", "Infinity", "-1", "1e999", "1000000000000001", json.Number("-1"), json.Number("1e999"), json.Number("1000000000000001")} {
				assertPerformanceFixedResult(t, tc.dialect, performanceFixtureMutation(t, tc.dialect, tc.path, bad))
			}
		})
	}
	for _, tc := range []struct {
		dialect, path string
		bad           any
	}{
		{"mysql", "query_block/ordering_operation/table/filtered", "100.01"},
		{"mysql", "query_block/ordering_operation/table/key_length", "1.5"},
		{"mysql", "query_block/ordering_operation/table/rows_produced_per_join", 1.5},
		{"postgres", "0/Plan/Plan Width", 1.5},
		{"mysql", "query_block/ordering_operation/table/cost_info/data_read_per_join", "999P"},
	} {
		assertPerformanceFixedResult(t, tc.dialect, performanceFixtureMutation(t, tc.dialect, tc.path, tc.bad))
	}
}

func TestPerformanceAuxiliaryObjects(t *testing.T) {
	for _, tc := range []struct {
		name, dialect, path string
		value               any
		unknown, invalid    bool
	}{
		{"sort_cost", "mysql", "query_block/ordering_operation/cost_info", map[string]any{"sort_cost": "1.5"}, false, false},
		{"sort_extension", "mysql", "query_block/ordering_operation/cost_info", map[string]any{"sort_cost": "1.5", "future": []any{"PRIVATE_CANARY"}}, true, false},
		{"sort_bad_number", "mysql", "query_block/ordering_operation/cost_info", map[string]any{"sort_cost": "NaN"}, false, true},
		{"nested_extension", "mysql", "query_block/nested_loop", []any{map[string]any{"future": "PRIVATE_CANARY"}}, true, false},
		{"nested_bad_table", "mysql", "query_block/nested_loop", []any{map[string]any{"table": false}}, false, true},
		{"pg_auxiliary", "postgres", "0/Plan/Plans/0/Full-sort Groups", map[string]any{"Sort Methods Used": []any{"PRIVATE_CANARY"}}, true, false},
		{"bytes_unit", "mysql", "query_block/ordering_operation/table/cost_info/data_read_per_join", "24K", false, false},
		{"index_merge_lengths", "mysql", "query_block/ordering_operation/table/key_length", "8,8", true, false},
		{"bad_key_length_list", "mysql", "query_block/ordering_operation/table/key_length", "8,NaN", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := performanceFixtureMutation(t, tc.dialect, tc.path, tc.value)
			if tc.invalid {
				assertPerformanceFixedResult(t, tc.dialect, raw)
			} else {
				assertPerformanceProjection(t, tc.dialect, raw, tc.unknown)
			}
		})
	}
}

func TestPerformanceCollectorUnknownAndMalformedJSON(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(strconv.FormatBool(invalid), func(t *testing.T) {
			db, m := performanceTestDB(t)
			q := New(db, "public.users", driver.PostgresDialect{}).Select("id").Limit(2)
			statement, args, snap, shape, e := q.builder.BuildPerformanceSnapshot()
			if e != nil {
				t.Fatal(e)
			}
			p := newQueryPlan(OperationSelect, statement, args)
			appendSelectSnapshotMetadata(p, snap)
			q.finalizePlan(p)
			raw := performanceFixtureMutation(t, "postgres", "0/future", map[string]any{"PRIVATE_CANARY": []any{true}})
			if invalid {
				raw = performanceFixtureMutation(t, "postgres", "0/Plan/Plan Width", "PRIVATE_CANARY")
			}
			m.ExpectBegin()
			m.ExpectQuery("SELECT pg_catalog.current_setting('server_version'), pg_catalog.current_database()").WillReturnRows(sqlmock.NewRows([]string{"version", "database"}).AddRow("16.10", "testdb"))
			performancePGMockCatalog(m, "12")
			m.ExpectQuery("EXPLAIN (ANALYZE FALSE, FORMAT JSON) " + statement).WillReturnRows(sqlmock.NewRows([]string{"plan"}).AddRow(string(raw))).RowsWillBeClosed()
			m.ExpectRollback()
			conn, e := db.Conn(t.Context())
			if e != nil {
				t.Fatal(e)
			}
			result, e := performanceTransaction(t.Context(), conn, "postgres", p, shape)
			conn.Close()
			if invalid {
				if e != ErrPerformanceResult || result != nil || fmt.Sprintf("%+v", e) != "PERFORMANCE_RESULT" {
					t.Fatal("collector malformed-result boundary")
				}
			} else if e != nil || result == nil || !result.Unknown || !result.PublicView().Unknown || result.Native() || strings.Contains(result.String(), "PRIVATE_CANARY") {
				t.Fatal("mock collection lost unknown or acquired native provenance")
			}
			if m.ExpectationsWereMet() != nil {
				t.Fatal("collector lifecycle")
			}
		})
	}
}
