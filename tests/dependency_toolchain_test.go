package tests

import (
	"encoding/asn1"
	"testing"
)

// TestDependencyASN1DepthRejection guards the Go runtime repair for
// GO-2026-5972. It checks decoding behavior, not a toolchain version string.
// This is a bounded standard-library regression, not a TLS exploitability test.
func TestDependencyASN1DepthRejection(t *testing.T) {
	type sequence []sequence
	for _, tc := range []struct {
		name   string
		depth  int
		reject bool
	}{
		{"ordinary", 16, false},
		{"excessive", 10001, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var value sequence
			for range tc.depth - 1 {
				value = sequence{value}
			}
			encoded, err := asn1.Marshal(value)
			if err != nil {
				t.Fatal("could not construct bounded ASN.1 fixture")
			}
			var decoded sequence
			rest, err := asn1.Unmarshal(encoded, &decoded)
			if tc.reject {
				if err == nil {
					t.Fatal("excessive ASN.1 nesting was accepted")
				}
			} else if err != nil || len(rest) != 0 {
				t.Fatal("ordinary ASN.1 sequence failed to decode completely")
			}
		})
	}
}
