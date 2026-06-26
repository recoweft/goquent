package review

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/migration"
	"github.com/recoweft/goquent/orm/query"
)

// Finding is a review finding emitted by goquent review.
type Finding struct {
	Code              string                  `json:"code"`
	Level             query.RiskLevel         `json:"level"`
	Message           string                  `json:"message"`
	Location          *query.SourceLocation   `json:"location,omitempty"`
	Hint              string                  `json:"hint,omitempty"`
	Evidence          []query.Evidence        `json:"evidence,omitempty"`
	AnalysisPrecision query.AnalysisPrecision `json:"analysis_precision"`
	Suppressed        bool                    `json:"suppressed"`
	Suppression       *query.Suppression      `json:"suppression,omitempty"`
}

// ReviewSummary aggregates findings for machine-readable output and CI.
type ReviewSummary struct {
	Total       int                     `json:"total"`
	Suppressed  int                     `json:"suppressed"`
	ByLevel     map[query.RiskLevel]int `json:"by_level,omitempty"`
	HighestRisk query.RiskLevel         `json:"highest_risk"`
}

// ManifestStatus is reserved for Phase 6 stale manifest integration.
type ManifestStatus struct {
	Fresh    bool   `json:"fresh"`
	Verified bool   `json:"verified,omitempty"`
	State    string `json:"state,omitempty"`
	Path     string `json:"path,omitempty"`
}

// ReviewReport is the top-level report produced by goquent review.
type ReviewReport struct {
	Findings           []Finding       `json:"findings"`
	SuppressedFindings []Finding       `json:"suppressed_findings,omitempty"`
	Summary            ReviewSummary   `json:"summary"`
	ManifestStatus     *ManifestStatus `json:"manifest_status,omitempty"`
}

// Options controls review discovery and output behavior.
type Options struct {
	Paths                []string
	ShowSuppressed       bool
	ConfigPath           string
	ManifestPath         string
	RequireFreshManifest bool
	CurrentManifest      *manifest.Manifest
	ManifestInputs       bool
	Rules                map[string]query.RiskRuleConfig
	ConfigSuppressions   []ConfigSuppression
}

// Run reviews all configured paths.
func Run(opts Options) (ReviewReport, error) {
	paths := opts.Paths
	if len(paths) == 0 {
		paths = []string{"."}
	}
	ctx := newReviewContext(opts)

	var report ReviewReport
	var errs []error
	var reviewedFiles []string
	if strings.TrimSpace(opts.ManifestPath) != "" || opts.RequireFreshManifest {
		findings, status, err := reviewManifestFreshness(opts)
		if err != nil {
			errs = append(errs, err)
		}
		if status != nil {
			report.ManifestStatus = status
		}
		report.Findings = append(report.Findings, findings...)
	}
	for _, path := range paths {
		files, err := discoverFiles(path)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		reviewedFiles = append(reviewedFiles, files...)
		for _, file := range files {
			findings, err := reviewFile(ctx, file)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			for _, finding := range findings {
				if finding.Suppressed {
					report.SuppressedFindings = append(report.SuppressedFindings, finding)
					if opts.ShowSuppressed {
						report.Findings = append(report.Findings, finding)
					}
					continue
				}
				report.Findings = append(report.Findings, finding)
			}
		}
	}
	report.Findings = append(report.Findings, lintConfigSuppressions(opts.ConfigPath, opts.ConfigSuppressions, reviewedFiles, report.SuppressedFindings)...)
	report.Summary = summarize(report)
	return report, errors.Join(errs...)
}

func reviewManifestFreshness(opts Options) ([]Finding, *ManifestStatus, error) {
	path := strings.TrimSpace(opts.ManifestPath)
	if path == "" {
		if !opts.RequireFreshManifest {
			return nil, nil, nil
		}
		status := &ManifestStatus{Fresh: false, State: "missing"}
		return []Finding{manifestFinding(
			manifest.WarningRequired,
			query.RiskHigh,
			"fresh manifest is required but no manifest path was provided",
			"pass --manifest and current schema, policy, code, or database fingerprint inputs",
			&query.SourceLocation{Line: 1},
			nil,
		)}, status, nil
	}
	m, err := manifest.Load(path)
	if err != nil {
		return nil, nil, err
	}

	if opts.CurrentManifest != nil {
		verification := manifest.Verify(m, opts.CurrentManifest, time.Time{})
		status := &ManifestStatus{Fresh: verification.Fresh, Verified: true, Path: path}
		if verification.Fresh {
			status.State = "fresh"
			return nil, status, nil
		}
		status.State = "stale"
		return []Finding{staleManifestFinding(path, verification)}, status, nil
	}

	if opts.RequireFreshManifest && !opts.ManifestInputs {
		status := &ManifestStatus{Fresh: false, State: "unverified", Path: path}
		return []Finding{manifestFinding(
			manifest.WarningUnverified,
			query.RiskHigh,
			"manifest freshness could not be verified against current inputs",
			"pass --schema, --policy, --code, or --database-schema with --require-fresh-manifest",
			&query.SourceLocation{File: path, Line: 1},
			nil,
		)}, status, nil
	}

	status := &ManifestStatus{Fresh: true, Path: path}
	if m.Verification != nil {
		status.Fresh = m.Verification.Fresh
		status.Verified = true
	}
	if m.Verification == nil {
		status.State = "unverified"
		return nil, status, nil
	}
	if m.Verification.Fresh {
		status.State = "fresh"
		return nil, status, nil
	}
	status.State = "stale"
	return []Finding{staleManifestFinding(path, *m.Verification)}, status, nil
}

func staleManifestFinding(path string, verification manifest.Verification) Finding {
	var evidence []query.Evidence
	for _, check := range verification.Checks {
		if check.Status == "stale" {
			evidence = append(evidence, query.Evidence{Key: check.Name, Value: check.Message})
		}
	}
	return manifestFinding(
		manifest.WarningStale,
		query.RiskHigh,
		"manifest does not match current schema, policy, generated code, or database fingerprint",
		"regenerate the manifest or run goquent manifest verify against current inputs",
		&query.SourceLocation{File: path, Line: 1},
		evidence,
	)
}

func manifestFinding(code string, level query.RiskLevel, message, hint string, loc *query.SourceLocation, evidence []query.Evidence) Finding {
	return Finding{
		Code:              code,
		Level:             level,
		Message:           message,
		Location:          loc,
		Hint:              hint,
		Evidence:          evidence,
		AnalysisPrecision: query.AnalysisPrecise,
	}
}

func discoverFiles(root string) ([]string, error) {
	root = expandEllipsis(root)
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if supportedFile(root) {
			return []string{root}, nil
		}
		return nil, nil
	}

	var files []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".codex", ".gocache", "vendor", "node_modules", "dist", "build", "coverage":
				return filepath.SkipDir
			}
			return nil
		}
		if supportedFile(path) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func expandEllipsis(path string) string {
	path = strings.TrimSpace(path)
	switch path {
	case "", "...", "./...":
		return "."
	}
	if strings.HasSuffix(path, "/...") {
		root := strings.TrimSuffix(path, "/...")
		if root == "" {
			return "."
		}
		return root
	}
	return path
}

func supportedFile(path string) bool {
	switch filepath.Ext(path) {
	case ".go", ".sql", ".json":
		return true
	default:
		return false
	}
}

func reviewFile(ctx reviewContext, path string) ([]Finding, error) {
	switch filepath.Ext(path) {
	case ".go":
		return reviewGoFile(ctx, path)
	case ".sql":
		return reviewSQLFile(ctx, path)
	case ".json":
		return reviewPlanJSONFile(ctx, path)
	default:
		return nil, nil
	}
}

func reviewSQLFile(ctx reviewContext, path string) ([]Finding, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	sqlText := string(b)
	if looksLikeMigrationSQL(sqlText) {
		if plan, err := migration.PlanSQL(sqlText); err != nil {
			return nil, err
		} else if len(plan.Steps) > 0 {
			findings := warningsToFindings(plan.Warnings, plan.AnalysisPrecision, &query.SourceLocation{File: path, Line: 1})
			return applyFileSuppressions(ctx, path, findings)
		}
	}
	plan := query.NewRawPlan(sqlText)
	findings := findingsFromPlan(plan, query.AnalysisPrecise, &query.SourceLocation{File: path, Line: 1})
	return applyFileSuppressions(ctx, path, findings)
}

func looksLikeMigrationSQL(sqlText string) bool {
	upper := strings.ToUpper(sqlText)
	for _, token := range []string{"CREATE", "ALTER", "DROP", "RENAME", "GRANT", "REVOKE", "TRUNCATE"} {
		if containsSQLWord(upper, token) {
			return true
		}
	}
	return false
}

func reviewPlanJSONFile(ctx reviewContext, path string) ([]Finding, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var plan query.QueryPlan
	if err := json.Unmarshal(b, &plan); err != nil {
		return nil, nil
	}
	if plan.Operation == "" || plan.SQL == "" {
		migrationFindings, ok, err := reviewMigrationPlanJSON(path, b)
		if err != nil || ok {
			if err != nil {
				return nil, err
			}
			return applyFileSuppressions(ctx, path, migrationFindings)
		}
		return nil, nil
	}
	if len(ctx.riskMetadata) > 0 {
		query.AttachTableRiskMetadata(&plan, ctx.riskMetadata)
	}
	result := query.DefaultRiskEngine.CheckQuery(&plan)
	warnings := plan.Warnings
	if len(warnings) == 0 {
		warnings = result.Warnings
	}
	findings := findingsFromPlanWarnings(&plan, warnings, query.AnalysisPrecise, &query.SourceLocation{File: path, Line: 1})
	for _, finding := range warningsToFindings(plan.SuppressedWarnings, query.AnalysisPrecise, &query.SourceLocation{File: path, Line: 1}) {
		finding.Suppressed = true
		findings = append(findings, finding)
	}
	return applyFileSuppressions(ctx, path, findings)
}

func reviewMigrationPlanJSON(path string, b []byte) ([]Finding, bool, error) {
	var plan migration.MigrationPlan
	if err := json.Unmarshal(b, &plan); err != nil {
		return nil, false, nil
	}
	if plan.SQL == "" && len(plan.Steps) == 0 && len(plan.Statements) == 0 {
		return nil, false, nil
	}
	if len(plan.Warnings) == 0 && plan.SQL != "" {
		planned, err := migration.PlanSQL(plan.SQL)
		if err != nil {
			return nil, true, err
		}
		plan.Warnings = planned.Warnings
		plan.AnalysisPrecision = planned.AnalysisPrecision
	}
	findings := warningsToFindings(plan.Warnings, plan.AnalysisPrecision, &query.SourceLocation{File: path, Line: 1})
	return findings, true, nil
}

func findingsFromPlan(plan *query.QueryPlan, precision query.AnalysisPrecision, loc *query.SourceLocation) []Finding {
	if plan == nil {
		return nil
	}
	return findingsFromPlanWarnings(plan, plan.Warnings, precision, loc)
}

func findingsFromPlanWarnings(plan *query.QueryPlan, warnings []query.Warning, precision query.AnalysisPrecision, loc *query.SourceLocation) []Finding {
	findings := warningsToFindings(warnings, precision, loc)
	if len(findings) == 0 || plan == nil {
		return findings
	}
	evidence := planEvidence(plan)
	if len(evidence) == 0 {
		return findings
	}
	for i := range findings {
		if findings[i].Code == query.WarningRawSQLUsed || plan.Operation == query.OperationRaw {
			findings[i].Evidence = append(findings[i].Evidence, evidence...)
		}
	}
	return findings
}

func planEvidence(plan *query.QueryPlan) []query.Evidence {
	var evidence []query.Evidence
	if plan.Approval != nil && strings.TrimSpace(plan.Approval.Reason) != "" {
		evidence = append(evidence, query.Evidence{Key: "approval_reason", Value: plan.Approval.Reason})
	}
	var tables []string
	for _, table := range plan.Tables {
		name := strings.TrimSpace(table.Name)
		if name == "" {
			continue
		}
		tables = append(tables, name)
	}
	if len(tables) > 0 {
		evidence = append(evidence, query.Evidence{Key: "touched_tables", Value: tables})
	}
	return evidence
}

func warningsToFindings(warnings []query.Warning, precision query.AnalysisPrecision, loc *query.SourceLocation) []Finding {
	findings := make([]Finding, 0, len(warnings))
	for _, warning := range warnings {
		findingLoc := loc
		if warning.Location != nil {
			copied := *warning.Location
			if copied.File == "" && loc != nil {
				copied.File = loc.File
			}
			findingLoc = &copied
		}
		findings = append(findings, Finding{
			Code:              warning.Code,
			Level:             warning.Level,
			Message:           warning.Message,
			Location:          cloneLocation(findingLoc),
			Hint:              warning.Hint,
			Evidence:          append([]query.Evidence(nil), warning.Evidence...),
			AnalysisPrecision: precision,
		})
	}
	return findings
}

func cloneLocation(loc *query.SourceLocation) *query.SourceLocation {
	if loc == nil {
		return nil
	}
	copied := *loc
	return &copied
}

func applyFileSuppressions(ctx reviewContext, path string, findings []Finding) ([]Finding, error) {
	findings = applyReviewRules(findings, ctx.rules)
	if len(findings) == 0 {
		return findings, nil
	}
	suppressions, err := suppressionsForFile(path)
	if err != nil {
		return nil, err
	}
	configSuppressions, err := configSuppressionsForFile(path, ctx.configSuppressions)
	if err != nil {
		return nil, err
	}
	suppressions = append(suppressions, configSuppressions...)
	if len(suppressions) == 0 {
		return findings, nil
	}

	var out []Finding
	now := time.Now().UTC()
	for _, finding := range findings {
		suppression, ok := findSuppressionForFinding(finding, suppressions)
		if !ok {
			out = append(out, finding)
			continue
		}
		if suppression.ExpiresAt != nil && !suppression.ExpiresAt.After(now) {
			out = append(out, finding)
			out = append(out, suppressionFinding(query.WarningSuppressionExpired, "suppression has expired", finding.Location))
			continue
		}
		if !findingSuppressible(finding.Code) {
			out = append(out, finding)
			out = append(out, suppressionFinding(query.WarningSuppressionNotAllowed, "finding is not suppressible", finding.Location))
			continue
		}
		finding.Suppressed = true
		finding.Suppression = &suppression
		out = append(out, finding)
	}
	return out, nil
}

func suppressionsForFile(path string) ([]query.Suppression, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(b), "\n")
	var suppressions []query.Suppression
	for i, line := range lines {
		suppression, ok, err := query.ParseInlineSuppression(line)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		lineNo := i + 1
		suppression.Location = &query.SourceLocation{File: path, Line: lineNo}
		suppressions = append(suppressions, suppression)
	}
	return suppressions, nil
}

func findSuppressionForFinding(finding Finding, suppressions []query.Suppression) (query.Suppression, bool) {
	for _, suppression := range suppressions {
		if suppression.Code != finding.Code {
			continue
		}
		if finding.Location == nil || suppression.Location == nil {
			return suppression, true
		}
		if suppression.Location.Line == finding.Location.Line || suppression.Location.Line+1 == finding.Location.Line {
			return suppression, true
		}
	}
	return query.Suppression{}, false
}

func findingSuppressible(code string) bool {
	switch code {
	case query.WarningUpdateWithoutWhere, query.WarningDeleteWithoutWhere, query.WarningDestructiveSQL,
		migration.WarningMigrationDropTable, migration.WarningMigrationDropColumn, migration.WarningMigrationTypeNarrowing,
		manifest.WarningStale, manifest.WarningUnverified, manifest.WarningRequired:
		return false
	default:
		return true
	}
}

func suppressionFinding(code, message string, loc *query.SourceLocation) Finding {
	return Finding{
		Code:              code,
		Level:             query.RiskMedium,
		Message:           message,
		Location:          cloneLocation(loc),
		AnalysisPrecision: query.AnalysisPrecise,
	}
}

func summarize(report ReviewReport) ReviewSummary {
	summary := ReviewSummary{
		ByLevel:     make(map[query.RiskLevel]int),
		HighestRisk: query.RiskLow,
		Suppressed:  len(report.SuppressedFindings),
	}
	for _, finding := range report.Findings {
		if finding.Suppressed {
			continue
		}
		summary.Total++
		summary.ByLevel[finding.Level]++
		if compareRisk(finding.Level, summary.HighestRisk) > 0 {
			summary.HighestRisk = finding.Level
		}
	}
	return summary
}

// HasFindingsAtOrAbove reports whether report should fail CI at threshold.
func HasFindingsAtOrAbove(report ReviewReport, threshold query.RiskLevel) bool {
	for _, finding := range report.Findings {
		if finding.Suppressed {
			continue
		}
		if compareRisk(finding.Level, threshold) >= 0 {
			return true
		}
	}
	return false
}

// HasFindingsAtOrAbovePrecision reports whether findings meet a precision threshold.
func HasFindingsAtOrAbovePrecision(report ReviewReport, threshold query.AnalysisPrecision) bool {
	for _, finding := range report.Findings {
		if finding.Suppressed {
			continue
		}
		if comparePrecision(finding.AnalysisPrecision, threshold) >= 0 {
			return true
		}
	}
	return false
}

// ParseRiskLevel parses a CLI threshold.
func ParseRiskLevel(s string) (query.RiskLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "high":
		return query.RiskHigh, nil
	case "low":
		return query.RiskLow, nil
	case "medium":
		return query.RiskMedium, nil
	case "destructive":
		return query.RiskDestructive, nil
	case "blocked":
		return query.RiskBlocked, nil
	default:
		return "", fmt.Errorf("unknown risk level %q", s)
	}
}

// ParseAnalysisPrecision parses a CLI precision threshold.
func ParseAnalysisPrecision(s string) (query.AnalysisPrecision, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return "", nil
	case "precise":
		return query.AnalysisPrecise, nil
	case "partial":
		return query.AnalysisPartial, nil
	case "unsupported":
		return query.AnalysisUnsupported, nil
	default:
		return "", fmt.Errorf("unknown analysis precision %q", s)
	}
}

func comparePrecision(a, b query.AnalysisPrecision) int {
	return precisionRank(a) - precisionRank(b)
}

func precisionRank(precision query.AnalysisPrecision) int {
	switch precision {
	case query.AnalysisPrecise, "":
		return 0
	case query.AnalysisPartial:
		return 1
	case query.AnalysisUnsupported:
		return 2
	default:
		return 0
	}
}

func compareRisk(a, b query.RiskLevel) int {
	return riskRank(a) - riskRank(b)
}

func riskRank(level query.RiskLevel) int {
	switch level {
	case query.RiskLow, "":
		return 0
	case query.RiskMedium:
		return 1
	case query.RiskHigh:
		return 2
	case query.RiskDestructive:
		return 3
	case query.RiskBlocked:
		return 4
	default:
		return 0
	}
}
