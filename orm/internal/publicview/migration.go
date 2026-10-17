package publicview

// MigrationStep is the shared closed vocabulary for migration display consumers.
func MigrationStep(s string) string {
	switch s {
	case "add_table", "drop_table", "add_column", "drop_column", "rename_column", "alter_column_type", "alter_nullability", "alter_index", "alter_column_default", "add_index", "drop_index", "unsupported", "add_foreign_key", "drop_foreign_key", "alter_foreign_key", "add_check", "drop_check", "alter_check", "schema_identity_difference":
		return s
	}
	return "unknown"
}
