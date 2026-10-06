// Package planversion implements the diagnostic/input envelope version contract.
package planversion

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const Current = 1

var ErrVersion = errors.New("goquent: invalid, ambiguous or unsupported envelope version")

func Check(v int) error {
	if v != 0 && v != Current {
		return ErrVersion
	}
	return nil
}

// Read checks spelling and duplicate declarations before a struct decoder can
// apply case-insensitive matching or last-wins semantics. Missing is legacy zero.
func Read(b []byte) error {
	d := json.NewDecoder(bytes.NewReader(b))
	t, err := d.Token()
	if err != nil {
		return err
	}
	if t != json.Delim('{') {
		return ErrVersion
	}
	seen := false
	for d.More() {
		t, err = d.Token()
		if err != nil {
			return err
		}
		k, ok := t.(string)
		if !ok {
			return ErrVersion
		}
		var raw json.RawMessage
		if err = d.Decode(&raw); err != nil {
			return err
		}
		if strings.EqualFold(k, "version") {
			if seen || k != "version" {
				return ErrVersion
			}
			seen = true
			if string(raw) != "0" && string(raw) != "1" {
				return ErrVersion
			}
		}
	}
	if _, err = d.Token(); err != nil {
		return err
	}
	var extra any
	if err = d.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return ErrVersion
	}
	return nil
}

// Decode preserves JSON number lexemes; it never infers native integer widths.
func Decode(b []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := d.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return ErrVersion
	}
	return nil
}
