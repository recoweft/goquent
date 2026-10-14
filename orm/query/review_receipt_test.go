package query

import (
	"context"
	sqldriver "database/sql/driver"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/recoweft/goquent/orm/analysis"
	"github.com/recoweft/goquent/orm/driver"
)

type reviewOpaque struct{}

func (reviewOpaque) Value() (sqldriver.Value, error) { panic("Valuer called") }
func (reviewOpaque) String() string                  { panic("String called") }
func (reviewOpaque) MarshalJSON() ([]byte, error)    { panic("MarshalJSON called") }
func TestReviewReceiptPrivateLifecycle(t *testing.T) {
	ctx := context.Background()
	token := NewReviewToken(ctx)
	defer token.Close()
	q := NewWithSettings(nil, "users", driver.MySQLDialect{}, Settings{}).Select("id").Where("id", 1)
	a, e := q.ReviewReceipt(ctx, token)
	if e != nil {
		t.Fatal(e)
	}
	b, e := q.ReviewReceipt(ctx, token)
	if e != nil {
		t.Fatal(e)
	}
	if c := CompareReviewReceipts(token, a, b); c.Arguments != analysis.Matched {
		t.Fatalf("%+v", c)
	}
	if _, e = json.Marshal(a); e == nil {
		t.Fatal("serialized receipt")
	}
	if got := fmt.Sprintf("%#v", a); got != "<review receipt>" {
		t.Fatal("receipt formatting leaked")
	}
	if e = json.Unmarshal([]byte(`{}`), &b); e == nil {
		t.Fatal("decoded receipt")
	}
	if c := CompareReviewReceipts(token, a, b); c.Source != analysis.Mismatched {
		t.Fatal("zero receipt accepted")
	}
	q.Where("extra", 2)
	if c := CompareReviewReceipts(token, a, a); c.Source != analysis.Mismatched {
		t.Fatal("owner mutation missed")
	}
	token.Close()
	if _, e = q.ReviewReceipt(ctx, token); e == nil {
		t.Fatal("expired session accepted")
	}
	other := NewReviewToken(ctx)
	defer other.Close()
	opaque := NewWithSettings(nil, "users", driver.MySQLDialect{}, Settings{}).Where("id", reviewOpaque{})
	if _, e = opaque.ReviewReceipt(ctx, other); e == nil {
		t.Fatal("opaque value accepted")
	}
}

func TestReviewReceiptContextAndNoExecution(t *testing.T) {
	q, _, spy, mock := bindingFixture(t, false)
	token := NewReviewToken(context.Background())
	defer token.Close()
	a, e := q.ReviewReceipt(context.Background(), token)
	if e != nil {
		t.Fatal(e)
	}
	c := CompareReviewReceipts(token, a, a)
	if c.Context != analysis.Matched {
		t.Fatalf("declared context: %+v", c)
	}
	if CompareReviewOperation(token, a, OperationUpdate) != analysis.Mismatched {
		t.Fatal("different operation matched")
	}
	if spy.calls != [6]int{} {
		t.Fatal("receipt executed SQL")
	}
	if e = mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	token.state.bytes.Store(64 << 20)
	if _, e = q.ReviewReceipt(context.Background(), token); e != ErrReviewBudget {
		t.Fatal("budget not enforced")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = q.ReviewReceipt(ctx, token); e == nil {
		t.Fatal("cancel not enforced")
	}
	if spy.calls != [6]int{} {
		t.Fatal("refusal executed SQL")
	}
}
