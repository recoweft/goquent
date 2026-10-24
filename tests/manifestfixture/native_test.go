package manifestfixture

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/migration"
)

var evidenceOutput = flag.String("manifest-evidence-output", "", "existing protected local output directory")

const mysqlDSN = "root:password@tcp(127.0.0.1:3306)/testdb?parseTime=true"
const postgresDSN = "postgres://postgres:password@127.0.0.1:5432/testdb?sslmode=disable"

type readOnly struct {
	db *sql.DB
	t  *testing.T
}

func (r readOnly) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	if !strings.HasPrefix(strings.TrimSpace(q), "SELECT ") {
		r.t.Fatal("non-read catalog statement")
	}
	return r.db.QueryContext(ctx, q, args...)
}
func (r readOnly) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
	r.t.Fatal("unexpected query-row catalog statement")
	return nil
}

func TestManifestEvidenceNative(t *testing.T) {
	if os.Getenv("GOQUENT_NATIVE_DISPOSABLE_FIXTURE") != "1" || os.Getenv("TEST_MYSQL_DSN") != mysqlDSN || os.Getenv("TEST_DB_DSN") != mysqlDSN || os.Getenv("TEST_POSTGRES_DSN") != postgresDSN {
		t.Skip("explicit disposable fixture opt-in and three DSNs required")
	}
	out := *evidenceOutput
	if out == "" {
		out = t.TempDir()
		if os.Chmod(out, 0700) != nil {
			t.Fatal("private directory failed")
		}
	}
	root, e := filepath.Abs("../..")
	if e != nil {
		t.Fatal("root unavailable")
	}
	resolved, e := filepath.EvalSymlinks(out)
	if e != nil || resolved != out || !filepath.IsAbs(out) {
		t.Fatal("private output required")
	}
	rel, e := filepath.Rel(root, out)
	if e != nil || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))) {
		t.Fatal("output must be outside checkout")
	}
	info, e := os.Stat(out)
	if e != nil || !info.IsDir() || info.Mode().Perm() != 0700 {
		t.Fatal("private output required")
	}
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var random [10]byte
			if _, e = rand.Read(random[:]); e != nil {
				t.Fatal("unique name unavailable")
			}
			table := "gq_manifest_" + hex.EncodeToString(random[:])
			expected := Expected(dialect, table)
			stored, e := manifest.Load(dialect + ".manifest.json")
			if e != nil {
				t.Fatal("stored manifest unavailable")
			}
			baseline := Expected(dialect, "gq_manifest_fixture")
			baselineManifest, err := manifest.Generate(manifest.Options{DatabaseSchema: &baseline})
			if err != nil || baselineManifest.DatabaseFingerprint != stored.DatabaseFingerprint {
				t.Fatal("committed database expectation stale")
			}
			// Only DB identity binding changes. Stored code/schema/policy remain immutable;
			// never rebuild them from current inputs before detecting stale evidence.
			dbOnly, e := manifest.Generate(manifest.Options{DatabaseSchema: &expected})
			if e != nil {
				t.Fatal("declaration failed")
			}
			stored.DatabaseFingerprint = dbOnly.DatabaseFingerprint
			save := func(name string, v any) {
				t.Helper()
				b, e := json.MarshalIndent(v, "", "  ")
				if e != nil {
					t.Fatal("encode failed")
				}
				f, e := os.OpenFile(filepath.Join(out, dialect+"-"+name+".json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
				if e != nil {
					t.Fatal("private artifact failed")
				}
				_, e = f.Write(b)
				ce := f.Close()
				if e != nil || ce != nil {
					t.Fatal("private artifact failed")
				}
			}
			save("expected", expected)
			save("manifest", stored)
			drv, dsn := dialect, mysqlDSN
			var d driver.Dialect = driver.MySQLDialect{}
			if dialect == "postgres" {
				drv, dsn, d = "postgres", postgresDSN, driver.PostgresDialect{}
			}
			db, e := sql.Open(drv, dsn)
			if e != nil {
				t.Fatal("fixture open failed")
			}
			defer db.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			exec := func(q string) {
				t.Helper()
				if _, e := db.ExecContext(ctx, q); e != nil {
					t.Fatal("fixture statement failed")
				}
			}
			statements := DDL(table)
			exec(statements[0])
			// Cleanup must run before closing the pool.
			defer func() {
				cleanup, stop := context.WithTimeout(context.Background(), 10*time.Second)
				defer stop()
				if _, e := db.ExecContext(cleanup, "DROP TABLE "+table); e != nil {
					t.Error("owned table cleanup failed")
				}
			}()
			// The deferred drop owns cleanup; no pre-existing object is dropped.
			exec(statements[1])
			read := func(name string) {
				t.Helper()
				s, e := migration.ReadSchema(ctx, readOnly{db, t}, d, migration.WithSchemaReadTables(table))
				if e != nil || len(s.Tables) != 1 || s.Tables[0].Name != table {
					t.Fatal("live export failed")
				}
				save(name, s)
				if name == "current" {
					actual, err := manifest.Generate(manifest.Options{DatabaseSchema: &s})
					if err != nil || actual.DatabaseFingerprint != stored.DatabaseFingerprint {
						t.Fatal("independent database declaration mismatch")
					}
				}
			}
			read("current")
			exec("ALTER TABLE " + table + " ADD COLUMN evidence_canary BIGINT NULL")
			read("drift")
		})
	}
}
