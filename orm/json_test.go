package orm

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/driver"
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

func TestJSONPathBuildsUpdateKey(t *testing.T) {
	key, err := JSONPath("payload", "retrieval", "score")
	if err != nil {
		t.Fatalf("JSONPath: %v", err)
	}
	if key != "payload->retrieval->score" {
		t.Fatalf("unexpected json path key: %s", key)
	}
	if _, err := JSONPath("payload", "bad-key"); err == nil || !strings.Contains(err.Error(), "safe identifier") {
		t.Fatalf("expected invalid path element error, got %v", err)
	}
}

func TestJSONPathUpdatePlan(t *testing.T) {
	key, err := JSONPath("payload", "status")
	if err != nil {
		t.Fatalf("JSONPath: %v", err)
	}
	db, _ := newScopeMockDB(t, driver.PostgresDialect{})
	plan, err := db.Table("events").
		Where("id", "event-1").
		PlanUpdate(context.Background(), map[string]any{key: `{"value":"ok"}`})
	if err != nil {
		t.Fatalf("PlanUpdate: %v", err)
	}
	if !strings.Contains(plan.SQL, `jsonb_set("payload", '{status}', $1)`) {
		t.Fatalf("expected jsonb_set update, sql=%s", plan.SQL)
	}
}

func TestJSONAggregateHelpers(t *testing.T) {
	objectExpr, err := JSONBuildObject(driver.PostgresDialect{}, map[string]string{
		"id":    "cells.id",
		"value": "cells.value_json",
	})
	if err != nil {
		t.Fatalf("JSONBuildObject postgres: %v", err)
	}
	if objectExpr.SQL != "jsonb_build_object('id', cells.id, 'value', cells.value_json)" {
		t.Fatalf("unexpected object expr: %s", objectExpr.SQL)
	}
	aggExpr, err := JSONAggregateArray(
		driver.PostgresDialect{},
		objectExpr,
		JSONAggOrderBy("cells.position ASC"),
		JSONAggFilter("cells.id IS NOT NULL"),
	)
	if err != nil {
		t.Fatalf("JSONAggregateArray postgres: %v", err)
	}
	for _, want := range []string{"COALESCE(jsonb_agg(", "ORDER BY cells.position ASC", "FILTER (WHERE cells.id IS NOT NULL)", "'[]'::jsonb"} {
		if !strings.Contains(aggExpr.SQL, want) {
			t.Fatalf("expected %q in aggregate expr: %s", want, aggExpr.SQL)
		}
	}

	mysqlObject, err := JSONBuildObject(driver.MySQLDialect{}, map[string]string{"id": "cells.id"})
	if err != nil {
		t.Fatalf("JSONBuildObject mysql: %v", err)
	}
	mysqlAgg, err := JSONAggregateArray(driver.MySQLDialect{}, mysqlObject, JSONAggOrderBy("cells.position ASC"))
	if err != nil {
		t.Fatalf("JSONAggregateArray mysql: %v", err)
	}
	if !strings.Contains(mysqlAgg.SQL, "COALESCE(JSON_ARRAYAGG(JSON_OBJECT('id', cells.id) ORDER BY cells.position ASC), JSON_ARRAY())") {
		t.Fatalf("unexpected mysql aggregate: %s", mysqlAgg.SQL)
	}
	if _, err := JSONAggregateArray(driver.MySQLDialect{}, mysqlObject, JSONAggFilter("cells.id IS NOT NULL")); err == nil {
		t.Fatalf("expected MySQL filter error")
	}
	if _, err := JSONBuildObject(driver.PostgresDialect{}, map[string]string{"id": "cells.id; DROP TABLE cells"}); err == nil {
		t.Fatalf("expected unsafe expression error")
	}
}
