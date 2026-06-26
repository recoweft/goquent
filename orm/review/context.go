package review

import (
	"strings"

	"github.com/recoweft/goquent/orm/manifest"
	"github.com/recoweft/goquent/orm/query"
)

type reviewContext struct {
	manifest             *manifest.Manifest
	riskMetadata         []query.TableRiskMetadata
	tables               map[string]manifest.Table
	rules                map[string]query.RiskRuleConfig
	configSuppressions   []ConfigSuppression
	registeredSoftDelete map[string]string
}

func newReviewContext(opts Options) reviewContext {
	m := opts.CurrentManifest
	if m == nil && strings.TrimSpace(opts.ManifestPath) != "" {
		loaded, err := manifest.Load(opts.ManifestPath)
		if err == nil {
			m = loaded
		}
	}
	ctx := reviewContext{
		manifest:           m,
		rules:              opts.Rules,
		configSuppressions: append([]ConfigSuppression(nil), opts.ConfigSuppressions...),
	}
	if m == nil {
		return ctx
	}
	ctx.tables = make(map[string]manifest.Table, len(m.Tables))
	for _, table := range m.Tables {
		ctx.tables[normalizeReviewName(table.Name)] = table
	}
	ctx.riskMetadata = tableRiskMetadataFromManifest(m)
	return ctx
}

func tableRiskMetadataFromManifest(m *manifest.Manifest) []query.TableRiskMetadata {
	if m == nil {
		return nil
	}
	out := make([]query.TableRiskMetadata, 0, len(m.Tables))
	for _, table := range m.Tables {
		meta := query.TableRiskMetadata{Table: table.Name}
		for _, column := range table.Columns {
			if column.Primary {
				meta.PrimaryKeyColumns = append(meta.PrimaryKeyColumns, column.Name)
			}
		}
		for _, index := range table.Indexes {
			if index.Unique && len(index.Columns) > 0 {
				meta.UniqueIndexes = append(meta.UniqueIndexes, append([]string(nil), index.Columns...))
			}
		}
		for _, policy := range table.Policies {
			switch policy.Type {
			case "tenant_scope":
				meta.TenantColumn = policy.Column
			case "soft_delete":
				meta.SoftDeleteColumn = policy.Column
			case "required_filter":
				meta.RequiredFilterColumns = append(meta.RequiredFilterColumns, policy.Column)
			}
		}
		for _, column := range table.Columns {
			if column.TenantScope && meta.TenantColumn == "" {
				meta.TenantColumn = column.Name
			}
			if column.SoftDelete && meta.SoftDeleteColumn == "" {
				meta.SoftDeleteColumn = column.Name
			}
			if column.RequiredFilter && !containsReviewColumn(meta.RequiredFilterColumns, column.Name) {
				meta.RequiredFilterColumns = append(meta.RequiredFilterColumns, column.Name)
			}
		}
		out = append(out, meta)
	}
	return out
}

func (ctx reviewContext) table(name string) (manifest.Table, bool) {
	if ctx.tables == nil {
		return manifest.Table{}, false
	}
	table, ok := ctx.tables[normalizeReviewName(name)]
	return table, ok
}

func normalizeReviewName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`\"")
	if idx := strings.LastIndex(s, "."); idx >= 0 {
		s = s[idx+1:]
	}
	return strings.ToLower(s)
}

func containsReviewColumn(cols []string, col string) bool {
	target := normalizeReviewName(col)
	for _, existing := range cols {
		if normalizeReviewName(existing) == target {
			return true
		}
	}
	return false
}
