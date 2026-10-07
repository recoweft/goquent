package contracts_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/migration"
	"github.com/recoweft/goquent/orm/operation"
	"github.com/recoweft/goquent/orm/query"
	"github.com/recoweft/goquent/orm/review"
)

// Any DB call panics, including calls whose error a planner might ignore.
// No DB connection or live schema is available to these plans.
type noExecution struct{}

func (noExecution) Query(string, ...any) (*sql.Rows, error) { panic("Plan called Query") }
func (noExecution) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	panic("Plan called QueryContext")
}
func (noExecution) QueryRow(string, ...any) *sql.Row { panic("Plan called QueryRow") }
func (noExecution) QueryRowContext(context.Context, string, ...any) *sql.Row {
	panic("Plan called QueryRowContext")
}
func (noExecution) Exec(string, ...any) (sql.Result, error) { panic("Plan called Exec") }
func (noExecution) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	panic("Plan called ExecContext")
}

func TestSharedPlanSerializationAndReview(t *testing.T) {
	for _, dialect := range []struct {
		name  string
		value driver.Dialect
		sql   string
	}{
		{"mysql", driver.MySQLDialect{}, "SELECT `id`, `name` FROM `users` WHERE `id` = ? LIMIT 1"},
		{"postgres", driver.PostgresDialect{}, `SELECT "id", "name" FROM "users" WHERE "id" = $1 LIMIT 1`},
	} {
		t.Run(dialect.name, func(t *testing.T) {
			for _, id := range []string{"C01", "C02"} {
				t.Run(id, func(t *testing.T) {
					requireCase(t, id)
					requireCase(t, "C28")
					q := query.New(noExecution{}, "users", dialect.value)
					if id == "C01" {
						q = q.Select("id", "name").Where("id", 10).Limit(1)
					}
					plan, err := q.Plan(context.Background())
					if err != nil {
						t.Fatal(err)
					}
					if id == "C01" {
						if plan.SQL != dialect.sql || !reflect.DeepEqual(plan.Params, []any{10}) || plan.RiskLevel != query.RiskLow || plan.Limit == nil || *plan.Limit != 1 {
							t.Fatalf("filtered plan shape changed: %s", plan)
						}
					} else if plan.RiskLevel != query.RiskMedium {
						t.Fatalf("wildcard risk: %s", plan.RiskLevel)
					}
					data, err := plan.ToJSON()
					if err != nil {
						t.Fatal(err)
					}
					again, err := plan.ToJSON()
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(data, again) {
						t.Fatal("serialization is not deterministic for the same plan")
					}
					path := filepath.Join(t.TempDir(), "plan.json")
					if err := os.WriteFile(path, data, 0600); err != nil {
						t.Fatal(err)
					}
					var decoded query.QueryPlan
					readJSON(t, path, &decoded)
					// JSON numbers retain json.Number lexemes; selected semantics, not byte identity to
					// every historical wire field/consumer or a trusted executable artifact.
					if decoded.Operation != query.OperationSelect || decoded.SQL != plan.SQL || decoded.RiskLevel != plan.RiskLevel || !reflect.DeepEqual(decoded.Columns, plan.Columns) || !reflect.DeepEqual(decoded.Predicates, plan.Predicates) {
						t.Fatal("selected plan fields changed through JSON")
					}
					if id == "C01" && !reflect.DeepEqual(decoded.Params, []any{json.Number("10")}) {
						t.Fatalf("decoded params: %#v", decoded.Params)
					}
					report, err := review.Run(review.Options{Paths: []string{path}})
					if err != nil {
						t.Fatal(err)
					}
					if id == "C01" && len(report.Findings) != 0 {
						t.Fatalf("unexpected findings: %#v", report.Findings)
					}
					if id == "C02" {
						for _, code := range []string{query.WarningSelectStarUsed, query.WarningLimitMissing} {
							found := false
							for _, f := range report.Findings {
								if f.Code == code && f.Level == query.RiskMedium && !f.Suppressed && f.Location != nil && f.Location.File == path {
									found = true
								}
							}
							if !found {
								t.Fatalf("missing diagnostic %s: %#v", code, report.Findings)
							}
						}
						if !review.HasFindingsAtOrAbove(report, query.RiskMedium) {
							t.Fatal("medium diagnostic lost by threshold")
						}
					}
					var out bytes.Buffer
					if err := review.WriteJSON(&out, report); err != nil {
						t.Fatal(err)
					}
					reportPath := filepath.Join(t.TempDir(), "review.json")
					if err := os.WriteFile(reportPath, out.Bytes(), 0600); err != nil {
						t.Fatal(err)
					}
					var roundTrip review.ReviewReport
					readJSON(t, reportPath, &roundTrip)
					if roundTrip.Summary.Total != report.Summary.Total || roundTrip.Summary.Suppressed != report.Summary.Suppressed || roundTrip.Summary.HighestRisk != report.Summary.HighestRisk || !maps.Equal(roundTrip.Summary.ByLevel, report.Summary.ByLevel) || len(roundTrip.Findings) != len(report.Findings) {
						t.Fatal("review summary/count changed through JSON")
					}
					out.Reset()
					if err := review.WritePretty(&out, report); err != nil {
						t.Fatal(err)
					}
					if id == "C02" && (!bytes.Contains(out.Bytes(), []byte(query.WarningSelectStarUsed)) || !bytes.Contains(out.Bytes(), []byte(query.WarningLimitMissing))) {
						t.Fatal("pretty output lost diagnostics")
					}
				})
			}
		})
	}
}

func TestExampleFixtureCompatibility(t *testing.T) {
	var fixtures []struct {
		Path   string `json:"path"`
		SHA256 string `json:"sha256"`
	}
	readJSON(t, "testdata/examples.json", &fixtures)
	paths, err := filepath.Glob(filepath.Join(repo, "examples/ai-safe-orm/*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 5 || len(paths) != len(fixtures) {
		t.Fatal("example baseline file set changed; review compatibility explicitly")
	}
	seen := map[string]bool{}
	for _, fixture := range fixtures {
		if seen[fixture.Path] || !filepath.IsLocal(fixture.Path) {
			t.Fatalf("invalid fixture path %q", fixture.Path)
		}
		seen[fixture.Path] = true
		data, err := os.ReadFile(filepath.Join(repo, fixture.Path))
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != fixture.SHA256 {
			t.Fatalf("%s differs from the PR1 example baseline; do not blindly update hashes", fixture.Path)
		}
	}
	for _, path := range paths {
		relative, err := filepath.Rel(repo, path)
		if err != nil {
			t.Fatal(err)
		}
		if !seen[relative] {
			t.Fatalf("missing fixture %s", relative)
		}
	}
	base := filepath.Join(repo, "examples/ai-safe-orm")
	stored, err := manifest.Load(filepath.Join(base, "goquent.manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := manifest.Validate(stored); err != nil {
		t.Fatal(err)
	}
	var schema migration.Schema
	var policies []query.TablePolicy
	var spec operation.OperationSpec
	var values map[string]any
	readJSON(t, filepath.Join(base, "schema.json"), &schema)
	readJSON(t, filepath.Join(base, "policies.json"), &policies)
	readJSON(t, filepath.Join(base, "operation.json"), &spec)
	readJSON(t, filepath.Join(base, "values.json"), &values)
	current, err := manifest.Generate(manifest.Options{Dialect: stored.Dialect, GeneratedAt: stored.GeneratedAt, Schema: &schema, Policies: policies})
	if err != nil {
		t.Fatal(err)
	}
	verification := manifest.Verify(stored, current, stored.GeneratedAt)
	for _, name := range []string{"schema", "policy"} {
		found := false
		for _, check := range verification.Checks {
			want := "ok"
			if name == "schema" {
				want = "stale"
			}
			if check.Name == name && check.Status == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s fingerprint incompatible: %#v", name, verification.Checks)
		}
	}
	// Missing code/database fingerprints are not evidence of freshness. Do not
	// assert that their continued absence is correct or report them as verified.
	var serialized bytes.Buffer
	if err := manifest.WriteJSON(&serialized, stored); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "manifest.json")
	if err := os.WriteFile(path, serialized.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := manifest.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stored, loaded) {
		t.Fatal("known manifest fields changed through write/load")
	}
	// The preserved legacy fixture now requires regeneration and trusted tenant
	// supply. Input values do not establish application provenance.
	_, err = operation.Compile(context.Background(), spec, operation.Options{Manifest: loaded, Values: values})
	if !errors.Is(err, operation.ErrReservedBinding) {
		t.Fatal("legacy tenant claim accepted", err)
	}

	// Negative compatibility check against a changed supplied schema, not a live DB.
	schema.Tables[0].Columns[0].Type = "fixture_changed_type"
	changed, err := manifest.Generate(manifest.Options{Schema: &schema, Policies: policies})
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Verify(stored, changed, stored.GeneratedAt).Fresh {
		t.Fatal("changed schema accepted as fresh")
	}
}

func TestExternalTransactionOwnership(t *testing.T) {
	requireCase(t, "C27")
	for _, dialect := range []driver.Dialect{driver.MySQLDialect{}, driver.PostgresDialect{}} {
		raw, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = raw.Close() })
		mock.ExpectBegin()
		tx, err := raw.Begin()
		if err != nil {
			t.Fatal(err)
		}
		wrapped := orm.NewTxDB(tx, dialect)
		if wrapped.SQLDB() != nil {
			t.Fatal("external Tx exposed owned SQLDB")
		}
		if err := wrapped.Close(); err != nil {
			t.Fatal(err)
		}
		// Closing the wrapper must leave the caller-owned transaction usable.
		mock.ExpectExec("DELETE FROM users WHERE id = 10").WillReturnResult(sqlmock.NewResult(0, 1))
		result, err := wrapped.RequireRawApproval("fictional fixture cleanup").Exec("DELETE FROM users WHERE id = 10")
		if err != nil {
			t.Fatal(err)
		}
		if count, err := result.RowsAffected(); err != nil || count != 1 {
			t.Fatalf("delegated result: %d, %v", count, err)
		}
		mock.ExpectRollback()
		if err := tx.Rollback(); err != nil {
			t.Fatal(err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}
