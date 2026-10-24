package tests

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/query"
)

// This suite creates only unique objects in the explicitly opted-in disposable
// fixture. Dialects and cases are sequential, including process-wide casts.
func TestPerformancePlanNativeBoundaries(t *testing.T) {
	if os.Getenv("GOQUENT_NATIVE_DISPOSABLE_FIXTURE") != "1" ||
		os.Getenv("TEST_MYSQL_DSN") != defaultMySQLTestDSN ||
		os.Getenv("TEST_DB_DSN") != defaultMySQLTestDSN ||
		os.Getenv("TEST_POSTGRES_DSN") != defaultPostgresTestDSN {
		t.Skip("disposable fixture opt-in and three explicit fixture DSNs required")
	}
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			dsn := os.Getenv("TEST_MYSQL_DSN")
			if dialect == "postgres" {
				dsn = os.Getenv("TEST_POSTGRES_DSN")
			}
			db, err := orm.OpenWithDriver(dialect, dsn)
			if err != nil {
				t.Fatal("boundary fixture connection failed")
			}
			t.Cleanup(func() {
				if db.Close() != nil {
					t.Error("boundary fixture pool cleanup failed")
				}
			})
			name := boundaryName(t)
			table := name
			if dialect == "postgres" {
				table = "public." + name
			}
			boundaryExec(t, db.SQLDB(), "CREATE TABLE "+table+" (id BIGINT PRIMARY KEY, hidden BIGINT)")
			t.Cleanup(func() {
				boundaryCleanupExec(t, db.SQLDB(), "DROP TABLE "+table)
				statement := "SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME=?"
				if dialect == "postgres" {
					statement = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public' AND table_name=$1"
				}
				boundaryAbsent(t, db.SQLDB(), statement, name)
			})
			boundaryExec(t, db.SQLDB(), "INSERT INTO "+table+" VALUES (1,1),(2,2)")
			boundaryControl(t, db, table)
			t.Run("restricted_role", func(t *testing.T) { boundaryRestricted(t, db, dialect, dsn, table) })
			if dialect == "postgres" {
				t.Run("custom_operator", func(t *testing.T) { boundaryPGCustom(t, db, false) })
				t.Run("custom_cast", func(t *testing.T) { boundaryPGCustom(t, db, true) })
			}
			// MySQL has no corresponding CREATE OPERATOR/CREATE CAST facility. There is
			// deliberately no passing MySQL subtest pretending to test those features.
			for _, fault := range []string{"disconnect", "timeout", "cancel"} {
				t.Run("network_"+fault, func(t *testing.T) { boundaryNetwork(t, db, dialect, dsn, table, fault) })
			}
			boundaryControl(t, db, table)
		})
	}
}

func boundaryName(t *testing.T) string {
	t.Helper()
	var b [10]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal("fixture identity unavailable")
	}
	return "gqm09_" + hex.EncodeToString(b[:])
}

func boundaryExec(t *testing.T, db *sql.DB, statement string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, statement); err != nil {
		t.Fatal("boundary fixture statement failed")
	}
}

func boundaryControl(t *testing.T, db *orm.DB, table string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p, err := db.Table(table).Select("id").Where("id", int64(1)).Limit(1).ExplainPerformance(ctx, query.PerformanceOptions{})
	if err != nil || p == nil || !p.Native() || p.Unknown || p.PublicView().Unknown || len(p.Raw) == 0 {
		t.Fatal("normal native control failed")
	}
}

func boundaryRefusal(t *testing.T, p *query.PerformancePlan, err error, want query.PerformanceError) {
	t.Helper()
	if err != want || errors.Unwrap(err) != nil || fmt.Sprintf("%v", err) != string(want) || fmt.Sprintf("%+v", err) != string(want) {
		t.Fatal("fixed failure classification mismatch")
	}
	if p != nil {
		t.Fatal("failed collection retained performance evidence")
	}
}

func boundaryRestricted(t *testing.T, owner *orm.DB, dialect, dsn, table string) {
	raw := owner.SQLDB()
	name, password := boundaryName(t), boundaryName(t)
	var limitedDSN string
	if dialect == "mysql" {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		_, err := raw.ExecContext(ctx, "CREATE USER '"+name+"'@'%' IDENTIFIED BY '"+password+"'")
		cancel()
		if err != nil {
			t.Skip("restricted role setup unavailable")
		}
		t.Cleanup(func() {
			boundaryCleanupExec(t, raw, "DROP USER '"+name+"'@'%'")
			boundaryAbsent(t, raw, "SELECT COUNT(*) FROM mysql.user WHERE User=?", name)
		})
		boundaryExec(t, raw, "GRANT SELECT(id) ON testdb."+table+" TO '"+name+"'@'%'")
		cfg, err := mysql.ParseDSN(dsn)
		if err != nil {
			t.Fatal("fixture DSN invalid")
		}
		cfg.User, cfg.Passwd = name, password
		limitedDSN = cfg.FormatDSN()
	} else {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		_, err := raw.ExecContext(ctx, "CREATE ROLE "+name+" LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS PASSWORD '"+password+"'")
		cancel()
		if err != nil {
			t.Skip("restricted role setup unavailable")
		}
		t.Cleanup(func() {
			boundaryCleanupExec(t, raw, "REVOKE ALL ON "+table+" FROM "+name)
			boundaryCleanupExec(t, raw, "DROP ROLE "+name)
			boundaryAbsent(t, raw, "SELECT COUNT(*) FROM pg_catalog.pg_roles WHERE rolname=$1", name)
		})
		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatal("fixture DSN invalid")
		}
		u.User = url.UserPassword(name, password)
		limitedDSN = u.String()
	}
	limited, err := orm.OpenWithDriver(dialect, limitedDSN)
	if err != nil {
		t.Fatal("restricted connection failed")
	}
	t.Cleanup(func() {
		if limited.Close() != nil {
			t.Error("restricted pool cleanup failed")
		}
	})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if dialect == "mysql" {
		var columns, broad int
		err = limited.SQLDB().QueryRowContext(ctx, `SELECT
   (SELECT COUNT(*) FROM information_schema.COLUMN_PRIVILEGES WHERE TABLE_SCHEMA='testdb' AND TABLE_NAME=? AND PRIVILEGE_TYPE='SELECT'),
   (SELECT COUNT(*) FROM information_schema.USER_PRIVILEGES WHERE PRIVILEGE_TYPE='SELECT')+
   (SELECT COUNT(*) FROM information_schema.SCHEMA_PRIVILEGES WHERE PRIVILEGE_TYPE='SELECT')+
   (SELECT COUNT(*) FROM information_schema.TABLE_PRIVILEGES WHERE PRIVILEGE_TYPE='SELECT')`, table).Scan(&columns, &broad)
		if err != nil || columns != 1 || broad != 0 {
			t.Fatal("column-only effective privilege check failed")
		}
	} else {
		var tableSelect, columnSelect, super, bypass, inherit bool
		var memberships int
		err = limited.SQLDB().QueryRowContext(ctx, `SELECT has_table_privilege(current_user,$1,'SELECT'), has_any_column_privilege(current_user,$1,'SELECT'), rolsuper, rolbypassrls, rolinherit, (SELECT count(*) FROM pg_auth_members WHERE member=r.oid) FROM pg_roles r WHERE rolname=current_user`, table).Scan(&tableSelect, &columnSelect, &super, &bypass, &inherit, &memberships)
		if err != nil || tableSelect || columnSelect || super || bypass || inherit || memberships != 0 {
			t.Fatal("restricted effective privilege check failed")
		}
	}
	p, err := limited.Table(table).Select("id").Limit(1).ExplainPerformance(t.Context(), query.PerformanceOptions{})
	want := query.ErrPerformanceUnsupported
	if dialect == "postgres" {
		want = query.ErrPerformanceQuery
	}
	boundaryRefusal(t, p, err, want)
	// The same principal and pool become a normal control only after an explicit
	// grant on this test-owned table. No existing role or object grant is changed.
	if dialect == "mysql" {
		boundaryExec(t, raw, "GRANT SELECT ON testdb."+table+" TO '"+name+"'@'%'")
	} else {
		boundaryExec(t, raw, "GRANT SELECT ON "+table+" TO "+name)
	}
	boundaryControl(t, limited, table)
	if limited.SQLDB().Stats().InUse != 0 {
		t.Fatal("restricted collection retained a connection")
	}
}

func boundaryPGCustom(t *testing.T, db *orm.DB, cast bool) {
	raw, ns := db.SQLDB(), boundaryName(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	_, err := raw.ExecContext(ctx, "CREATE SCHEMA "+ns)
	cancel()
	if err != nil {
		t.Skip("custom object setup unavailable")
	}
	t.Cleanup(func() {
		boundaryCleanupExec(t, raw, "DROP SCHEMA "+ns+" CASCADE")
		boundaryAbsent(t, raw, "SELECT COUNT(*) FROM pg_catalog.pg_namespace WHERE nspname=$1", ns)
	})
	boundaryExec(t, raw, "CREATE SEQUENCE "+ns+".marker")
	boundaryExec(t, raw, "CREATE TABLE "+ns+".target (id BIGINT)")
	if cast {
		boundaryExec(t, raw, "CREATE TYPE "+ns+".box AS (n BIGINT)")
		boundaryExec(t, raw, "CREATE FUNCTION "+ns+".convert(BIGINT) RETURNS "+ns+".box LANGUAGE plpgsql VOLATILE AS $$ BEGIN PERFORM nextval('"+ns+".marker'); RETURN ROW($1)::"+ns+".box; END $$")
		boundaryExec(t, raw, "CREATE CAST (BIGINT AS "+ns+".box) WITH FUNCTION "+ns+".convert(BIGINT) AS ASSIGNMENT")
	} else {
		boundaryExec(t, raw, "CREATE FUNCTION "+ns+".equal(BIGINT,BIGINT) RETURNS BOOLEAN LANGUAGE plpgsql VOLATILE AS $$ BEGIN PERFORM nextval('"+ns+".marker'); RETURN $1 OPERATOR(pg_catalog.=) $2; END $$")
		boundaryExec(t, raw, "CREATE OPERATOR "+ns+".= (LEFTARG=BIGINT, RIGHTARG=BIGINT, FUNCTION="+ns+".equal)")
		boundaryExec(t, raw, "CREATE OPERATOR CLASS "+ns+".custom_ops FOR TYPE BIGINT USING btree AS OPERATOR 1 pg_catalog.< (BIGINT,BIGINT), OPERATOR 2 pg_catalog.<= (BIGINT,BIGINT), OPERATOR 3 "+ns+".= (BIGINT,BIGINT), OPERATOR 4 pg_catalog.>= (BIGINT,BIGINT), OPERATOR 5 pg_catalog.> (BIGINT,BIGINT), FUNCTION 1 pg_catalog.btint8cmp(BIGINT,BIGINT)")
		// Empty index construction does not invoke the custom code. The catalog gate
		// must reject this index even for a structurally ordinary integer predicate.
		boundaryExec(t, raw, "CREATE INDEX custom_index ON "+ns+".target (id "+ns+".custom_ops)")
	}
	marker := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		var called bool
		if raw.QueryRowContext(ctx, "SELECT is_called FROM "+ns+".marker").Scan(&called) != nil || called {
			t.Fatal("custom code marker changed")
		}
	}
	marker()
	spy, spyDSN := newBoundaryProxy(t, "postgres", os.Getenv("TEST_POSTGRES_DSN"))
	spy.mu.Lock()
	spy.auditTarget = `"` + ns + `"."target"`
	spy.mu.Unlock()
	observed, openErr := orm.OpenWithDriver("postgres", spyDSN)
	if openErr != nil {
		t.Fatal("custom observer connection failed")
	}
	t.Cleanup(func() {
		if observed.Close() != nil {
			t.Error("custom observer pool cleanup failed")
		}
	})
	checkDispatch := func(explains int) {
		t.Helper()
		spy.mu.Lock()
		defer spy.mu.Unlock()
		if spy.targetExplain != explains || spy.targetExecute != 0 {
			t.Fatal("custom target dispatch boundary failed")
		}
	}
	p, err := observed.Table(ns+".target").Select("id").Where("id", int64(1)).Limit(1).ExplainPerformance(t.Context(), query.PerformanceOptions{})
	boundaryRefusal(t, p, err, query.ErrPerformanceUnsupported)
	// Raw custom expressions are separately refused by the structural boundary.
	expression := "id OPERATOR(" + ns + ".=) 1"
	if cast {
		expression = "id::" + ns + ".box IS NOT NULL"
	}
	p, err = observed.Table(ns+".target").Select("id").WhereRaw(expression, nil).Limit(1).ExplainPerformance(t.Context(), query.PerformanceOptions{})
	boundaryRefusal(t, p, err, query.ErrPerformanceUnsupported)
	marker()
	checkDispatch(0)
	if cast {
		boundaryExec(t, raw, "DROP CAST (BIGINT AS "+ns+".box)")
	} else {
		boundaryExec(t, raw, "DROP INDEX "+ns+".custom_index")
	}
	boundaryControl(t, observed, ns+".target")
	checkDispatch(1)
	marker()
	if raw.Stats().InUse != 0 {
		t.Fatal("custom collection retained a connection")
	}
}

// Cleanup errors must not prevent the remaining owned-object cleanup attempts.
func boundaryCleanupExec(t *testing.T, db *sql.DB, statement string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, statement); err != nil {
		t.Error("owned object cleanup failed")
	}
}
func boundaryAbsent(t *testing.T, db *sql.DB, statement string, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var remaining int
	if db.QueryRowContext(ctx, statement, name).Scan(&remaining) != nil {
		t.Error("owned object cleanup observation failed")
		return
	}
	if remaining != 0 {
		t.Error("owned object remains after cleanup")
	}
}
