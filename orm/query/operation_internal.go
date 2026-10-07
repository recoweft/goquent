package query

import "github.com/recoweft/goquent/orm/internal/querybridge"

func init() {
	querybridge.AutomaticTenant = func(v any) bool { s, ok := v.(Settings); return ok && s.autoTenant }
	querybridge.ConfigureOperation = func(v any, r querybridge.OperationValidation) error {
		q, ok := v.(*Query)
		if !ok || q == nil {
			return ErrBlockedOperation
		}
		r.Checks = append([]querybridge.OperationCheck(nil), r.Checks...)
		r.Unknown = append([]string(nil), r.Unknown...)
		r.WarningCodes = append([]string(nil), r.WarningCodes...)
		q.operationValidation = &r
		return nil
	}
}

func (q *Query) finalizeOperation(p *QueryPlan) {
	r := q.operationValidation
	if r == nil {
		return
	}
	p.operationValidation = r
	for _, code := range r.WarningCodes {
		found := false
		for _, w := range p.Warnings {
			found = found || w.Code == code
		}
		if !found {
			level := RiskMedium
			if code == "MANIFEST_STALE" || code == "OPERATION_SPEC_REQUIRED_FILTER_MISSING" {
				level = RiskHigh
			}
			p.Warnings = append(p.Warnings, Warning{Code: code, Level: level, Message: "operation validation warning"})
		}
	}
	if len(r.Unknown) > 0 {
		p.Unverified = append(p.Unverified, r.Unknown...)
		p.AnalysisPrecision = AnalysisPartial
		p.Warnings = append(p.Warnings, Warning{Code: "OPERATION_TYPE_UNVERIFIED", Level: RiskBlocked, Message: "operation type verification is incomplete"})
	}
	level, blocked := aggregateWarnings(p.Warnings)
	if riskRank(level) > riskRank(p.RiskLevel) {
		p.RiskLevel = level
	}
	p.Blocked = p.Blocked || blocked
	p.RequiredApproval = requiresApprovalLevel(p.RiskLevel)
}
