package review

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/faciam-dev/goquent/orm/manifest"
	"github.com/faciam-dev/goquent/orm/migration"
	"github.com/faciam-dev/goquent/orm/query"
)

func TestRunReviewsRawSQLAndQueryPlanJSON(t *testing.T) {
	dir := t.TempDir()
	sqlPath := filepath.Join(dir, "danger.sql")
	if err := os.WriteFile(sqlPath, []byte("DROP TABLE users;"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan := query.NewRawPlan("DELETE FROM users WHERE id = ?", 10)
	planJSON, err := plan.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(planPath, planJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}

	if !hasFinding(report.Findings, query.WarningRawSQLUsed) {
		t.Fatalf("expected %s finding, got %#v", query.WarningRawSQLUsed, report.Findings)
	}
	if !hasFinding(report.Findings, migration.WarningMigrationDropTable) {
		t.Fatalf("expected %s finding, got %#v", migration.WarningMigrationDropTable, report.Findings)
	}
	if !HasFindingsAtOrAbove(report, query.RiskHigh) {
		t.Fatalf("expected high threshold failure")
	}
}

func TestRunReviewsMigrationPlanJSON(t *testing.T) {
	dir := t.TempDir()
	plan, err := migration.PlanSQL("ALTER TABLE users DROP COLUMN legacy_id;")
	if err != nil {
		t.Fatal(err)
	}
	planJSON, err := plan.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(dir, "migration.json")
	if err := os.WriteFile(planPath, planJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{planPath}})
	if err != nil {
		t.Fatal(err)
	}
	finding, ok := findFinding(report.Findings, migration.WarningMigrationDropColumn)
	if !ok {
		t.Fatalf("expected migration drop column finding, got %#v", report.Findings)
	}
	if finding.Location == nil || finding.Location.File != planPath || finding.Location.Line != 1 {
		t.Fatalf("expected migration finding location from plan JSON, got %#v", finding.Location)
	}
}

func TestRunReviewsStaleManifest(t *testing.T) {
	dir := t.TempDir()
	m, err := manifest.Generate(manifest.Options{
		GeneratedAt: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
		Schema: &migration.Schema{Tables: []migration.TableSchema{{
			Name:    "users",
			Columns: []migration.ColumnSchema{{Name: "id", Type: "bigint"}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	current, err := manifest.Generate(manifest.Options{
		GeneratedAt: time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC),
		Schema: &migration.Schema{Tables: []migration.TableSchema{{
			Name:    "users",
			Columns: []migration.ColumnSchema{{Name: "id", Type: "uuid"}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	verification := manifest.Verify(m, current, time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC))
	m = manifest.AttachVerification(m, verification)
	b, err := m.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "goquent.manifest.json")
	if err := os.WriteFile(manifestPath, b, 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{ManifestPath: manifestPath})
	if err != nil {
		t.Fatal(err)
	}
	if report.ManifestStatus == nil || report.ManifestStatus.Fresh {
		t.Fatalf("expected stale manifest status, got %#v", report.ManifestStatus)
	}
	if !hasFinding(report.Findings, manifest.WarningStale) {
		t.Fatalf("expected manifest stale finding, got %#v", report.Findings)
	}
}

func TestRunRequiresFreshManifestRejectsMissingAndUnverified(t *testing.T) {
	dir := t.TempDir()
	report, err := Run(Options{Paths: []string{dir}, RequireFreshManifest: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.ManifestStatus == nil || report.ManifestStatus.State != "missing" {
		t.Fatalf("expected missing manifest status, got %#v", report.ManifestStatus)
	}
	if !hasFinding(report.Findings, manifest.WarningRequired) {
		t.Fatalf("expected %s finding, got %#v", manifest.WarningRequired, report.Findings)
	}

	m, err := manifest.Generate(manifest.Options{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := m.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "goquent.manifest.json")
	if err := os.WriteFile(manifestPath, b, 0o644); err != nil {
		t.Fatal(err)
	}
	report, err = Run(Options{ManifestPath: manifestPath, RequireFreshManifest: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.ManifestStatus == nil || report.ManifestStatus.State != "unverified" {
		t.Fatalf("expected unverified manifest status, got %#v", report.ManifestStatus)
	}
	if !hasFinding(report.Findings, manifest.WarningUnverified) {
		t.Fatalf("expected %s finding, got %#v", manifest.WarningUnverified, report.Findings)
	}
}

func TestRunVerifiesManifestAgainstCurrentInputs(t *testing.T) {
	dir := t.TempDir()
	stored, err := manifest.Generate(manifest.Options{
		Schema: &migration.Schema{Tables: []migration.TableSchema{{
			Name:    "users",
			Columns: []migration.ColumnSchema{{Name: "id", Type: "bigint"}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	current, err := manifest.Generate(manifest.Options{
		Schema: &migration.Schema{Tables: []migration.TableSchema{{
			Name:    "users",
			Columns: []migration.ColumnSchema{{Name: "id", Type: "bigint"}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := stored.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "goquent.manifest.json")
	if err := os.WriteFile(manifestPath, b, 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{
		ManifestPath:         manifestPath,
		RequireFreshManifest: true,
		CurrentManifest:      current,
		ManifestInputs:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.ManifestStatus == nil || report.ManifestStatus.State != "fresh" || !report.ManifestStatus.Verified {
		t.Fatalf("expected verified fresh manifest status, got %#v", report.ManifestStatus)
	}
	if hasFinding(report.Findings, manifest.WarningUnverified) || hasFinding(report.Findings, manifest.WarningStale) {
		t.Fatalf("unexpected freshness finding: %#v", report.Findings)
	}
}

func TestRunReviewsSuppressedWarningsFromQueryPlanJSON(t *testing.T) {
	dir := t.TempDir()
	plan := query.QueryPlan{
		Operation: query.OperationInsert,
		SQL:       "INSERT INTO users (id) VALUES (?)",
		SuppressedWarnings: []query.Warning{{
			Code:    query.WarningLimitMissing,
			Level:   query.RiskMedium,
			Message: "SELECT query has no LIMIT",
		}},
	}
	planJSON, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(dir, "plan.json")
	if err := os.WriteFile(planPath, planJSON, 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Suppressed != 1 {
		t.Fatalf("expected one suppressed finding, got %d", report.Summary.Suppressed)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("expected suppressed finding to be hidden by default, got %#v", report.Findings)
	}
}

func TestDiscoverFilesSupportsEllipsis(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	goPath := filepath.Join(dir, "nested", "repo.go")
	if err := os.WriteFile(goPath, []byte("package nested\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := discoverFiles(dir + "/...")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != goPath {
		t.Fatalf("expected ellipsis discovery to find %s, got %#v", goPath, files)
	}
}

func TestRunReviewsGoSourcePreciseAndSuppressed(t *testing.T) {
	dir := t.TempDir()
	goPath := filepath.Join(dir, "repo.go")
	src := `package sample

func run(db any) {
	var rows []map[string]any
	db.Table("users").Update(map[string]any{"name": "x"})
	// goquent:suppress LIMIT_MISSING reason="batch report is intentionally unbounded"
	db.Table("users").Select("id").GetMaps(&rows)
}
`
	if err := os.WriteFile(goPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{goPath}})
	if err != nil {
		t.Fatal(err)
	}

	update, ok := findFinding(report.Findings, query.WarningUpdateWithoutWhere)
	if !ok {
		t.Fatalf("expected %s finding, got %#v", query.WarningUpdateWithoutWhere, report.Findings)
	}
	if update.AnalysisPrecision != query.AnalysisPrecise {
		t.Fatalf("expected precise update finding, got %s", update.AnalysisPrecision)
	}
	if update.Location == nil || update.Location.Line != 5 {
		t.Fatalf("expected update location line 5, got %#v", update.Location)
	}
	if report.Summary.Suppressed != 1 {
		t.Fatalf("expected one suppressed finding, got %d", report.Summary.Suppressed)
	}
	if !hasFinding(report.SuppressedFindings, query.WarningLimitMissing) {
		t.Fatalf("expected suppressed %s finding, got %#v", query.WarningLimitMissing, report.SuppressedFindings)
	}

	report, err = Run(Options{Paths: []string{goPath}, ShowSuppressed: true})
	if err != nil {
		t.Fatal(err)
	}
	if !hasSuppressedFinding(report.Findings, query.WarningLimitMissing) {
		t.Fatalf("expected suppressed finding in primary output, got %#v", report.Findings)
	}
}

func TestRunReviewsGoSourcePartialAndUnsupported(t *testing.T) {
	dir := t.TempDir()
	goPath := filepath.Join(dir, "dynamic.go")
	src := `package sample

func run(q any, db any, sqlText string) {
	q.Update(map[string]any{"name": "x"})
	db.Exec(sqlText)
}
`
	if err := os.WriteFile(goPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{goPath}})
	if err != nil {
		t.Fatal(err)
	}

	partial, ok := findFinding(report.Findings, query.WarningStaticReviewPartial)
	if !ok {
		t.Fatalf("expected %s finding, got %#v", query.WarningStaticReviewPartial, report.Findings)
	}
	if partial.AnalysisPrecision != query.AnalysisPartial {
		t.Fatalf("expected partial precision, got %s", partial.AnalysisPrecision)
	}
	unsupported, ok := findFinding(report.Findings, query.WarningStaticReviewUnsupported)
	if !ok {
		t.Fatalf("expected %s finding, got %#v", query.WarningStaticReviewUnsupported, report.Findings)
	}
	if unsupported.AnalysisPrecision != query.AnalysisUnsupported {
		t.Fatalf("expected unsupported precision, got %s", unsupported.AnalysisPrecision)
	}
}

func TestRunReviewsGoSourceWithManifestPolicies(t *testing.T) {
	dir := t.TempDir()
	goPath := filepath.Join(dir, "repo.go")
	src := `package sample

func run(db any) {
	var rows []map[string]any
	db.Table("users").Select("id", "email").GetMaps(&rows)
	db.Table("users").Where("tenant_id", "tenant-1").Update(map[string]any{"name": "x"})
}
`
	if err := os.WriteFile(goPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &manifest.Manifest{Tables: []manifest.Table{{
		Name:  "users",
		Model: "User",
		Columns: []manifest.Column{
			{Name: "id", Primary: true},
			{Name: "tenant_id", TenantScope: true, RequiredFilter: true},
			{Name: "deleted_at", SoftDelete: true},
			{Name: "email", PII: true},
			{Name: "name"},
		},
		Policies: []manifest.Policy{
			{Type: "tenant_scope", Column: "tenant_id", Mode: query.PolicyModeEnforce},
			{Type: "soft_delete", Column: "deleted_at", Mode: query.PolicyModeEnforce},
			{Type: "pii", Column: "email", Mode: query.PolicyModeWarn},
		},
	}}}

	report, err := Run(Options{Paths: []string{goPath}, CurrentManifest: m, ManifestInputs: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{
		query.WarningTenantFilterMissing,
		query.WarningSoftDeleteFilterMissing,
		query.WarningPIIColumnSelected,
		query.WarningBulkUpdateDetected,
	} {
		if !hasFinding(report.Findings, code) {
			t.Fatalf("expected %s finding, got %#v", code, report.Findings)
		}
	}
}

func TestWriteJSONAndPretty(t *testing.T) {
	report := ReviewReport{
		Findings: []Finding{{
			Code:              query.WarningRawSQLUsed,
			Level:             query.RiskHigh,
			Message:           "raw SQL was used",
			AnalysisPrecision: query.AnalysisPrecise,
		}},
	}
	report.Summary = summarize(report)

	var jsonBuf bytes.Buffer
	if err := WriteJSON(&jsonBuf, report); err != nil {
		t.Fatal(err)
	}
	var decoded ReviewReport
	if err := json.Unmarshal(jsonBuf.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Summary.HighestRisk != query.RiskHigh {
		t.Fatalf("expected high summary risk, got %s", decoded.Summary.HighestRisk)
	}

	var pretty bytes.Buffer
	if err := WritePretty(&pretty, report); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(pretty.Bytes(), []byte("Database Review")) {
		t.Fatalf("expected pretty output header, got %s", pretty.String())
	}
}

func hasFinding(findings []Finding, code string) bool {
	_, ok := findFinding(findings, code)
	return ok
}

func hasSuppressedFinding(findings []Finding, code string) bool {
	for _, finding := range findings {
		if finding.Code == code && finding.Suppressed {
			return true
		}
	}
	return false
}

func findFinding(findings []Finding, code string) (Finding, bool) {
	for _, finding := range findings {
		if finding.Code == code {
			return finding, true
		}
	}
	return Finding{}, false
}
