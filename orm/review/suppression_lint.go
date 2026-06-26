package review

import (
	"strings"
	"time"

	"github.com/recoweft/goquent/orm/query"
)

const (
	WarningSuppressionUnused        = "SUPPRESSION_UNUSED"
	WarningSuppressionOverbroad     = "SUPPRESSION_OVERBROAD"
	WarningSuppressionOwnerMissing  = "SUPPRESSION_OWNER_MISSING"
	WarningSuppressionReasonWeak    = "SUPPRESSION_REASON_WEAK"
	WarningSuppressionConfigInvalid = "SUPPRESSION_CONFIG_INVALID"
)

func lintConfigSuppressions(configPath string, configs []ConfigSuppression, reviewedFiles []string, suppressed []Finding) []Finding {
	if len(configs) == 0 {
		return nil
	}

	used := make(map[string]struct{})
	for _, finding := range suppressed {
		if finding.Suppression == nil || finding.Suppression.Scope != query.SuppressionScopeConfig {
			continue
		}
		used[suppressionUsageKey(finding.Suppression.Code, finding.Suppression.Reason, finding.Suppression.Owner, finding.Suppression.ExpiresAt)] = struct{}{}
	}

	now := time.Now().UTC()
	var findings []Finding
	for _, cfg := range configs {
		loc := configLintLocation(configPath)
		evidence := configSuppressionEvidence(cfg)
		code := strings.TrimSpace(cfg.Code)
		reason := strings.TrimSpace(cfg.Reason)
		owner := strings.TrimSpace(cfg.Owner)

		var expiresAt *time.Time
		valid := true
		if code == "" {
			findings = append(findings, configSuppressionLintFinding(WarningSuppressionConfigInvalid,
				"config suppression code is required", loc, evidence))
			valid = false
		}
		if reason == "" {
			findings = append(findings, configSuppressionLintFinding(WarningSuppressionConfigInvalid,
				"config suppression reason is required", loc, evidence))
			valid = false
		}
		if strings.TrimSpace(cfg.Expires) != "" {
			parsed, err := parseReviewTime(cfg.Expires)
			if err != nil {
				findings = append(findings, configSuppressionLintFinding(WarningSuppressionConfigInvalid,
					err.Error(), loc, evidence))
				valid = false
			} else {
				expiresAt = &parsed
			}
		}
		if !valid {
			continue
		}

		if expiresAt != nil && !expiresAt.After(now) {
			findings = append(findings, configSuppressionLintFinding(query.WarningSuppressionExpired,
				"config suppression has expired", loc, evidence))
		}
		if !findingSuppressible(code) {
			findings = append(findings, configSuppressionLintFinding(query.WarningSuppressionNotAllowed,
				"config suppression targets a non-suppressible finding", loc, evidence))
		}
		if configSuppressionPathOverbroad(cfg.Path) {
			findings = append(findings, configSuppressionLintFinding(WarningSuppressionOverbroad,
				"config suppression path is broad; narrow it to the smallest file or directory", loc, evidence))
		}
		if owner == "" {
			findings = append(findings, configSuppressionLintFinding(WarningSuppressionOwnerMissing,
				"config suppression owner is missing", loc, evidence))
		}
		if configSuppressionReasonWeak(reason) {
			findings = append(findings, configSuppressionLintFinding(WarningSuppressionReasonWeak,
				"config suppression reason is too short or generic", loc, evidence))
		}
		if _, ok := used[suppressionUsageKey(code, reason, owner, expiresAt)]; !ok {
			msg := "config suppression did not match any finding"
			if !configSuppressionMatchesReviewedFile(reviewedFiles, cfg.Path) {
				msg = "config suppression path did not match any reviewed file"
			}
			findings = append(findings, configSuppressionLintFinding(WarningSuppressionUnused, msg, loc, evidence))
		}
	}
	return findings
}

func suppressionUsageKey(code, reason, owner string, expiresAt *time.Time) string {
	expires := ""
	if expiresAt != nil {
		expires = expiresAt.UTC().Format(time.RFC3339)
	}
	return strings.Join([]string{
		strings.TrimSpace(code),
		strings.TrimSpace(reason),
		strings.TrimSpace(owner),
		expires,
	}, "\x00")
}

func configSuppressionLintFinding(code, message string, loc *query.SourceLocation, evidence []query.Evidence) Finding {
	return Finding{
		Code:              code,
		Level:             query.RiskMedium,
		Message:           message,
		Location:          cloneLocation(loc),
		Hint:              "remove the suppression or narrow it with a current, specific reason and owner",
		Evidence:          append([]query.Evidence(nil), evidence...),
		AnalysisPrecision: query.AnalysisPrecise,
	}
}

func configLintLocation(path string) *query.SourceLocation {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return &query.SourceLocation{File: path, Line: 1}
}

func configSuppressionEvidence(cfg ConfigSuppression) []query.Evidence {
	evidence := []query.Evidence{
		{Key: "suppression_code", Value: strings.TrimSpace(cfg.Code)},
	}
	if path := strings.TrimSpace(cfg.Path); path != "" {
		evidence = append(evidence, query.Evidence{Key: "suppression_path", Value: path})
	}
	if owner := strings.TrimSpace(cfg.Owner); owner != "" {
		evidence = append(evidence, query.Evidence{Key: "suppression_owner", Value: owner})
	}
	if expires := strings.TrimSpace(cfg.Expires); expires != "" {
		evidence = append(evidence, query.Evidence{Key: "suppression_expires", Value: expires})
	}
	return evidence
}

func configSuppressionPathOverbroad(pattern string) bool {
	pattern = filepathLikePattern(pattern)
	switch pattern {
	case "", ".", "./", "/", "*", "**", "...", "./...":
		return true
	default:
		return false
	}
}

func filepathLikePattern(pattern string) string {
	pattern = strings.TrimSpace(pattern)
	pattern = strings.ReplaceAll(pattern, "\\", "/")
	for strings.Contains(pattern, "//") {
		pattern = strings.ReplaceAll(pattern, "//", "/")
	}
	return pattern
}

func configSuppressionReasonWeak(reason string) bool {
	reason = strings.ToLower(strings.TrimSpace(reason))
	if len(reason) < 20 || len(strings.Fields(reason)) < 3 {
		return true
	}
	switch reason {
	case "approved", "reviewed", "temporary", "todo", "ignore", "ignored", "n/a", "na", "safe", "test", "fixture", "known issue":
		return true
	default:
		return false
	}
}

func configSuppressionMatchesReviewedFile(files []string, pattern string) bool {
	for _, file := range files {
		if configSuppressionMatchesPath(file, pattern) {
			return true
		}
	}
	return false
}
