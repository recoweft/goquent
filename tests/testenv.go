package tests

import (
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"github.com/recoweft/goquent/orm"
)

const (
	defaultMySQLTestDSN    = "root:password@tcp(127.0.0.1:3306)/testdb?parseTime=true"
	defaultPostgresTestDSN = "postgres://postgres:password@127.0.0.1:5432/testdb?sslmode=disable"
)

func lookupTestDSN(envKey, fallback string) (dsn string, explicit bool) {
	if dsn = os.Getenv(envKey); dsn != "" {
		return dsn, true
	}
	return fallback, false
}

func isConnectionUnavailable(err error) bool {
	var opErr *net.OpError
	return errors.As(err, &opErr)
}

func openTestDB(t testing.TB, driverName, dsn string, explicit bool) *orm.DB {
	t.Helper()

	var lastErr error
	for i := 0; i < 20; i++ {
		db, err := orm.OpenWithDriver(driverName, dsn)
		if err == nil {
			return db
		}
		lastErr = err
		time.Sleep(250 * time.Millisecond)
	}

	if !explicit && isConnectionUnavailable(lastErr) {
		t.Skipf("%s not available", driverName)
	}
	t.Fatalf("open %s: %v", driverName, lastErr)
	return nil
}
