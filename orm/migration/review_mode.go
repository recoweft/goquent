package migration

import (
	"fmt"
	"strings"

	"github.com/recoweft/goquent/orm/query"
)

// ApplyReviewMode annotates plan with additional review guidance.
func ApplyReviewMode(plan *MigrationPlan, mode ReviewMode) error {
	if plan == nil {
		return nil
	}
	mode = ReviewMode(strings.ToLower(strings.TrimSpace(string(mode))))
	if mode == "" {
		return nil
	}
	switch mode {
	case ReviewModeBackfill:
		applyBackfillReviewMode(plan)
	default:
		return fmt.Errorf("goquent: unsupported migration review mode %q", mode)
	}
	return nil
}

func applyBackfillReviewMode(plan *MigrationPlan) {
	if plan.Metadata == nil {
		plan.Metadata = map[string]any{}
	}
	plan.Metadata["review_mode"] = string(ReviewModeBackfill)
	for i := range plan.Steps {
		warnings, preflight := backfillReviewForStep(plan.Steps[i])
		if len(warnings) == 0 && len(preflight) == 0 {
			continue
		}
		plan.Steps[i].Warnings = append(plan.Steps[i].Warnings, warnings...)
		plan.Steps[i].Preflight = appendUniqueStrings(plan.Steps[i].Preflight, preflight...)
		plan.Warnings = append(plan.Warnings, warnings...)
	}
	plan.RiskLevel, plan.Blocked = aggregateWarnings(plan.Warnings)
	plan.RequiredApproval = requiresApproval(plan.RiskLevel)
}

func backfillReviewForStep(step MigrationStep) ([]query.Warning, []string) {
	switch step.Type {
	case AddColumn:
		nullable := true
		if step.Nullable != nil {
			nullable = *step.Nullable
		}
		if nullable {
			return nil, []string{
				"document whether the new column needs a backfill before application code depends on it",
			}
		}
		return []query.Warning{newWarning(
				WarningMigrationBackfillReview,
				query.RiskMedium,
				"backfill review mode requires evidence for a NOT NULL column rollout",
				"prefer add nullable, backfill in batches, then enforce NOT NULL after evidence is collected",
				true,
				step.Line,
			)}, []string{
				"record the backfill query, batch size, retry behavior, and expected runtime",
				"attach before/after row counts or NULL-count evidence before enforcing NOT NULL",
				"confirm the application can tolerate the column during the expand/backfill/contract window",
			}
	case AlterNullability:
		nullable := true
		if step.Nullable != nil {
			nullable = *step.Nullable
		}
		if nullable {
			return nil, nil
		}
		return []query.Warning{newWarning(
				WarningMigrationBackfillReview,
				query.RiskMedium,
				"backfill review mode requires evidence before enforcing NOT NULL",
				"attach a zero-NULL proof or a completed backfill record before applying",
				true,
				step.Line,
			)}, []string{
				"run and record a zero-NULL verification query for the target column",
				"confirm any backfill job completed successfully before enforcing NOT NULL",
			}
	case AlterColumnType, RenameColumn, DropColumn:
		return []query.Warning{newWarning(
				WarningMigrationBackfillReview,
				query.RiskMedium,
				"backfill review mode requires rollout evidence for shape-changing migrations",
				"use expand/backfill/contract and attach compatibility evidence before applying",
				true,
				step.Line,
			)}, []string{
				"confirm read/write compatibility across old and new application versions",
				"record backfill or dual-write verification evidence before the contract step",
			}
	default:
		return nil, nil
	}
}

func appendUniqueStrings(dst []string, values ...string) []string {
	seen := make(map[string]struct{}, len(dst)+len(values))
	for _, value := range dst {
		seen[value] = struct{}{}
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		dst = append(dst, value)
	}
	return dst
}
