package orm

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

type numericRow struct {
	ID         int64         `db:"id"`
	Amount     NumericString `db:"amount"`
	Confidence NumericString `db:"confidence"`
}

func TestNumericStringScanValueAndDefault(t *testing.T) {
	var n NumericString
	if err := n.Scan([]byte("123.4500")); err != nil {
		t.Fatalf("scan bytes: %v", err)
	}
	if !n.Valid || n.String != "123.4500" {
		t.Fatalf("unexpected numeric: %+v", n)
	}
	value, err := n.Value()
	if err != nil {
		t.Fatalf("value: %v", err)
	}
	if value != "123.4500" {
		t.Fatalf("unexpected driver value: %v", value)
	}
	if err := n.Scan(nil); err != nil {
		t.Fatalf("scan null: %v", err)
	}
	if got := n.OrDefault("0"); got != "0" {
		t.Fatalf("default=%s", got)
	}
	if _, err := NumericStringOf("").Value(); err == nil || !strings.Contains(err.Error(), "empty numeric") {
		t.Fatalf("expected empty numeric error, got %v", err)
	}
}

func TestNumericStringScansThroughSelectOne(t *testing.T) {
	ctx := context.Background()
	db, mock := newMockDB(t, BoolCompat)
	rows := sqlmock.NewRows([]string{"id", "amount", "confidence"}).
		AddRow(int64(1), []byte("1000.25"), "0.987")
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	row, err := SelectOne[numericRow](ctx, db, "SELECT id, amount, confidence FROM scores")
	if err != nil {
		t.Fatalf("select numeric row: %v", err)
	}
	if row.Amount.String != "1000.25" || row.Confidence.String != "0.987" {
		t.Fatalf("unexpected numeric row: %+v", row)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
