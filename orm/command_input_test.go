package orm

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	ormdriver "github.com/recoweft/goquent/orm/driver"
)

const commandTestID = "d52e691e-3919-43d6-a98b-2468499536ff"

func commandTestInput() Command {
	return Command{ID: commandTestID, Kind: "set-score", Steps: []CommandStep{{Operation: "insert", Table: "items", Values: map[string]any{"id": int64(1), "score": int64(2)}}}}
}

type commandTrap struct{}

func (commandTrap) Value() (driver.Value, error) { panic("must not evaluate value") }
func (commandTrap) MarshalJSON() ([]byte, error) { panic("must not marshal") }
func (commandTrap) String() string               { panic("must not stringify") }

func TestCommandCanonicalIdentity(t *testing.T) {
	c := commandTestInput()
	a, requests, e := snapshotCommand(c, "public")
	if e != nil {
		t.Fatal(e)
	}
	c.Steps[0].Values = map[string]any{"score": int64(2), "id": int64(1)}
	b, _, e := snapshotCommand(c, "public")
	if e != nil || !bytes.Equal(a, b) {
		t.Fatal("map order changed identity")
	}
	c.Steps[0].Values["score"] = int64(9)
	if requests[0].Rows[0]["score"] != int64(2) {
		t.Fatal("snapshot mutated")
	}
	cases := []any{int(2), int8(2), int16(2), int32(2), uint(2), uint8(2), uint16(2), uint32(2), uint64(2), json.Number("2"), nil, "", false}
	seen := map[string]bool{string(a): true}
	for _, v := range cases {
		c.Steps[0].Values["score"] = v
		b, _, e = snapshotCommand(c, "public")
		if e != nil || seen[string(b)] {
			t.Fatal("type/value identity collapsed")
		}
		seen[string(b)] = true
	}
	c = commandTestInput()
	delete(c.Steps[0].Values, "score")
	b, _, _ = snapshotCommand(c, "public")
	if seen[string(b)] {
		t.Fatal("missing collapsed")
	}
	c = commandTestInput()
	c.Steps = append(c.Steps, CommandStep{Operation: "update", Table: "items", Values: map[string]any{"score": int64(3)}, Keys: map[string]any{"id": int64(1)}})
	a, _, _ = snapshotCommand(c, "public")
	c.Steps[0], c.Steps[1] = c.Steps[1], c.Steps[0]
	b, _, _ = snapshotCommand(c, "public")
	if bytes.Equal(a, b) {
		t.Fatal("step order collapsed")
	}
}

func TestCommandInputRefusals(t *testing.T) {
	type named int
	cycle := map[string]any{}
	cycle["cycle"] = cycle
	for _, v := range []any{commandTrap{}, named(2), float64(1), float32(1), math.NaN(), json.Number("1.0"), json.Number("1e2"), json.Number("01"), json.Number("9223372036854775808"), uint64(math.MaxUint64), &cycle, cycle, []any{1}, []byte{1}, strings.Repeat("a", CommandMaxBytes+1), "\xff"} {
		c := commandTestInput()
		c.Steps[0].Values["score"] = v
		if _, _, e := snapshotCommand(c, "public"); !errors.Is(e, ErrCommandInput) {
			t.Fatal("unsupported value accepted")
		}
	}
	for _, mutate := range []func(*Command){
		func(c *Command) { c.ID = "not-uuid" }, func(c *Command) { c.Kind = "" }, func(c *Command) { c.Steps = nil }, func(c *Command) { c.Steps = make([]CommandStep, 65) },
		func(c *Command) { c.Steps[0].Table = commandTable }, func(c *Command) { c.Steps[0].Table = "public.items" }, func(c *Command) { c.Steps[0].Operation = "delete" },
		func(c *Command) { c.Steps[0].Keys = map[string]any{"id": 1} }, func(c *Command) { c.Steps[0].Operation = "update" }, func(c *Command) { c.Steps[0].Values["raw.x"] = 1 },
		func(c *Command) { c.Steps[0].Operation = "update"; c.Steps[0].Keys = map[string]any{"id": nil} },
	} {
		c := commandTestInput()
		mutate(&c)
		if _, _, e := snapshotCommand(c, "public"); !errors.Is(e, ErrCommandInput) {
			t.Fatal("invalid shape accepted")
		}
	}
}

func TestCommandOwnershipAndProvenance(t *testing.T) {
	std, mock, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	defer std.Close()
	db := NewDB(std, ormdriver.PostgresDialect{})
	for _, d := range []*DB{nil, {}, NewDBWithExecutor(std, ormdriver.PostgresDialect{}), db.WrapExecutor(std)} {
		if _, e = NewCommandStore(t.Context(), d, "app"); !errors.Is(e, ErrCommandOwnership) {
			t.Fatal("external executor accepted")
		}
	}
	if _, e = NewCommandStore(t.Context(), db, "app"); !errors.Is(e, ErrUnsupportedExecutionContext) {
		t.Fatal("missing trusted tenant accepted")
	}
	plain, _ := NewExecutionContext(ExecutionContextInput{TenantPresent: true, CurrentTenant: int64(1)})
	if _, e = NewCommandStore(t.Context(), db.WithOptions(WithExecutionContext(plain)), "app"); !errors.Is(e, ErrUnsupportedExecutionContext) {
		t.Fatal("public context accepted")
	}
	mock.ExpectBegin()
	tx, e := std.Begin()
	if e != nil {
		t.Fatal(e)
	}
	for _, d := range []*DB{NewTxDB(tx, ormdriver.PostgresDialect{}), db.WrapTx(tx)} {
		if _, e = NewCommandStore(t.Context(), d, "app"); !errors.Is(e, ErrCommandOwnership) {
			t.Fatal("external transaction accepted")
		}
	}
	mock.ExpectRollback()
	tx.Rollback()
	var zero CommandStore
	if _, e = zero.Run(context.Background(), commandTestInput()); !errors.Is(e, ErrCommandOwnership) {
		t.Fatal("zero store accepted")
	}
	if e = mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestCommandCanonicalV1AndBounds(t *testing.T) {
	c := commandTestInput()
	b, _, e := snapshotCommand(c, "public")
	if e != nil {
		t.Fatal(e)
	}
	want := `{"Version":1,"Kind":"set-score","Steps":[{"Operation":"insert","Table":"items","Values":[{"name":"id","atom":{"type":"int64","value":"1"}},{"name":"score","atom":{"type":"int64","value":"2"}}],"Keys":[]}]}`
	if string(b) != want {
		t.Fatal("v1 canonical format changed")
	}
	c.Steps[0].Values["score"] = strings.Repeat("x", CommandMaxBytes-512)
	if _, _, e = snapshotCommand(c, "public"); e != nil {
		t.Fatal("bounded near-limit scalar rejected")
	}
	c.Steps[0].Values["score"] = strings.Repeat("x", CommandMaxBytes)
	if _, _, e = snapshotCommand(c, "public"); !errors.Is(e, ErrCommandInput) {
		t.Fatal("encoded budget ignored")
	}
	c = commandTestInput()
	step := c.Steps[0]
	c.Steps = make([]CommandStep, CommandMaxSteps)
	for i := range c.Steps {
		c.Steps[i] = step
	}
	if _, _, e = snapshotCommand(c, "public"); e != nil {
		t.Fatal("maximum step boundary rejected")
	}
	for _, v := range []any{int64(math.MinInt64), int64(math.MaxInt64), uint64(math.MaxInt64), json.Number("-9223372036854775808"), json.Number("9223372036854775807"), json.Number("-0")} {
		if _, _, e = commandScalar(v); e != nil {
			t.Fatal("valid integer boundary rejected")
		}
	}
	c = commandTestInput()
	c.Steps[0].Values = map[string]any{}
	for i := 0; i < CommandMaxColumns; i++ {
		c.Steps[0].Values["c"+strconv.Itoa(i)] = int64(i)
	}
	if _, _, e = snapshotCommand(c, "public"); e != nil {
		t.Fatal("maximum column boundary rejected")
	}
	c.Steps[0].Values["overflow"] = 1
	if _, _, e = snapshotCommand(c, "public"); !errors.Is(e, ErrCommandInput) {
		t.Fatal("column budget ignored")
	}
	c.Steps[0].Values = map[string]any{"v": 1}
	c.Steps[0].Operation = "update"
	c.Steps[0].Keys = map[string]any{}
	for i := 0; i < CommandMaxColumns; i++ {
		c.Steps[0].Keys["k"+strconv.Itoa(i)] = 1
	}
	step = c.Steps[0]
	c.Steps = make([]CommandStep, 64)
	for i := range c.Steps {
		c.Steps[i] = step
	}
	if _, _, e = snapshotCommand(c, "public"); !errors.Is(e, ErrCommandInput) {
		t.Fatal("node budget ignored")
	}
}
