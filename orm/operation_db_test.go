package orm

import (
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/operation"
	"github.com/recoweft/goquent/orm/query"
	"strings"
	"testing"
)

func TestDBOperationCannotReplaceCurrentSettings(t *testing.T) {
	sqlDB, m, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	defer sqlDB.Close()
	schema, e := query.NewApplicationSchema(query.ApplicationSchemaInput{Database: "fixture", Dialect: "postgres", Tables: []query.ApplicationTable{{Table: "items", PlainTable: true, Columns: []query.WriteKeyColumn{{Name: "id", DBType: "bigint", Bits: 64}}}}})
	if e != nil {
		t.Fatal(e)
	}
	current := query.Settings{}.WithTenantPolicy("fixture", schema, false)
	db := NewDB(sqlDB, driver.PostgresDialect{}, WithSettings(current))
	spec := operation.OperationSpec{Operation: "select", Model: "items", Select: []string{"id"}, Filters: []operation.FilterSpec{{Field: "id", Value: 1}}}
	unknown := &manifest.Manifest{Version: manifest.Version, Dialect: "postgres", Tables: []manifest.Table{{Name: "items", Columns: []manifest.Column{{Name: "id", Type: "bigint"}}}}}
	replacement := query.Settings{}
	opts := operation.Options{Manifest: unknown, Settings: &replacement, Dialect: driver.MySQLDialect{}}
	if _, e := db.CompileOperation(t.Context(), spec, opts); !errors.Is(e, operation.ErrTypeUnverified) {
		t.Fatal("caller replaced Strict", e)
	}
	if _, e := db.ValidateOperation(spec, opts); !errors.Is(e, operation.ErrTypeUnverified) {
		t.Fatal(e)
	}
	unknown.Tables[0].Columns[0].TypeSource = "sql"
	unknown.Tables[0].Columns[0].NullableKnown = true
	p, e := db.CompileOperation(t.Context(), spec, opts)
	if e != nil {
		t.Fatal(e)
	}

	if !strings.Contains(p.SQL, "$1") || strings.Contains(p.SQL, "?") {
		t.Fatal("caller replaced DB dialect")
	}

	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal("planning used DB", e)
	}
}
