package migration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/recoweft/goquent/orm/driver"
)

var preflightNow = time.Date(2026, 10, 18, 0, 0, 0, 0, time.UTC)

func preflightFixture(t *testing.T) *Preflight {
	t.Helper()
	ctx := &SchemaContext{Dialect: "postgres", Catalog: "testdb", Schema: "fixture", Scope: "tables", Tables: []string{"items"}, ServerVersion: "16.10", Visibility: "catalog_visible", VersionSource: "catalog_select", Coverage: SchemaCoverage{Identity: CoverageComplete, Version: CoverageComplete, Tables: CoveragePartial, Columns: CoveragePartial}}
	schema := Schema{Context: ctx, Tables: []TableSchema{{Name: "items", Identity: ObjectIdentity{"testdb", "fixture", "items", CoverageComplete}, Columns: []ColumnSchema{{Name: "a", Type: "integer", Nullable: true, NullableKnown: true}, {Name: "b", Type: "integer", Nullable: true, NullableKnown: true}}}}}
	nullable := false
	p := &MigrationPlan{CurrentSchema: &schema, DesiredSchema: copySchema(schema), Steps: []MigrationStep{{Type: AlterNullability, Table: "items", Column: "a", Nullable: &nullable}}}
	p.DesiredSchema.Tables[0].Columns[0].Nullable = false
	out, e := PlanPreflight(p, []PhaseContract{{Phase: "contract", OldReaders: "compatible", OldWriters: "compatible"}}, nil)
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func fixtureEvidence(t *testing.T) (*Preflight, *Collector) {
	p := preflightFixture(t)
	c := &Collector{}
	n := int64(0)
	p.Collection = &PreflightCollection{PlanID: p.Plan.ID, SchemaFingerprint: schemaFingerprint(p.Plan), FingerprintDomain: "goquent.preflight.schema.v1", StartedAt: preflightNow.Add(-time.Minute), FinishedAt: preflightNow, Method: "bounded_select", IdentityBasis: "dedicated_connection_session", Scope: "explicit_target_rows", Completed: true, Code: "PREFLIGHT_OK", Results: []PreflightResult{{RequirementID: p.Plan.Requirements[0].ID, Count: &n, Completed: true, Status: "satisfied", Code: "PREFLIGHT_OK"}}}
	p.Collection.Digest = collectionDigest(*p.Collection)
	p.receipt = &preflightReceipt{owner: c, digest: p.Collection.Digest, plan: p.Plan.ID, generation: c.generation}
	return p, c
}
func fixedPreflightOptions() PreflightOptions {
	return PreflightOptions{Clock: func() time.Time { return preflightNow }, MaxAge: PreflightMaxAge}
}
func TestPreflightReceiptAndFreshness(t *testing.T) {
	p, c := fixtureEvidence(t)
	v, e := c.Evaluate(p, fixedPreflightOptions())
	if e != nil || v.Status != "satisfied" {
		t.Fatalf("%v %v", v, e)
	}
	for _, name := range []string{"external", "closed", "other_session", "superseded", "count", "completed", "start", "target", "fingerprint", "plan", "kind", "missing_count", "unknown_result", "partial", "negative", "future", "expired", "reversed", "boundary", "bad_options"} {
		t.Run(name, func(t *testing.T) {
			p, c := fixtureEvidence(t)
			owner := c
			opts := fixedPreflightOptions()
			valid := false
			switch name {
			case "external":
				p, _ = detachPreflight(p)
			case "closed":
				c.closed = true
			case "other_session":
				owner = &Collector{}
			case "superseded":
				c.generation++
			case "count":
				*p.Collection.Results[0].Count = 1
			case "completed":
				p.Collection.Results[0].Completed = false
			case "start":
				p.Collection.StartedAt = p.Collection.StartedAt.Add(time.Second)
			case "target":
				p.Plan.Requirements[0].Target.Schema = "other"
			case "fingerprint":
				p.Collection.SchemaFingerprint = "sha256:bad"
			case "plan":
				p.Plan.SQL = "other"
			case "kind":
				p.Plan.Requirements[0].Kind = "duplicate_count"
			case "missing_count":
				p.Collection.Results[0].Count = nil
			case "unknown_result":
				p.Collection.Results[0].Status = "magic"
			case "partial":
				p.Collection.Completed = false
			case "negative":
				*p.Collection.Results[0].Count = -1
			case "future":
				opts.Clock = func() time.Time { return preflightNow.Add(-time.Second) }
			case "expired":
				opts.Clock = func() time.Time { return preflightNow.Add(4*time.Minute + time.Nanosecond) }
			case "reversed":
				p.Collection.StartedAt = preflightNow.Add(time.Second)
			case "boundary":
				opts.Clock = func() time.Time { return preflightNow.Add(4 * time.Minute) }
				valid = true
			case "bad_options":
				opts.MaxAge = 0
			}
			// Rehashing unauthenticated source cannot restore the original private receipt.
			if name == "count" || name == "start" {
				p.Collection.Digest = collectionDigest(*p.Collection)
			}
			v, _ := owner.Evaluate(p, opts)
			if (v.Status == "satisfied") != valid {
				t.Fatalf("%s: %v", name, v)
			}
		})
	}
}
func TestPreflightPlanBindingsAndDetachment(t *testing.T) {
	p := preflightFixture(t)
	if ValidatePreflightSource(p) != nil {
		t.Fatal("invalid baseline")
	}
	for _, edit := range []func(*Preflight){
		func(p *Preflight) { p.Plan.Current.Tables[0].Columns[0].NullableKnown = false },
		func(p *Preflight) { p.Plan.Current.Context.Coverage.Columns = CoverageComplete },
		func(p *Preflight) { p.Plan.Current.Context.ServerVersion = "16.11" },
		func(p *Preflight) { p.Plan.Current.Context.Schema = "other" },
		func(p *Preflight) { p.Plan.Steps[0].Phase.OldReaders = "incompatible" },
		func(p *Preflight) { p.Plan.Steps[0].Definition.SQL = "changed" },
		func(p *Preflight) { p.Plan.Steps[0].Phase.DependsOn = []int{0} },
		func(p *Preflight) { p.Plan.Requirements = nil },
	} {
		q, _ := detachPreflight(p)
		edit(q)
		if ValidatePreflightSource(q) == nil {
			t.Fatal("mutation accepted")
		}
	}
	q, _ := detachPreflight(p)
	q.Plan.Current.Tables[0].Columns[0].Name = "changed"
	if p.Plan.Current.Tables[0].Columns[0].Name != "a" {
		t.Fatal("alias")
	}
	b, _ := json.Marshal(p)
	var q2 Preflight
	if json.Unmarshal(b, &q2) != nil || ValidatePreflightSource(&q2) != nil {
		t.Fatal("roundtrip")
	}
	for _, raw := range []string{`{"version":1,"version":1}`, `{"version":null}`, `{"Version":1}`, `{"version":2}`} {
		var q Preflight
		e := json.Unmarshal([]byte(raw), &q)
		if e == nil && ValidatePreflightSource(&q) == nil {
			t.Fatal(raw)
		}
	}
}
func TestPreflightLegacyPhasesAndUnknown(t *testing.T) {
	p := preflightFixture(t)
	s := p.Plan.Steps[0].Definition
	s.Preflight = []string{"done"}
	old := &MigrationPlan{CurrentSchema: &p.Plan.Current, DesiredSchema: &p.Plan.Desired, Steps: []MigrationStep{s}}
	q, e := PlanPreflight(old, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(q.Plan.Requirements) < 2 {
		t.Fatal("legacy became phase evidence")
	}
	q.Plan.Steps[0].Phase.Phase = "invented"
	if buildPreflight(&q.Plan) != nil {
		t.Fatal("unknown enum must survive")
	}
	v, _ := EvaluatePreflight(q, fixedPreflightOptions())
	if v.Status != "unknown" {
		t.Fatal(v)
	}
	old.Steps = append(old.Steps, s)
	_, e = PlanPreflight(old, []PhaseContract{{Phase: "contract"}, {Phase: "expand"}}, nil)
	if e == nil {
		t.Fatal("phase order")
	}
	_, e = PlanPreflight(old, []PhaseContract{{Phase: "expand"}, {Phase: "contract", DependsOn: []int{0, 0}}}, nil)
	if e == nil {
		t.Fatal("duplicate dependency")
	}
	old.Steps[0].Type = AddForeignKey
	if EnsureExecutable(old) == nil {
		t.Fatal("structural refusal changed")
	}
}
func TestPreflightFiniteSQLAndCounts(t *testing.T) {
	for _, d := range []string{"mysql", "postgres"} {
		for _, kind := range []string{"null_count", "duplicate_count", "orphan_count"} {
			r := preflightFixture(t).Plan.Requirements[0]
			r.Kind = kind
			switch kind {
			case "null_count":
				r.Semantics = "null_rows"
			case "duplicate_count":
				r.Semantics = "duplicate_groups_nonnull"
				r.Columns = []string{"b", "a"}
			case "orphan_count":
				r.Semantics = "match_simple_nonnull_rows"
				r.Columns = []string{"b", "a"}
				r.Reference = ObjectIdentity{"testdb", "other", "parents", CoverageComplete}
				r.ReferencedColumns = []string{"x", "y"}
			}
			q, e := preflightSQL(d, r)
			if e != nil || !strings.HasPrefix(q, "SELECT COUNT(*)") || strings.Contains(q, "LIMIT") {
				t.Fatalf("%s %v", q, e)
			}
			r.Columns[0] = "a; DELETE"
			if _, e = preflightSQL(d, r); e == nil {
				t.Fatal("identifier injection")
			}
		}
	}
	for _, name := range []string{"zero", "positive", "scan", "null", "negative", "overflow", "rows_error", "query", "cancel", "timeout", "extra", "empty"} {
		t.Run(name, func(t *testing.T) {
			db, m, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			conn, e := db.Conn(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			defer conn.Close()
			c := &Collector{conn: conn, queryTimeout: time.Second}
			ctx := context.Background()
			rows := sqlmock.NewRows([]string{"n"})
			valid := false
			switch name {
			case "zero":
				rows.AddRow(0)
				valid = true
			case "positive":
				rows.AddRow(4)
				valid = true
			case "scan":
				rows.AddRow("bad")
			case "null":
				rows.AddRow(nil)
			case "negative":
				rows.AddRow(-1)
			case "overflow":
				rows.AddRow("9223372036854775808")
			case "rows_error":
				rows.AddRow(0).RowError(0, ErrPreflightRead)
			case "extra":
				rows.AddRow(0).AddRow(0)
			}
			if name == "cancel" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			} else if name == "query" {
				m.ExpectQuery("SELECT").WillReturnError(ErrPreflightRead)
			} else if name == "timeout" {
				c.queryTimeout = time.Millisecond
				m.ExpectQuery("SELECT").WillDelayFor(50 * time.Millisecond).WillReturnRows(rows)
			} else {
				m.ExpectQuery("SELECT").WillReturnRows(rows)
			}
			_, e = c.count(ctx, "SELECT COUNT(*)")
			if (e == nil) != valid {
				t.Fatalf("%s %v", name, e)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestPreflightPublicCanary(t *testing.T) {
	p, c := fixtureEvidence(t)
	p.Plan.SQL = "canary_secret"
	v, _ := c.Evaluate(p, fixedPreflightOptions())
	b, _ := json.Marshal(v)
	if strings.Contains(string(b), "canary") {
		t.Fatal(string(b))
	}
	v = PreflightView{Kind: "canary", Status: "canary", Code: "canary", Checks: []PreflightCheckView{{"canary", "canary"}}}
	if strings.Contains(v.String(), "canary") {
		t.Fatal(v)
	}
}
func TestPreflightCollectorRejectsCustomDriver(t *testing.T) {
	db, _, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = NewPreflightCollector(context.Background(), db, driver.PostgresDialect{}, CollectorOptions{Clock: func() time.Time { return preflightNow }}); e == nil {
		t.Fatal("custom driver trusted")
	}
}

func mockPreflightCatalog(m sqlmock.Sqlmock, changed bool) {
	rows := sqlmock.NewRows([]string{"table", "column", "type", "nullable", "default"}).AddRow("items", "a", "integer", "YES", nil).AddRow("items", "b", "integer", "YES", nil)
	if changed {
		rows.AddRow("items", "c", "integer", "YES", nil)
	}
	m.ExpectQuery("information_schema.columns").WillReturnRows(rows)
	m.ExpectQuery("current_database").WillReturnRows(sqlmock.NewRows([]string{"catalog", "schema", "version"}).AddRow("testdb", "fixture", "16.10"))
	m.ExpectQuery("FROM pg_constraint").WillReturnRows(sqlmock.NewRows(pgConstraintColumns))
	m.ExpectQuery("JOIN pg_index").WillReturnRows(sqlmock.NewRows(pgIndexColumns))
}
func mockPreflightShape(m sqlmock.Sqlmock, failure string) {
	e := m.ExpectQuery("SELECT a.attname")
	if failure == "query" {
		e.WillReturnError(ErrPreflightRead)
		return
	}
	rows := sqlmock.NewRows([]string{"column", "type", "kind", "rls", "inherit", "priv", "collation", "generated", "namespace"}).AddRow("a", "int4", "r", false, false, true, 0, "", "pg_catalog").AddRow("b", "int4", "r", false, false, true, 0, "", "pg_catalog")
	if failure == "scan" {
		rows = sqlmock.NewRows([]string{"wrong"}).AddRow(1)
	}
	if failure == "rows" {
		rows.RowError(1, ErrPreflightRead)
	}
	e.WillReturnRows(rows)
}
func TestPreflightCollectionPartialAndSchemaChange(t *testing.T) {
	for _, failure := range []string{"second_query", "shape_query", "shape_scan", "shape_rows", "after_schema", "after_error"} {
		t.Run(failure, func(t *testing.T) {
			db, m, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			mockPreflightCatalog(m, false)
			schema, e := ReadSchema(context.Background(), db, driver.PostgresDialect{}, WithSchemaReadSchema("fixture"), WithSchemaReadTables("items"))
			if e != nil {
				t.Fatal(e)
			}
			desired := copySchema(schema)
			desired.Tables[0].Columns[0].Nullable = false
			n := false
			step := MigrationStep{Type: AlterNullability, Table: "items", Column: "a", Nullable: &n}
			phases := []PhaseContract{{Phase: "contract", OldReaders: "compatible", OldWriters: "compatible"}}
			steps := []MigrationStep{step}
			if failure == "second_query" {
				steps = append(steps, step)
				phases = append(phases, PhaseContract{Phase: "contract", OldReaders: "compatible", OldWriters: "compatible", DependsOn: []int{0}})
			}
			plan, e := PlanPreflight(&MigrationPlan{CurrentSchema: &schema, DesiredSchema: desired, Steps: steps}, phases, nil)
			if e != nil {
				t.Fatal(e)
			}
			conn, e := db.Conn(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			c := &Collector{conn: conn, dialect: driver.PostgresDialect{}, name: "postgres", clock: func() time.Time { return preflightNow }, queryTimeout: time.Second, totalTimeout: 5 * time.Second}
			defer c.Close()
			mockPreflightCatalog(m, false)
			if strings.HasPrefix(failure, "shape_") {
				mockPreflightShape(m, strings.TrimPrefix(failure, "shape_"))
			} else {
				mockPreflightShape(m, "")
				m.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
				if failure == "second_query" {
					mockPreflightShape(m, "")
					m.ExpectQuery("SELECT COUNT").WillReturnError(ErrPreflightRead)
				} else {
					mockPreflightShape(m, "")
					if failure == "after_error" {
						m.ExpectQuery("information_schema.columns").WillReturnError(ErrPreflightRead)
					} else {
						mockPreflightCatalog(m, true)
					}
				}
			}
			out, e := c.Collect(context.Background(), plan)
			if e == nil || out == nil || out.Collection.Completed {
				t.Fatal("partial became complete")
			}
			if failure == "second_query" && (len(out.Collection.Results) != 2 || !out.Collection.Results[0].Completed) {
				t.Fatal("partial source lost")
			}
			v, e := c.Evaluate(out, fixedPreflightOptions())
			if e == nil || v.Status == "satisfied" {
				t.Fatal("failure weakened")
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestPreflightDecodeInvalidatesReceipt(t *testing.T) {
	p, c := fixtureEvidence(t)
	if e := p.UnmarshalJSON([]byte(`{"version":null}`)); e == nil {
		t.Fatal("null accepted")
	}
	v, _ := c.Evaluate(p, fixedPreflightOptions())
	if v.Status == "satisfied" {
		t.Fatal("decode retained receipt")
	}
	for _, raw := range []string{`{"kind":"goquent.preflight_view","version":1,"status":"unknown","code":"PREFLIGHT_MISSING","checks":[],"extra":1}`, `{"kind":"goquent.preflight_view","version":1,"status":"secret","code":"PREFLIGHT_MISSING","checks":[]}`} {
		var v PreflightView
		if json.Unmarshal([]byte(raw), &v) == nil {
			t.Fatal("public decoder accepted unknown")
		}
	}
}
func TestPreflightTargetShapeLimits(t *testing.T) {
	for _, tc := range []struct {
		name, typ, kind      string
		rls, inherit, priv   bool
		coll                 int
		generated, namespace string
		want                 string
	}{
		{"plain", "int4", "r", false, false, true, 0, "", "pg_catalog", "satisfied"},
		{"rls", "int4", "r", true, false, true, 0, "", "pg_catalog", "unknown"},
		{"privilege", "int4", "r", false, false, false, 0, "", "pg_catalog", "unknown"},
		{"partition", "int4", "p", false, false, true, 0, "", "pg_catalog", "unsupported"},
		{"inheritance", "int4", "r", false, true, true, 0, "", "pg_catalog", "unsupported"},
		{"view", "int4", "v", false, false, true, 0, "", "pg_catalog", "unsupported"},
		{"domain", "private_type", "r", false, false, true, 0, "", "private", "unsupported"},
		{"collation", "text", "r", false, false, true, 100, "", "pg_catalog", "unsupported"},
		{"generated", "int4", "r", false, false, true, 0, "s", "pg_catalog", "unsupported"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, m, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			conn, e := db.Conn(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			defer conn.Close()
			m.ExpectQuery("SELECT a.attname").WillReturnRows(sqlmock.NewRows([]string{"column", "type", "kind", "rls", "inherit", "priv", "collation", "generated", "namespace"}).AddRow("a", tc.typ, tc.kind, tc.rls, tc.inherit, tc.priv, tc.coll, tc.generated, tc.namespace))
			c := &Collector{conn: conn, name: "postgres", queryTimeout: time.Second}
			p := preflightFixture(t)
			_, status, _, e := c.checkTarget(context.Background(), p.Plan, p.Plan.Requirements[0])
			if e != nil || status != tc.want {
				t.Fatalf("%s %v", status, e)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
