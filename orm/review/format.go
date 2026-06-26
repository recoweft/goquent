package review

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/recoweft/goquent/orm/query"
)

// WriteJSON writes a stable machine-readable review report.
func WriteJSON(w io.Writer, report ReviewReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// WritePretty writes a human-readable review report.
func WritePretty(w io.Writer, report ReviewReport) error {
	if _, err := fmt.Fprintln(w, "Database Review"); err != nil {
		return err
	}
	if len(report.Findings) == 0 {
		_, err := fmt.Fprintln(w, "\nNo findings.")
		return err
	}

	for _, finding := range report.Findings {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		suffix := ""
		if finding.Suppressed {
			suffix = " suppressed"
		}
		if _, err := fmt.Fprintf(w, "[%s%s] %s: %s\n", riskLabel(finding.Level), suffix, finding.Code, finding.Message); err != nil {
			return err
		}
		if finding.Location != nil {
			if finding.Location.Column > 0 {
				if _, err := fmt.Fprintf(w, "  file: %s:%d:%d\n", finding.Location.File, finding.Location.Line, finding.Location.Column); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintf(w, "  file: %s:%d\n", finding.Location.File, finding.Location.Line); err != nil {
					return err
				}
			}
		}
		if _, err := fmt.Fprintf(w, "  precision: %s\n", finding.AnalysisPrecision); err != nil {
			return err
		}
		if finding.Hint != "" {
			if _, err := fmt.Fprintf(w, "  hint: %s\n", finding.Hint); err != nil {
				return err
			}
		}
		for _, evidence := range finding.Evidence {
			if evidence.Key == "" {
				continue
			}
			if _, err := fmt.Fprintf(w, "  evidence: %s=%v\n", evidence.Key, evidence.Value); err != nil {
				return err
			}
		}
		if finding.Suppression != nil {
			if _, err := fmt.Fprintf(w, "  suppression_reason: %s\n", finding.Suppression.Reason); err != nil {
				return err
			}
			if finding.Suppression.Owner != "" {
				if _, err := fmt.Fprintf(w, "  suppression_owner: %s\n", finding.Suppression.Owner); err != nil {
					return err
				}
			}
			if finding.Suppression.ExpiresAt != nil {
				if _, err := fmt.Fprintf(w, "  suppression_expires: %s\n", finding.Suppression.ExpiresAt.Format("2006-01-02T15:04:05Z07:00")); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// WriteGitHub writes GitHub Actions annotations for review findings.
func WriteGitHub(w io.Writer, report ReviewReport) error {
	for _, finding := range report.Findings {
		level := "warning"
		if compareRisk(finding.Level, query.RiskHigh) >= 0 {
			level = "error"
		}
		if finding.Suppressed {
			level = "notice"
		}
		loc := finding.Location
		if loc == nil {
			loc = &query.SourceLocation{}
		}
		props := []string{}
		if loc.File != "" {
			props = append(props, "file="+escapeGitHubProperty(loc.File))
		}
		if loc.Line > 0 {
			props = append(props, fmt.Sprintf("line=%d", loc.Line))
		}
		if loc.Column > 0 {
			props = append(props, fmt.Sprintf("col=%d", loc.Column))
		}
		message := fmt.Sprintf("[%s] %s", finding.Code, finding.Message)
		if finding.Hint != "" {
			message += " hint: " + finding.Hint
		}
		if evidence := evidenceSummary(finding.Evidence); evidence != "" {
			message += " evidence: " + evidence
		}
		if finding.Suppression != nil && finding.Suppression.Reason != "" {
			message += " suppression_reason: " + finding.Suppression.Reason
		}
		propText := ""
		if len(props) > 0 {
			propText = " " + strings.Join(props, ",")
		}
		if _, err := fmt.Fprintf(w, "::%s%s::%s\n", level, propText, escapeGitHubMessage(message)); err != nil {
			return err
		}
	}
	return nil
}

func evidenceSummary(evidence []query.Evidence) string {
	if len(evidence) == 0 {
		return ""
	}
	parts := make([]string, 0, len(evidence))
	for _, item := range evidence {
		if item.Key == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%v", item.Key, item.Value))
	}
	return strings.Join(parts, ", ")
}

func riskLabel(level query.RiskLevel) string {
	switch level {
	case query.RiskLow:
		return "Low"
	case query.RiskMedium:
		return "Medium"
	case query.RiskHigh:
		return "High"
	case query.RiskDestructive:
		return "Destructive"
	case query.RiskBlocked:
		return "Blocked"
	default:
		return string(level)
	}
}

func escapeGitHubMessage(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	return s
}

func escapeGitHubProperty(s string) string {
	s = escapeGitHubMessage(s)
	s = strings.ReplaceAll(s, ":", "%3A")
	s = strings.ReplaceAll(s, ",", "%2C")
	return s
}
