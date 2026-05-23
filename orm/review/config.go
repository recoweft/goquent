package review

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/faciam-dev/goquent/orm/query"
)

// Config is the JSON configuration accepted by goquent review --config.
type Config struct {
	Manifest             string                          `json:"manifest,omitempty"`
	Schema               string                          `json:"schema,omitempty"`
	Policy               string                          `json:"policy,omitempty"`
	DatabaseSchema       string                          `json:"database_schema,omitempty"`
	Code                 []string                        `json:"code,omitempty"`
	RequireFreshManifest bool                            `json:"require_fresh_manifest,omitempty"`
	FailOn               string                          `json:"fail_on,omitempty"`
	FailOnPrecision      string                          `json:"fail_on_precision,omitempty"`
	ShowSuppressed       bool                            `json:"show_suppressed,omitempty"`
	Rules                map[string]query.RiskRuleConfig `json:"rules,omitempty"`
	Suppressions         []ConfigSuppression             `json:"suppressions,omitempty"`
}

// ConfigSuppression suppresses a finding for paths matched by Path.
type ConfigSuppression struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Reason  string `json:"reason"`
	Owner   string `json:"owner,omitempty"`
	Expires string `json:"expires,omitempty"`
}

// LoadConfig reads a JSON review config file.
func LoadConfig(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyReviewRules(findings []Finding, rules map[string]query.RiskRuleConfig) []Finding {
	if len(findings) == 0 || len(rules) == 0 {
		return findings
	}
	out := make([]Finding, 0, len(findings))
	for _, finding := range findings {
		rule, ok := rules[finding.Code]
		if !ok {
			out = append(out, finding)
			continue
		}
		if rule.Enabled != nil && !*rule.Enabled {
			continue
		}
		if rule.Severity != nil {
			finding.Level = *rule.Severity
		}
		out = append(out, finding)
	}
	return out
}

func configSuppressionsForFile(path string, configs []ConfigSuppression) ([]query.Suppression, error) {
	var suppressions []query.Suppression
	for _, cfg := range configs {
		if !configSuppressionMatchesPath(path, cfg.Path) {
			continue
		}
		suppression := query.Suppression{
			Code:   strings.TrimSpace(cfg.Code),
			Reason: strings.TrimSpace(cfg.Reason),
			Scope:  query.SuppressionScopeConfig,
			Owner:  strings.TrimSpace(cfg.Owner),
		}
		if suppression.Code == "" {
			return nil, fmt.Errorf("goquent: config suppression code is required")
		}
		if suppression.Reason == "" {
			return nil, fmt.Errorf("goquent: config suppression reason is required")
		}
		if strings.TrimSpace(cfg.Expires) != "" {
			expiresAt, err := parseReviewTime(cfg.Expires)
			if err != nil {
				return nil, err
			}
			suppression.ExpiresAt = &expiresAt
		}
		suppressions = append(suppressions, suppression)
	}
	return suppressions, nil
}

func configSuppressionMatchesPath(path, pattern string) bool {
	pattern = filepath.ToSlash(strings.TrimSpace(pattern))
	if pattern == "" {
		return true
	}
	path = filepath.ToSlash(path)
	if filepath.IsAbs(path) {
		if rel, err := filepath.Rel(".", path); err == nil && !strings.HasPrefix(rel, "..") {
			path = filepath.ToSlash(rel)
		}
	}
	if path == pattern || strings.HasSuffix(path, "/"+pattern) {
		return true
	}
	if strings.HasSuffix(pattern, "/...") {
		prefix := strings.TrimSuffix(pattern, "/...")
		return path == prefix || strings.HasPrefix(path, prefix+"/") || strings.HasSuffix(path, "/"+prefix) || strings.Contains(path, "/"+prefix+"/")
	}
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(path, pattern) || strings.Contains(path, "/"+pattern)
	}
	return false
}

func parseReviewTime(value string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("goquent: invalid config suppression expires %q", value)
}
