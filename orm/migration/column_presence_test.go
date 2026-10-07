package migration

import (
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"reflect"
	"strings"
	"testing"
)

func TestColumnNullablePresence(t *testing.T) {
	var c ColumnSchema
	for _, tc := range []struct {
		s            string
		known, value bool
	}{{`{"name":"v","nullable":true}`, true, true}, {`{"name":"v","nullable":false}`, true, false}, {`{"name":"v"}`, false, false}} {
		if e := json.Unmarshal([]byte(tc.s), &c); e != nil {
			t.Fatal(e)
		}
		if c.NullableKnown != tc.known || c.Nullable != tc.value {
			t.Fatal("presence or reused receiver")
		}
		b, e := json.Marshal(c)
		if e != nil || strings.Contains(string(b), `"nullable"`) != tc.known {
			t.Fatal("marshal presence")
		}
	}
	for _, s := range []string{`{"nullable":null}`, `{"nullable":0}`, `{"nullable":true,"nullable":false}`, `{"Nullable":true}`, `{"nullable_known":true}`, `{"NullableKnown":true}`} {
		if json.Unmarshal([]byte(s), &c) == nil {
			t.Fatal(s)
		}
	}
	for _, known := range []bool{false, true} {
		for _, v := range []bool{false, true} {
			c = ColumnSchema{Name: "v", Type: "bigint", Nullable: v, NullableKnown: known}
			b, _ := json.Marshal(c)
			var out ColumnSchema
			if json.Unmarshal(b, &out) != nil || out.NullableKnown != known {
				t.Fatal("Go roundtrip")
			}
			if known && out.Nullable != v {
				t.Fatal("known value lost")
			}
		}
	}
}
func TestNullablePresenceDoesNotChangeInProcessDiff(t *testing.T) {
	a := Schema{Tables: []TableSchema{{Name: "t", Columns: []ColumnSchema{{Name: "v", Type: "bigint", Nullable: true}}}}}
	b := Schema{Tables: []TableSchema{{Name: "t", Columns: []ColumnSchema{{Name: "v", Type: "bigint", Nullable: true, NullableKnown: true}}}}}
	if !reflect.DeepEqual(DiffSchemas(a, a), DiffSchemas(a, b)) || !reflect.DeepEqual(CompareSchemaDrift(a, a), CompareSchemaDrift(a, b)) {
		t.Fatal("presence changed DDL judgement")
	}
}
func TestReadNullableTokensAndErrors(t *testing.T) {
	for _, token := range []string{"YES", "NO", "yes", "no", "", "unknown"} {
		db, m, e := sqlmock.New()
		if e != nil {
			t.Fatal(e)
		}
		m.ExpectQuery("columns").WillReturnRows(sqlmock.NewRows([]string{"table", "name", "type", "nullable", "default"}).AddRow("t", "v", "bigint", token, nil))
		rows, e := db.Query("columns")
		if e != nil {
			t.Fatal(e)
		}
		tables, e := scanSchemaColumns(rows, nil)
		if e != nil {
			t.Fatal(e)
		}
		c := tables["t"].Columns[0]
		if c.NullableKnown != (strings.EqualFold(token, "yes") || strings.EqualFold(token, "no")) {
			t.Fatal("unknown became known")
		}
		db.Close()
	}
	for _, scan := range []bool{false, true} {
		db, m, _ := sqlmock.New()
		r := sqlmock.NewRows([]string{"table", "name", "type", "nullable", "default"})
		if scan {
			r.AddRow(nil, "v", "bigint", "NO", nil)
		} else {
			r.AddRow("t", "v", "bigint", "NO", nil).RowError(0, errors.New("fixture rows error"))
		}
		m.ExpectQuery("columns").WillReturnRows(r)
		rows, _ := db.Query("columns")
		if _, e := scanSchemaColumns(rows, nil); e == nil {
			t.Fatal("DB error swallowed")
		}
		db.Close()
	}
}
