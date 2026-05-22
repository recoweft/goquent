package orm

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
)

type jsonSummary struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

func TestJSONFieldScanValueDefaultAndValidate(t *testing.T) {
	var field JSONField[jsonSummary]
	if err := field.Scan([]byte(`{"status":"ok","count":2}`)); err != nil {
		t.Fatalf("scan json: %v", err)
	}
	if !field.Valid || field.Data.Status != "ok" || field.Data.Count != 2 {
		t.Fatalf("unexpected field: %+v", field)
	}
	value, err := field.Value()
	if err != nil {
		t.Fatalf("value: %v", err)
	}
	if !strings.Contains(value.(string), `"status":"ok"`) {
		t.Fatalf("unexpected JSON value: %v", value)
	}
	if got := (JSONField[jsonSummary]{}).OrDefault(jsonSummary{Status: "empty"}); got.Status != "empty" {
		t.Fatalf("default=%+v", got)
	}
	if err := field.Validate(func(v jsonSummary) error {
		if v.Count != 2 {
			return errors.New("bad count")
		}
		return nil
	}); err != nil {
		t.Fatalf("validate: %v", err)
	}
}

func TestEncodeDecodeJSONHelpers(t *testing.T) {
	encoded, err := EncodeJSON(jsonSummary{Status: "ok", Count: 2})
	if err != nil {
		t.Fatalf("EncodeJSON: %v", err)
	}
	if !strings.Contains(encoded, `"status":"ok"`) {
		t.Fatalf("encoded=%s", encoded)
	}

	decoded, err := DecodeJSON([]byte(encoded), jsonSummary{Status: "fallback"})
	if err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if decoded.Status != "ok" || decoded.Count != 2 {
		t.Fatalf("decoded=%+v", decoded)
	}

	fallback, err := DecodeJSON(nil, jsonSummary{Status: "fallback"})
	if err != nil {
		t.Fatalf("DecodeJSON nil: %v", err)
	}
	if fallback.Status != "fallback" {
		t.Fatalf("fallback=%+v", fallback)
	}
}

func TestJSONFieldNullAndNullableStrings(t *testing.T) {
	field := JSONOf(jsonSummary{Status: "ok"})
	if err := field.Scan(nil); err != nil {
		t.Fatalf("scan null: %v", err)
	}
	if field.Valid {
		t.Fatalf("expected invalid null field: %+v", field)
	}
	value, err := field.Value()
	if err != nil {
		t.Fatalf("value null: %v", err)
	}
	if value != nil {
		t.Fatalf("expected nil driver value, got %v", value)
	}

	tenant := "tenant-1"
	if got := NullStringPtr(&tenant); !got.Valid || got.String != "tenant-1" {
		t.Fatalf("NullStringPtr=%+v", got)
	}
	if got := NullStringPtr(nil); got.Valid {
		t.Fatalf("nil NullStringPtr=%+v", got)
	}
	if got := NullStringEmpty(""); got != (sql.NullString{}) {
		t.Fatalf("empty NullString=%+v", got)
	}
}
