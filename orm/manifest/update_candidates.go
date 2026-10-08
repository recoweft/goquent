package manifest

// RepositoryUpdateCandidate reports supplied update metadata only. Runtime
// settings and write policy checks remain mandatory, including for dynamic input.
func RepositoryUpdateCandidate(t Table, c Column) bool {
	if c.Primary || c.Readonly || c.Generated || c.PII || c.Forbidden || c.TenantScope || c.SoftDelete {
		return false
	}
	for _, p := range t.Policies {
		if normalizeName(p.Column) != normalizeName(c.Name) {
			continue
		}
		switch p.Type {
		case "tenant_scope", "soft_delete", "forbidden", "protected", "immutable", "readonly", "generated":
			return false
		}
	}
	return true
}
