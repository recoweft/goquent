package review

import (
	"encoding/json"
	"io"
)

// WriteJSON writes internal diagnostic data that can contain secrets.
// Public consumers must use PublicView and WritePublicJSON.
func WriteJSON(w io.Writer, report ReviewReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

// WritePretty writes a redacted public report. Decisions must use the source report.
func WritePretty(w io.Writer, report ReviewReport) error {
	v, err := report.PublicView()
	if err != nil {
		return err
	}
	return WritePublicPretty(w, v)
}

// WriteGitHub writes redacted annotations without source paths or messages.
func WriteGitHub(w io.Writer, report ReviewReport) error {
	v, err := report.PublicView()
	if err != nil {
		return err
	}
	return WritePublicGitHub(w, v)
}
