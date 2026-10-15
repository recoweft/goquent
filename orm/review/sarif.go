package review

import (
	"encoding/json"
	"io"
	"sort"

	"github.com/recoweft/goquent/orm/internal/publicview"
)

// WriteSARIF writes a SARIF 2.1.0 public projection, never file annotations.
// Line/column properties match the public JSON projection of canonical Run
// diagnostics. File identity and all supplied URI/message claims are omitted.
func WriteSARIF(w io.Writer, r ReviewReport) error { return writeSARIF(w, canonicalDisplay(r), nil) }
func (r CIResult) WriteSARIF(w io.Writer) error {
	v, e := r.PublicView()
	if e != nil {
		return e
	}
	return writeSARIF(w, r.selected, &v)
}
func writeSARIF(w io.Writer, r ReviewReport, ci *CIView) error {
	v, e := r.PublicViewV2()
	if e != nil {
		return publicview.ErrSource
	}
	results := []any{}
	codes := map[string]bool{}
	appendList := func(fs []FindingView) {
		for _, f := range fs {
			codes[f.Code] = true
			level := "warning"
			switch f.Level {
			case "low":
				level = "note"
			case "high", "destructive", "blocked":
				level = "error"
			}
			props := map[string]any{"risk": f.Level, "precision": f.Precision, "line": f.Line, "column": f.Column, "columnEncoding": "utf8Bytes", "fileIdentityOmitted": true, "detailsOmitted": true, "suppressed": f.Suppressed}
			item := map[string]any{"ruleId": f.Code, "level": level, "message": map[string]string{"text": f.Message}, "properties": props}
			if f.Suppressed {
				item["suppressions"] = []any{map[string]string{"kind": "external", "status": "accepted", "justification": "review_suppression_only"}}
				props["suppressionReason"] = "review_suppression_only"
			}
			results = append(results, item)
		}
	}
	appendList(v.Findings)
	appendList(v.SuppressedFindings)
	names := make([]string, 0, len(codes))
	for c := range codes {
		names = append(names, c)
	}
	sort.Strings(names)
	rules := []any{}
	for _, c := range names {
		rules = append(rules, map[string]any{"id": c, "shortDescription": map[string]string{"text": publicview.Message(c)}})
	}
	props := map[string]any{"findingCount": v.FindingCount, "suppressedFindingCount": v.SuppressedFindingCount, "truncated": v.Truncated, "detailsOmitted": true, "fileIdentityOmitted": true, "operations": v.Operations, "operationCount": v.OperationCount}
	if ci != nil {
		props["reviewCI"] = ci
	}
	run := map[string]any{"tool": map[string]any{"driver": map[string]any{"name": "goquent", "version": "review-sarif-v1", "informationUri": "https://github.com/recoweft/goquent", "rules": rules}}, "results": results, "properties": props}
	doc := map[string]any{"$schema": "https://docs.oasis-open.org/sarif/sarif/v2.1.0/errata01/os/schemas/sarif-schema-2.1.0.json", "version": "2.1.0", "runs": []any{run}}
	b, e := json.Marshal(doc)
	if e != nil {
		return publicview.ErrInvalid
	}
	return publicview.Write(w, append(b, '\n'))
}
