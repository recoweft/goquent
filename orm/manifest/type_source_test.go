package manifest

import (
	"encoding/json"
	"github.com/recoweft/goquent/orm/migration"
	"reflect"
	"strings"
	"testing"
	"time"
)

type sourceModel struct {
	ID    int8    `db:"id,pk"`
	Name  *string `db:"name,pii"`
	Extra int     `db:"extra,tenant"`
}

func (sourceModel) TableName() string { return "records" }
func TestTypeSourceAndNullableMerge(t *testing.T) {
	schema := migration.Schema{Tables: []migration.TableSchema{{Name: "records", Columns: []migration.ColumnSchema{{Name: "id", Type: "int8", Nullable: true, NullableKnown: true}, {Name: "name", Type: "varchar(12)", Nullable: false, NullableKnown: true}}}}}
	opts := Options{Dialect: "postgres", Schema: &schema, Models: []any{sourceModel{}}, GeneratedAt: time.Unix(1, 0)}
	m, e := Generate(opts)
	if e != nil {
		t.Fatal(e)
	}
	cols := map[string]Column{}
	for _, c := range m.Tables[0].Columns {
		cols[c.Name] = c
	}
	if cols["id"].Type != "int8" || cols["id"].TypeSource != "sql" || !cols["id"].Nullable || !cols["id"].NullableKnown || cols["id"].Primary {
		t.Fatal("model replaced SQL constraints")
	}
	if cols["name"].Nullable || !cols["name"].NullableKnown || !cols["name"].PII {
		t.Fatal("pointer replaced SQL or policy lost")
	}
	if cols["extra"].TypeSource != "go" || !cols["extra"].TenantScope || cols["extra"].NullableKnown {
		t.Fatal("Go metadata promoted")
	}
	b, e := json.Marshal(m)
	if e != nil {
		t.Fatal(e)
	}
	var out Manifest
	if json.Unmarshal(b, &out) != nil || !reflect.DeepEqual(m.Tables, out.Tables) {
		t.Fatal("source roundtrip")
	}
	again, _ := Generate(opts)
	if again.SchemaFingerprint != m.SchemaFingerprint {
		t.Fatal("nondeterministic")
	}
	changed := append([]Table(nil), m.Tables...)
	changed[0].Columns = append([]Column(nil), m.Tables[0].Columns...)
	changed[0].Columns[0].TypeSource = "sql"
	if fingerprintSchema(changed) == m.SchemaFingerprint {
		t.Fatal("source omitted from hash")
	}
	schema.Tables[0].Columns[0].NullableKnown = false
	unknown, _ := Generate(opts)
	for _, c := range unknown.Tables[0].Columns {
		if c.Name == "id" && c.NullableKnown {
			t.Fatal("model filled unknown nullable")
		}
	}
}
func TestColumnSourcePresenceAndFingerprintMigration(t *testing.T) {
	for _, s := range []string{`{"name":"v","type_source":""}`, `{"name":"v","type_source":null}`, `{"type_source":"SQL"}`, `{"type_source":"sql","Type_Source":"go"}`, `{"nullable":null}`, `{"NullableKnown":true}`} {
		var c Column
		if json.Unmarshal([]byte(s), &c) == nil {
			t.Fatal(s)
		}
	}
	for _, source := range []string{"", "sql", "go"} {
		c := Column{Name: "v", Type: "int8", TypeSource: source}
		b, _ := json.Marshal(c)
		var out Column
		if json.Unmarshal(b, &out) != nil || out.TypeSource != source || out.NullableKnown {
			t.Fatal("legacy promotion")
		}
		if strings.Contains(string(b), `"nullable"`) {
			t.Fatal("unknown became false")
		}
	}
	schema := migration.Schema{Tables: []migration.TableSchema{{Name: "t", Columns: []migration.ColumnSchema{{Name: "v", Type: "bigint"}}}}}
	old := fingerprintMigrationSchema(schema)
	schema.Tables[0].Columns[0].NullableKnown = true
	known := fingerprintMigrationSchema(schema)
	if old == known || known != fingerprintMigrationSchema(schema) {
		t.Fatal("database fingerprint presence")
	}
	c := Column{Name: "v", Type: "bigint", NullableKnown: true}
	b, _ := json.Marshal(c)
	if string(b) != `{"name":"v","type":"bigint","nullable":false}` {
		t.Fatal("known layout changed")
	}
}
