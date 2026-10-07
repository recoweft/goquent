package manifest

import (
	"encoding/json"
	"github.com/recoweft/goquent/orm/publicoutput"
	"io"
)

// WriteJSON writes internal manifest data that can contain secrets.
func WriteJSON(w io.Writer, m *Manifest) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

// WritePretty writes a redacted manifest summary.
func WritePretty(w io.Writer, m *Manifest) error { return publicoutput.WriteSummary(w, m.PublicView()) }

// WriteVerificationPretty writes redacted supplied verification claims.
func WriteVerificationPretty(w io.Writer, v Verification) error {
	return publicoutput.WriteSummary(w, v.PublicView())
}

// WriteVerificationJSON writes sensitive internal verification data.
func WriteVerificationJSON(w io.Writer, v Verification) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
