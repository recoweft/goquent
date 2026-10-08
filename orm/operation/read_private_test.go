package operation

import (
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/internal/querybridge"
	"github.com/recoweft/goquent/orm/query"
)

func TestReadPreparationRetainsPrivateSeal(t *testing.T) {
	std, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer std.Close()
	spec, opts := typedFixture("bigint", "postgres")
	p, err := querybridge.PrepareOperation(t.Context(), spec, opts, std)
	if err != nil {
		t.Fatal(err)
	}
	diagnostic := p.Diagnostic.(*query.QueryPlan)
	sqlText := diagnostic.SQL
	diagnostic.SQL = "tampered"
	diagnostic.Params = []any{"tampered"}
	diagnostic.Blocked = false
	mock.ExpectQuery(regexp.QuoteMeta(sqlText)).WithArgs(1).WillReturnRows(sqlmock.NewRows([]string{"v"}).AddRow(1))
	if err = p.Scan(func(rows *sql.Rows) error {
		if !rows.Next() {
			t.Fatal("missing row")
		}
		var n int
		return rows.Scan(&n)
	}); err != nil {
		t.Fatal(err)
	}
	if err = p.Scan(func(*sql.Rows) error { t.Fatal("consumed scanner called"); return nil }); !errors.Is(err, query.ErrBlockedOperation) {
		t.Fatal("private plan reused")
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
