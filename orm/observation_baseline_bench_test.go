package orm

import (
	"context"
	"database/sql"
	sqldriver "database/sql/driver"
	"errors"
	"github.com/recoweft/goquent/orm/driver"
	"io"
	"sync"
	"testing"
)

// This in-memory driver is a benchmark fixture, never native execution evidence.
type observationBenchDriver struct{}
type observationBenchConn struct{}
type observationBenchRows struct{ done bool }

func (observationBenchDriver) Open(string) (sqldriver.Conn, error) {
	return observationBenchConn{}, nil
}
func (observationBenchConn) Prepare(string) (sqldriver.Stmt, error) {
	return nil, errors.New("unsupported fixture operation")
}
func (observationBenchConn) Close() error { return nil }
func (observationBenchConn) Begin() (sqldriver.Tx, error) {
	return nil, errors.New("unsupported fixture operation")
}
func (observationBenchConn) ExecContext(context.Context, string, []sqldriver.NamedValue) (sqldriver.Result, error) {
	return sqldriver.RowsAffected(1), nil
}
func (observationBenchConn) QueryContext(context.Context, string, []sqldriver.NamedValue) (sqldriver.Rows, error) {
	return &observationBenchRows{}, nil
}
func (*observationBenchRows) Columns() []string { return []string{"id"} }
func (*observationBenchRows) Close() error      { return nil }
func (r *observationBenchRows) Next(dest []sqldriver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = int64(1)
	return nil
}

var observationBenchRegister sync.Once

func observationBenchDB(b *testing.B) *DB {
	b.Helper()
	observationBenchRegister.Do(func() { sql.Register("goquent-observation-benchmark", observationBenchDriver{}) })
	raw, e := sql.Open("goquent-observation-benchmark", "")
	if e != nil {
		b.Fatal("fixture setup")
	}
	b.Cleanup(func() { _ = raw.Close() })
	return NewDB(raw, driver.PostgresDialect{}).RequireRawApproval("benchmark")
}

// One benchmark operation is a request with eight writes and eight structured reads.
func observationBenchRequest(b *testing.B, db *DB, ctx context.Context) {
	b.Helper()
	for i := 0; i < 8; i++ {
		if _, e := db.ExecContext(ctx, "UPDATE observation_fixture SET value=$1 WHERE id=$2", 1, 2); e != nil {
			b.Fatal("dispatch failed")
		}
		var row map[string]any
		if e := db.Table("observation_fixture").WithContext(ctx).Select("id").Where("id", i).Limit(1).FirstMap(&row); e != nil {
			b.Fatal("read failed")
		}
	}
}
func BenchmarkObservationDisabled(b *testing.B) {
	db := observationBenchDB(b)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		observationBenchRequest(b, db, ctx)
	}
}
