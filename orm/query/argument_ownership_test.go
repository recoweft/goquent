package query

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	ormdriver "github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/internal/valuecopy"
	"github.com/recoweft/goquent/orm/predicate"
)

func ownershipArgs(n int) []any {
	out := make([]any, n)
	for i := range out {
		switch i % 6 {
		case 0:
			out[i] = int64(i)
		case 1:
			out[i] = strconv.Itoa(i)
		case 2:
			out[i] = nil
		case 3:
			out[i] = []byte(nil)
		case 4:
			out[i] = i%2 == 0
		case 5:
			out[i] = float64(i)
		}
	}
	return out
}
func assertOwnershipArgs(t *testing.T, got, want []any) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("argument count %d != %d", len(got), len(want))
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Fatalf("argument %d: type/value %T/%v != %T/%v", i, got[i], got[i], want[i], want[i])
		}
	}
}
func assertOwnershipTree(t *testing.T, tree *predicate.Node, want []any, rendered bool) {
	t.Helper()
	if tree == nil || (rendered && len(tree.Parameters) != len(want)) || len(tree.Values) != len(want) {
		t.Fatal("missing IN trace")
	}
	for i, v := range tree.Values {
		if (rendered && tree.Parameters[i] != i) || v.Isolation != "detached" || !reflect.DeepEqual(v.Data, want[i]) {
			t.Fatalf("trace mismatch at %d", i)
		}
	}
}

// All large statements are built in memory; no database bind limit is involved.
func TestArgumentOwnershipBoundaries(t *testing.T) {
	for _, d := range []ormdriver.Dialect{ormdriver.MySQLDialect{}, ormdriver.PostgresDialect{}} {
		for _, n := range []int{65535, 65536, 65537} {
			t.Run(fmt.Sprintf("%T/%d", d, n), func(t *testing.T) {
				want := ownershipArgs(n)
				exec := &recordingExec{}
				q := New(exec, "users", d).WhereIn("id", want)
				sql, args, err := q.Build()
				if err != nil {
					t.Fatal(err)
				}
				assertOwnershipArgs(t, args, want)
				placeholders := make([]string, n)
				for i := range placeholders {
					placeholders[i] = "?"
					if _, ok := d.(ormdriver.PostgresDialect); ok {
						placeholders[i] = "$" + strconv.Itoa(i+1)
					}
				}
				if !strings.Contains(sql, "IN ("+strings.Join(placeholders, ", ")+")") {
					t.Fatal("placeholder sequence mismatch")
				}
				sql2, args2, snap, err := q.builder.BuildSnapshot()
				if err != nil || sql2 != sql {
					t.Fatalf("BuildSnapshot: %v", err)
				}
				assertOwnershipArgs(t, args2, want)
				assertOwnershipTree(t, snap.WhereTree, want, true)
				copied := newSelectBuilder(d)
				q.builder.CopyStateToSelect(copied)
				_, copiedArgs, err := copied.Build()
				if err != nil {
					t.Fatal(err)
				}
				assertOwnershipArgs(t, copiedArgs, want)
				rawPlan := NewRawPlan(sql, args...)
				snap2 := q.builder.Snapshot()
				assertOwnershipTree(t, snap2.WhereTree, want, false)
				p, err := q.Plan(context.Background())
				if err != nil || p.SQL != sql {
					t.Fatalf("Plan: %v", err)
				}
				p2, err := q.Plan(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				assertOwnershipArgs(t, p.Params, want)
				assertOwnershipTree(t, p.WhereTree, want, true)
				for _, marshal := range []func() ([]byte, error){p.ToJSON, func() ([]byte, error) { return json.Marshal(p) }, func() ([]byte, error) { return json.Marshal(*p) }, func() ([]byte, error) { return json.MarshalIndent(p, "", "  ") }} {
					if _, err := marshal(); !errors.Is(err, ErrOutputBudget) {
						t.Fatalf("output limit: %v", err)
					}
				}
				if !strings.Contains(p.String(), "omitted; unverified") {
					t.Fatal("missing String marker")
				}
				assertOwnershipArgs(t, p.Params, want)
				if p.SQL != sql {
					t.Fatal("output changed SQL")
				}
				// Rebuild and another builder exercise cleanup again. Correctness does not
				// depend on sync.Pool choosing any particular backing array.
				_, again, err := q.Build()
				if err != nil {
					t.Fatal(err)
				}
				_, other, err := New(exec, "other", d).WhereIn("id", want).Build()
				if err != nil {
					t.Fatal(err)
				}
				for _, v := range [][]any{args, args2, again, other, p.Params, p2.Params} {
					assertOwnershipArgs(t, v, want)
				}
				for i := range args {
					args[i] = "changed"
					p.Params[i] = "plan changed"
					snap.WhereTree.Values[i].Data = "snapshot changed"
				}
				for _, v := range [][]any{args2, again, other, p2.Params, copiedArgs, rawPlan.Params} {
					assertOwnershipArgs(t, v, want)
				}
				assertOwnershipTree(t, snap2.WhereTree, want, false)
				assertOwnershipTree(t, p2.WhereTree, want, true)
				_, fresh, err := q.Build()
				if err != nil {
					t.Fatal(err)
				}
				assertOwnershipArgs(t, fresh, want)
				if exec.calls != 0 {
					t.Fatal("build or plan executed SQL")
				}
			})
		}
	}
}

func TestArgumentOwnershipOpaqueAndAggregatePayload(t *testing.T) {
	large := make([]any, valuecopy.MaxSlots)
	large[0], large[len(large)-1] = "first", "last"
	a, b := make([]byte, valuecopy.MaxBytes/2+1), make([]byte, valuecopy.MaxBytes/2+1)
	a[0], b[0] = 1, 2
	pointer := new(int)
	want := []any{large, a, b, pointer, nil, []byte(nil)}
	q := newPlanTestQuery(&recordingExec{}).WhereGroup(func(q *Query) { q.WhereIn("payload", want) })
	_, args, snap, err := q.builder.BuildSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	assertOwnershipArgs(t, args, want)
	if reflect.ValueOf(args[0]).Pointer() != reflect.ValueOf(large).Pointer() || args[3] != pointer {
		t.Fatal("opaque reference lost")
	}
	if snap.WhereTree.Correspondence != "unverified" {
		t.Fatal("ancestor not marked unverified")
	}
	leaf := leaves(snap.WhereTree)[0]
	for _, i := range []int{0, 2, 3} {
		if leaf.Values[i].Isolation != "unverified" || leaf.Values[i].Reason == "" {
			t.Fatalf("missing isolation reason %d", i)
		}
	}
	p, err := q.Plan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	assertOwnershipArgs(t, p.Params, want)
	for i := range args {
		args[i] = nil
	}
	assertOwnershipArgs(t, p.Params, want)
	_, again, err := q.Build()
	if err != nil {
		t.Fatal(err)
	}
	assertOwnershipArgs(t, again, want)
}

type ownershipExecutor struct {
	recordingExec
	args []any
	sql  string
	err  error
}

func (e *ownershipExecutor) Query(s string, args ...any) (*sql.Rows, error) {
	e.calls++
	e.sql, e.args = s, args
	return nil, e.err
}
func TestLargeArgumentsReachCustomExecutor(t *testing.T) {
	for _, d := range []ormdriver.Dialect{ormdriver.MySQLDialect{}, ormdriver.PostgresDialect{}} {
		e := &ownershipExecutor{err: errors.New("recorded without DB")}
		want := ownershipArgs(65537)
		q := New(e, "users", d).WhereIn("id", want)
		sql, args, err := q.Build()
		if err != nil {
			t.Fatal(err)
		}
		var rows []map[string]any
		if err := q.GetMaps(&rows); !errors.Is(err, e.err) {
			t.Fatalf("executor error timing: %v", err)
		}
		if e.calls != 1 || e.sql != sql {
			t.Fatal("execution SQL/call count changed")
		}
		assertOwnershipArgs(t, e.args, want)
		for i := range args {
			args[i] = nil
		}
		_, _, err = q.Build()
		if err != nil {
			t.Fatal(err)
		}
		assertOwnershipArgs(t, e.args, want)
	}
}
