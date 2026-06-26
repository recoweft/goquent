package orm

import (
	"time"

	"github.com/recoweft/goquent/orm/query"
)

type OperationType = query.OperationType
type RiskLevel = query.RiskLevel
type AnalysisPrecision = query.AnalysisPrecision
type SourceLocation = query.SourceLocation
type Evidence = query.Evidence
type Warning = query.Warning
type Approval = query.Approval
type Suppression = query.Suppression
type SuppressionScope = query.SuppressionScope
type SuppressionOption = query.SuppressionOption
type RiskEngine = query.RiskEngine
type RiskResult = query.RiskResult
type RiskConfig = query.RiskConfig
type RiskRuleConfig = query.RiskRuleConfig
type PolicyMode = query.PolicyMode
type TablePolicy = query.TablePolicy
type TableRef = query.TableRef
type ColumnRef = query.ColumnRef
type JoinRef = query.JoinRef
type PredicateRef = query.PredicateRef
type QueryPlan = query.QueryPlan

const (
	OperationSelect = query.OperationSelect
	OperationInsert = query.OperationInsert
	OperationUpdate = query.OperationUpdate
	OperationDelete = query.OperationDelete
	OperationRaw    = query.OperationRaw

	RiskLow         = query.RiskLow
	RiskMedium      = query.RiskMedium
	RiskHigh        = query.RiskHigh
	RiskDestructive = query.RiskDestructive
	RiskBlocked     = query.RiskBlocked

	AnalysisPrecise     = query.AnalysisPrecise
	AnalysisPartial     = query.AnalysisPartial
	AnalysisUnsupported = query.AnalysisUnsupported

	WarningRawSQLUsed               = query.WarningRawSQLUsed
	WarningUpdateWithoutWhere       = query.WarningUpdateWithoutWhere
	WarningDeleteWithoutWhere       = query.WarningDeleteWithoutWhere
	WarningSelectStarUsed           = query.WarningSelectStarUsed
	WarningLimitMissing             = query.WarningLimitMissing
	WarningBulkUpdateDetected       = query.WarningBulkUpdateDetected
	WarningBulkDeleteDetected       = query.WarningBulkDeleteDetected
	WarningDestructiveSQL           = query.WarningDestructiveSQL
	WarningWeakPredicate            = query.WarningWeakPredicate
	WarningSuppressionExpired       = query.WarningSuppressionExpired
	WarningSuppressionNotAllowed    = query.WarningSuppressionNotAllowed
	WarningStaticReviewPartial      = query.WarningStaticReviewPartial
	WarningStaticReviewUnsupported  = query.WarningStaticReviewUnsupported
	WarningRequiredPredicateMissing = query.WarningRequiredPredicateMissing
	WarningTenantFilterMissing      = query.WarningTenantFilterMissing
	WarningSoftDeleteFilterMissing  = query.WarningSoftDeleteFilterMissing
	WarningPIIColumnSelected        = query.WarningPIIColumnSelected
	WarningRequiredFilterMissing    = query.WarningRequiredFilterMissing

	SuppressionScopeQuery  = query.SuppressionScopeQuery
	SuppressionScopeInline = query.SuppressionScopeInline
	SuppressionScopeConfig = query.SuppressionScopeConfig

	PolicyModeWarn    = query.PolicyModeWarn
	PolicyModeEnforce = query.PolicyModeEnforce
	PolicyModeBlock   = query.PolicyModeBlock
)

var (
	ErrApprovalRequired       = query.ErrApprovalRequired
	ErrApprovalReasonRequired = query.ErrApprovalReasonRequired
	ErrAccessReasonRequired   = query.ErrAccessReasonRequired
	ErrBlockedOperation       = query.ErrBlockedOperation
	DefaultRiskEngine         = query.DefaultRiskEngine
)

func NewSuppression(code, reason string, opts ...SuppressionOption) (Suppression, error) {
	return query.NewSuppression(code, reason, opts...)
}

func SuppressionExpiresAt(t time.Time) SuppressionOption {
	return query.SuppressionExpiresAt(t)
}

func SuppressionOwner(owner string) SuppressionOption {
	return query.SuppressionOwner(owner)
}

func ParseInlineSuppression(comment string) (Suppression, bool, error) {
	return query.ParseInlineSuppression(comment)
}

func NewRiskEngine(config RiskConfig) RiskEngine {
	return query.NewRiskEngine(config)
}

func EnsurePlanExecutable(plan *QueryPlan) error {
	return query.EnsurePlanExecutable(plan)
}

func PlanHasPredicateColumn(plan *QueryPlan, table, column string) bool {
	return query.PlanHasPredicateColumn(plan, table, column)
}

func MissingRequiredPredicates(plan *QueryPlan, required []RequiredPredicate) []RequiredPredicate {
	return query.MissingRequiredPredicates(plan, required)
}
