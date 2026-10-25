package storedmodel

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
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

func nativeEnabled() bool {
	return os.Getenv("GOQUENT_NATIVE_DISPOSABLE_FIXTURE") == "1" && os.Getenv("TEST_MYSQL_DSN") == mysqlDSN && os.Getenv("TEST_DB_DSN") == mysqlDSN && os.Getenv("TEST_POSTGRES_DSN") == postgresDSN
}

type catalogReader struct{ db *sql.DB }

func (r catalogReader) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	if !strings.HasPrefix(strings.TrimSpace(q), "SELECT ") {
		panic("catalog read boundary")
	}
	return r.db.QueryContext(ctx, q, args...)
}
func (r catalogReader) QueryRowContext(context.Context, string, ...any) *sql.Row {
	panic("catalog query-row boundary")
}

func privateOutput(t *testing.T, out string) string {
	t.Helper()
	if out == "" {
		out = t.TempDir()
		if os.Chmod(out, 0700) != nil {
			t.Fatal("private directory failed")
		}
	}
	root, e := filepath.Abs("../../..")
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
	return out
}
func saveNative(t *testing.T, out, name string, value any) {
	t.Helper()
	b, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		t.Fatal("artifact encoding failed")
	}
	f, e := os.OpenFile(filepath.Join(out, name+".json"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		t.Fatal("private artifact unavailable")
	}
	_, e = f.Write(b)
	ce := f.Close()
	if e != nil || ce != nil {
		t.Fatal("private artifact write failed")
	}
}

// collectNative fixes expectations before opening the DB. It substitutes only
// the owned table/index names, including their raw definition and context list.
func collectNative(t *testing.T, dialect, out string, force bool) {
	t.Helper()
	var entropy [10]byte
	if _, e := rand.Read(entropy[:]); e != nil {
		t.Fatal("owned identity unavailable")
	}
	table := "gq_saved_" + hex.EncodeToString(entropy[:])
	expected := ExpectedDatabase(dialect, table)
	path := "../goquent.manifest.json"
	if dialect == "postgres" {
		path = "postgres.manifest.json"
	}
	stored, e := manifest.Load(path)
	if e != nil {
		t.Fatal("stored expectation unavailable")
	}
	canonical := ExpectedDatabase(dialect, CanonicalTable)
	baseline, e := manifest.Generate(manifest.Options{DatabaseSchema: &canonical})
	if e != nil || baseline.DatabaseFingerprint != stored.DatabaseFingerprint {
		t.Fatal("independent canonical expectation stale")
	}
	bound, e := manifest.Generate(manifest.Options{DatabaseSchema: &expected})
	if e != nil {
		t.Fatal("owned declaration failed")
	}
	stored.DatabaseFingerprint = bound.DatabaseFingerprint
	saveNative(t, out, dialect+"-expected", expected)
	saveNative(t, out, dialect+"-manifest", stored)
	drv, dsn := "mysql", mysqlDSN
	var d driver.Dialect = driver.MySQLDialect{}
	if dialect == "postgres" {
		drv, dsn, d = "postgres", postgresDSN, driver.PostgresDialect{}
	}
	db, e := sql.Open(drv, dsn)
	if e != nil {
		t.Fatal("native open failed")
	}
	db.SetMaxOpenConns(2)
	created := false
	// Registered before the first assertion or acquisition. This also executes
	// after t.Fatal in the child process, without fabricating a passing test leaf.
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ok := true
		if created {
			if _, e := db.ExecContext(ctx, "DROP TABLE "+table); e != nil {
				t.Error("owned cleanup failed")
				ok = false
			}
			s, e := migration.ReadSchema(ctx, catalogReader{db}, d, migration.WithSchemaReadTables(table))
			if e != nil || len(s.Tables) != 0 {
				t.Error("owned absence unverified")
				ok = false
			}
		}
		var control int
		if e := db.QueryRowContext(ctx, "SELECT 1").Scan(&control); e != nil || control != 1 {
			t.Error("native control failed")
			ok = false
		}
		if e := db.Close(); e != nil {
			t.Error("pool close failed")
			ok = false
		}
		if db.Stats().OpenConnections != 0 {
			t.Error("pool absence failed")
			ok = false
		}
		if ok && created {
			saveNative(t, out, dialect+"-cleanup", map[string]bool{"owned_removed": true, "absence": true, "control": true, "pool_closed": true, "forced_assertion": force})
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	statements := OwnedDDL(table)
	if _, e := db.ExecContext(ctx, statements[0]); e != nil {
		t.Fatal("owned create failed")
	}
	created = true
	if _, e := db.ExecContext(ctx, statements[1]); e != nil {
		t.Fatal("owned index failed")
	}
	if force {
		t.Fatal("EXPECTED_OWNED_ASSERTION")
	}
	read := func(name string) migration.Schema {
		t.Helper()
		s, e := migration.ReadSchema(ctx, catalogReader{db}, d, migration.WithSchemaReadTables(table))
		if e != nil || len(s.Tables) != 1 {
			t.Fatal("native current unavailable")
		}
		saveNative(t, out, dialect+"-"+name, s)
		return s
	}
	current := read("current")
	actual, e := manifest.Generate(manifest.Options{DatabaseSchema: &current})
	if e != nil || actual.DatabaseFingerprint != stored.DatabaseFingerprint {
		t.Fatal("independent native expectation mismatch")
	}
	if _, e := db.ExecContext(ctx, "ALTER TABLE "+table+" ADD COLUMN evidence_canary BIGINT NULL"); e != nil {
		t.Fatal("owned drift failed")
	}
	drift := read("drift")
	changed, e := manifest.Generate(manifest.Options{DatabaseSchema: &drift})
	if e != nil || changed.DatabaseFingerprint == stored.DatabaseFingerprint {
		t.Fatal("native drift undetected")
	}
}

func TestManifestEvidenceNative(t *testing.T) {
	if !nativeEnabled() {
		t.Skip("explicit disposable opt-in and three DSNs required")
	}
	out := privateOutput(t, *evidenceOutput)
	for _, dialect := range []string{"mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			collectNative(t, dialect, out, false)
			t.Run("assertion-cleanup", func(t *testing.T) {
				child := filepath.Join(out, dialect+"-forced")
				if os.Mkdir(child, 0700) != nil {
					t.Fatal("private child directory failed")
				}
				if os.Chmod(os.Args[0], 0700) != nil {
					t.Fatal("private child executable failed")
				}
				cmd := exec.Command(os.Args[0], "-test.run=^TestOwnedAssertionChild$", "-test.count=1")
				cmd.Env = append(os.Environ(), "GOQUENT_SAVED_CHILD="+dialect, "GOQUENT_SAVED_OUTPUT="+child)
				raw, e := cmd.CombinedOutput()
				if os.WriteFile(filepath.Join(child, "child.log"), raw, 0600) != nil {
					t.Fatal("child evidence unavailable")
				}
				exit, ok := e.(*exec.ExitError)
				if !ok || exit.ExitCode() != 1 || !strings.Contains(string(raw), "EXPECTED_OWNED_ASSERTION") {
					t.Fatal("forced assertion not observed")
				}
				b, e := os.ReadFile(filepath.Join(child, dialect+"-cleanup.json"))
				var receipt map[string]bool
				if e != nil || json.Unmarshal(b, &receipt) != nil || len(receipt) != 5 {
					t.Fatal("forced cleanup evidence unavailable")
				}
				for _, v := range receipt {
					if !v {
						t.Fatal("forced cleanup incomplete")
					}
				}
			})
		})
	}
}
func TestOwnedAssertionChild(t *testing.T) {
	dialect := os.Getenv("GOQUENT_SAVED_CHILD")
	if dialect == "" {
		t.Skip("subprocess-only deliberate assertion")
	}
	if !nativeEnabled() || (dialect != "mysql" && dialect != "postgres") {
		t.Fatal("child prerequisites invalid")
	}
	collectNative(t, dialect, privateOutput(t, os.Getenv("GOQUENT_SAVED_OUTPUT")), true)
}
