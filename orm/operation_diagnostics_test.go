package orm

import (
	"errors"
	"testing"

	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/operation"
	"github.com/recoweft/goquent/orm/query"
)

func TestDBOperationDiagnosticsSettingsAndNoExecutor(t *testing.T) {
	// A nil executor would panic if either diagnostic entry dispatched.
	db := NewDBWithExecutor(nil, driver.PostgresDialect{}, WithSettings(Settings{}))
	spec := OperationSpec{Model: "items", Select: []string{"v"}, Filters: []FilterSpec{{Field: "v", Value: 1}}}
	opts := OperationOptions{Manifest: &manifest.Manifest{Version: "1", Dialect: "postgres", Tables: []manifest.Table{{Name: "items", Columns: []manifest.Column{{Name: "v", Type: "bigint", TypeSource: "sql", NullableKnown: true}}}}}}
	invalid := query.NewSettings(query.PolicySet{}, query.RiskConfig{}, query.ExecutionContext{}).WithTenantPolicy("missing", query.ApplicationSchema{}, false)
	opts.Settings = &invalid
	opts.Dialect = driver.MySQLDialect{}
	p, v, e := db.CompileOperationWithDiagnostics(t.Context(), spec, opts)
	if e != nil || p == nil || v.Outcome != "compiled" || v.Coverage != "checked_subset" {
		t.Fatal("caller overrides DB settings/dialect")
	}
	_, vv, e := db.ValidateOperationWithDiagnostics(spec, opts)
	if e != nil || vv.String() != v.String() {
		t.Fatal("DB validate differs")
	}
	spec.Filters[0].Value = "wrong"
	p, v, e = db.CompileOperationWithDiagnostics(t.Context(), spec, opts)
	if p != nil || !errors.Is(e, operation.ErrTypeMismatch) || v.Diagnostics[0].Code != "OPERATION_TYPE_MISMATCH" {
		t.Fatal("DB refusal identity")
	}
}
