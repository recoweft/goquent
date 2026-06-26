package orm

import (
	"context"

	"github.com/recoweft/goquent/orm/migration"
)

type MigrationStepType = migration.MigrationStepType
type MigrationStatement = migration.MigrationStatement
type MigrationStep = migration.MigrationStep
type MigrationPlan = migration.MigrationPlan
type Migrator = migration.Migrator
type Schema = migration.Schema
type TableSchema = migration.TableSchema
type ColumnSchema = migration.ColumnSchema
type IndexSchema = migration.IndexSchema

const (
	AddTable         = migration.AddTable
	DropTable        = migration.DropTable
	AddColumn        = migration.AddColumn
	DropColumn       = migration.DropColumn
	RenameColumn     = migration.RenameColumn
	AlterColumnType  = migration.AlterColumnType
	AlterNullability = migration.AlterNullability
	AddIndex         = migration.AddIndex
	DropIndex        = migration.DropIndex
	UnsupportedStep  = migration.UnsupportedStep

	WarningMigrationUnsupported           = migration.WarningMigrationUnsupported
	WarningMigrationDropTable             = migration.WarningMigrationDropTable
	WarningMigrationDropColumn            = migration.WarningMigrationDropColumn
	WarningMigrationAddNotNullColumn      = migration.WarningMigrationAddNotNullColumn
	WarningMigrationRenameColumn          = migration.WarningMigrationRenameColumn
	WarningMigrationAlterColumnType       = migration.WarningMigrationAlterColumnType
	WarningMigrationTypeNarrowing         = migration.WarningMigrationTypeNarrowing
	WarningMigrationSetNotNull            = migration.WarningMigrationSetNotNull
	WarningMigrationAddIndexNonConcurrent = migration.WarningMigrationAddIndexNonConcurrent
	WarningMigrationDropIndex             = migration.WarningMigrationDropIndex
)

func NewMigrator(sqlText string) *Migrator {
	return migration.New(sqlText)
}

func PlanMigrationSQL(ctx context.Context, sqlText string) (*MigrationPlan, error) {
	return migration.New(sqlText).Plan(ctx)
}

func PlanMigrationSteps(steps []MigrationStep) *MigrationPlan {
	return migration.PlanSteps(steps)
}

func DiffSchemas(current, desired Schema) *MigrationPlan {
	return migration.DiffSchemas(current, desired)
}
