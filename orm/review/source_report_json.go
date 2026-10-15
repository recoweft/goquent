package review

import (
	"bytes"
	"encoding/json"
	"strings"
	"time"

	"github.com/recoweft/goquent/orm/query"
)

// sourceReportEnvelope recognizes unsupported diagnostic artifacts, not evidence.
// A top-level report list (including a truncated list) is enough for legacy input.
// Tokens inside strings or nested objects cannot establish an envelope.
func sourceReportEnvelope(b []byte) bool {
	if len(b) > maxTypeFileBytes {
		b = b[:maxTypeFileBytes]
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	tok, err := d.Token()
	if err != nil || tok != json.Delim('{') {
		return false
	}
	candidate := false
	tokens := 0
	for d.More() && tokens < 65536 {
		key, err := d.Token()
		if err != nil {
			break
		}
		tokens++
		name, ok := key.(string)
		if !ok {
			break
		}
		name = strings.ToLower(name)
		reportList := name == "findings" || name == "suppressed_findings" || name == "operations"
		tok, err = d.Token()
		tokens++
		if err != nil {
			candidate = candidate || reportList
			break
		}
		if reportList && (tok == json.Delim('[') || tok == nil) {
			candidate = true
		}
		depth := 0
		if tok == json.Delim('[') || tok == json.Delim('{') {
			depth = 1
		}
		for depth > 0 && depth <= 64 && tokens < 65536 {
			tok, err = d.Token()
			tokens++
			if err != nil {
				break
			}
			switch tok {
			case json.Delim('['), json.Delim('{'):
				depth++
			case json.Delim(']'), json.Delim('}'):
				depth--
			}
		}
		if err != nil || depth != 0 {
			break
		}
	}
	if !candidate {
		return false
	}
	// Only independently valid public views are display envelopes. Merely claiming
	// their kind cannot disguise mixed, duplicate or damaged source input.
	if _, err := DecodeReportView(b); err == nil {
		return false
	}
	if _, err := DecodeReportViewV2(b); err == nil {
		return false
	}
	return true
}

func unsupportedReportJSONFindings(ctx reviewContext, path string) ([]Finding, error) {
	marker := staticFinding(query.WarningStaticReviewUnsupported, query.RiskMedium,
		"source review report is not a supported review file input",
		"review original inputs; serialized diagnostics are not evidence",
		&query.SourceLocation{File: path, Line: 1}, query.AnalysisUnsupported)
	fs := applyReviewRules([]Finding{marker}, ctx.rules)
	config, err := configSuppressionsForFile(path, ctx.configSuppressions)
	if err != nil {
		return nil, err
	}
	// Supplied JSON strings and suppression fields never configure this marker.
	return applyReviewSuppressions(fs, config, time.Now().UTC()), nil
}
