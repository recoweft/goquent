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
type DefaultState = migration.DefaultState
type DefaultDefinition = migration.DefaultDefinition

const (
	DefaultUnknown                     = migration.DefaultUnknown
	DefaultNone                        = migration.DefaultNone
	DefaultExpression                  = migration.DefaultExpression
	AlterIndex                         = migration.AlterIndex
	AlterColumnDefault                 = migration.AlterColumnDefault
	WarningSchemaComparisonUnknown     = migration.WarningSchemaComparisonUnknown
	WarningMigrationAlterIndex         = migration.WarningMigrationAlterIndex
	WarningMigrationAlterColumnDefault = migration.WarningMigrationAlterColumnDefault
	AddTable                           = migration.AddTable
	DropTable                          = migration.DropTable
	AddColumn                          = migration.AddColumn
	DropColumn                         = migration.DropColumn
	RenameColumn                       = migration.RenameColumn
	AlterColumnType                    = migration.AlterColumnType
	AlterNullability                   = migration.AlterNullability
	AddIndex                           = migration.AddIndex
	DropIndex                          = migration.DropIndex
	UnsupportedStep                    = migration.UnsupportedStep

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

type CoverageState = migration.CoverageState
type SchemaCoverage = migration.SchemaCoverage
type SchemaContext = migration.SchemaContext
type ObjectIdentity = migration.ObjectIdentity
type ExpressionDefinition = migration.ExpressionDefinition
type IndexKey = migration.IndexKey
type ForeignKeySchema = migration.ForeignKeySchema
type CheckSchema = migration.CheckSchema
type CapabilityAssessment = migration.CapabilityAssessment
type OperationCapability = migration.OperationCapability

const (
	CoverageUnknown          = migration.CoverageUnknown
	CoverageComplete         = migration.CoverageComplete
	CoveragePartial          = migration.CoveragePartial
	CoverageUnsupported      = migration.CoverageUnsupported
	AddForeignKey            = migration.AddForeignKey
	DropForeignKey           = migration.DropForeignKey
	AlterForeignKey          = migration.AlterForeignKey
	AddCheck                 = migration.AddCheck
	DropCheck                = migration.DropCheck
	AlterCheck               = migration.AlterCheck
	SchemaIdentityDifference = migration.SchemaIdentityDifference
)

// Preflight source and views remain separate from migration execution APIs.
type Preflight = migration.Preflight
type PreflightPlan = migration.PreflightPlan
type PhaseContract = migration.PhaseContract
type PreflightOptions = migration.PreflightOptions
type PreflightView = migration.PreflightView
type PreflightCollector = migration.Collector
