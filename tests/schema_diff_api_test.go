package tests

import (
	"testing"

	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/migration"
)

func TestSchemaDefinitionFacade(t *testing.T) {
	var state orm.DefaultState = orm.DefaultNone
	var definition orm.DefaultDefinition
	definition.State = state
	current := orm.Schema{Tables: []orm.TableSchema{{Name: "records", Columns: []orm.ColumnSchema{{Name: "value", DefaultState: definition.State}}}}}
	desired := orm.Schema{Tables: []orm.TableSchema{{Name: "records", Columns: []orm.ColumnSchema{{Name: "value", DefaultState: orm.DefaultExpression, HasDefault: true, DefaultExpression: "1"}}}}}
	p := orm.DiffSchemas(current, desired)
	if len(p.Steps) != 1 || p.Steps[0].Type != orm.AlterColumnDefault || p.Steps[0].Warnings[0].Code != orm.WarningMigrationAlterColumnDefault {
		t.Fatal("facade comparison contract")
	}
	p = orm.PlanMigrationSteps([]orm.MigrationStep{{Type: orm.AlterIndex}})
	if !p.Blocked || p.Warnings[0].Code != orm.WarningMigrationAlterIndex {
		t.Fatal("facade plan contract")
	}
	if orm.DefaultUnknown != migration.DefaultUnknown || orm.WarningSchemaComparisonUnknown != migration.WarningSchemaComparisonUnknown {
		t.Fatal("facade classifications diverged")
	}
}
