package migration

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/recoweft/goquent/orm/driver"
)

// StatusExecutor is the read surface used by ReadStatus.
type StatusExecutor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// StatusOptions configures the migration status table reader.
type StatusOptions struct {
	Table           string
	VersionColumn   string
	DirtyColumn     string
	AppliedAtColumn string
}

// StatusOption configures ReadStatus.
type StatusOption func(*StatusOptions)

// AppliedMigration is one row read from the migration status table.
type AppliedMigration struct {
	Version   string     `json:"version"`
	AppliedAt *time.Time `json:"applied_at,omitempty"`
	Dirty     bool       `json:"dirty,omitempty"`
}

// Status is a lightweight best-effort view of migration table state.
type Status struct {
	Table         string             `json:"table"`
	Exists        bool               `json:"exists"`
	Applied       []AppliedMigration `json:"applied,omitempty"`
	LatestApplied string             `json:"latest_applied,omitempty"`
	Pending       []string           `json:"pending,omitempty"`
	Dirty         bool               `json:"dirty,omitempty"`
	Unknown       bool               `json:"unknown,omitempty"`
	Warnings      []string           `json:"warnings,omitempty"`
}

// WithStatusTable sets the migration table name. The default is
// schema_migrations.
func WithStatusTable(table string) StatusOption {
	return func(o *StatusOptions) { o.Table = table }
}

// WithStatusVersionColumn sets the version column. The default is version.
func WithStatusVersionColumn(column string) StatusOption {
	return func(o *StatusOptions) { o.VersionColumn = column }
}

// WithStatusDirtyColumn enables dirty-state detection from column.
func WithStatusDirtyColumn(column string) StatusOption {
	return func(o *StatusOptions) { o.DirtyColumn = column }
}

// WithStatusAppliedAtColumn reads an optional applied-at timestamp column.
func WithStatusAppliedAtColumn(column string) StatusOption {
	return func(o *StatusOptions) { o.AppliedAtColumn = column }
}

// ReadStatus reads the migration table state and compares it with desired
// versions supplied by the caller. It is intended for readiness checks, not as
// a complete schema drift detector.
func ReadStatus(ctx context.Context, exec StatusExecutor, dialect driver.Dialect, desired []string, opts ...StatusOption) (Status, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg := defaultStatusOptions()
	for _, opt := range opts {
		opt(&cfg)
	}
	cfg.normalize()
	status := Status{Table: cfg.Table}
	desired = normalizeDesiredVersions(desired)
	if exec == nil {
		return status, fmt.Errorf("goquent: migration status executor is required")
	}
	if dialect == nil {
		return status, fmt.Errorf("goquent: migration status dialect is required")
	}

	exists, err := migrationStatusTableExists(ctx, exec, dialect, cfg.Table)
	if err != nil {
		return status, err
	}
	status.Exists = exists
	if !exists {
		status.Pending = desired
		return status, nil
	}

	applied, err := readAppliedMigrations(ctx, exec, dialect, cfg)
	if err != nil {
		return status, err
	}
	sort.SliceStable(applied, func(i, j int) bool {
		return applied[i].Version < applied[j].Version
	})
	status.Applied = applied
	if len(applied) > 0 {
		status.LatestApplied = applied[len(applied)-1].Version
	}
	for _, row := range applied {
		if row.Dirty {
			status.Dirty = true
			break
		}
	}
	status.Pending = pendingMigrationVersions(desired, applied)
	status.Warnings = migrationStatusWarnings(desired, applied)
	if len(status.Warnings) > 0 {
		status.Unknown = true
	}
	return status, nil
}

func defaultStatusOptions() StatusOptions {
	return StatusOptions{
		Table:         "schema_migrations",
		VersionColumn: "version",
	}
}

func (o *StatusOptions) normalize() {
	o.Table = strings.TrimSpace(o.Table)
	if o.Table == "" {
		o.Table = "schema_migrations"
	}
	o.VersionColumn = strings.TrimSpace(o.VersionColumn)
	if o.VersionColumn == "" {
		o.VersionColumn = "version"
	}
	o.DirtyColumn = strings.TrimSpace(o.DirtyColumn)
	o.AppliedAtColumn = strings.TrimSpace(o.AppliedAtColumn)
}

func migrationStatusTableExists(ctx context.Context, exec StatusExecutor, dialect driver.Dialect, table string) (bool, error) {
	switch dialect.(type) {
	case driver.PostgresDialect:
		return queryStatusBool(ctx, exec, "SELECT to_regclass($1) IS NOT NULL", table)
	case driver.MySQLDialect:
		schema, name := splitMySQLStatusTable(table)
		if schema == "" {
			return queryStatusCount(ctx, exec, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?", name)
		}
		return queryStatusCount(ctx, exec, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ? AND table_name = ?", schema, name)
	default:
		return false, fmt.Errorf("goquent: migration status is not supported on dialect: %T", dialect)
	}
}

func queryStatusBool(ctx context.Context, exec StatusExecutor, sqlStr string, args ...any) (bool, error) {
	rows, err := exec.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return false, rows.Err()
	}
	var exists bool
	if err := rows.Scan(&exists); err != nil {
		return false, err
	}
	return exists, rows.Err()
}

func queryStatusCount(ctx context.Context, exec StatusExecutor, sqlStr string, args ...any) (bool, error) {
	rows, err := exec.QueryContext(ctx, sqlStr, args...)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return false, rows.Err()
	}
	var count int64
	if err := rows.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, rows.Err()
}

func readAppliedMigrations(ctx context.Context, exec StatusExecutor, dialect driver.Dialect, cfg StatusOptions) ([]AppliedMigration, error) {
	versionSQL, err := quoteStatusIdentifierPath(dialect, cfg.VersionColumn)
	if err != nil {
		return nil, err
	}
	selectCols := []string{versionSQL}
	if cfg.AppliedAtColumn != "" {
		col, err := quoteStatusIdentifierPath(dialect, cfg.AppliedAtColumn)
		if err != nil {
			return nil, err
		}
		selectCols = append(selectCols, col)
	}
	if cfg.DirtyColumn != "" {
		col, err := quoteStatusIdentifierPath(dialect, cfg.DirtyColumn)
		if err != nil {
			return nil, err
		}
		selectCols = append(selectCols, col)
	}
	tableSQL, err := quoteStatusIdentifierPath(dialect, cfg.Table)
	if err != nil {
		return nil, err
	}
	sqlStr := fmt.Sprintf("SELECT %s FROM %s ORDER BY %s ASC", strings.Join(selectCols, ", "), tableSQL, versionSQL)
	rows, err := exec.QueryContext(ctx, sqlStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var applied []AppliedMigration
	for rows.Next() {
		var versionRaw any
		scanDst := []any{&versionRaw}
		var appliedAtRaw any
		var dirtyRaw any
		if cfg.AppliedAtColumn != "" {
			scanDst = append(scanDst, &appliedAtRaw)
		}
		if cfg.DirtyColumn != "" {
			scanDst = append(scanDst, &dirtyRaw)
		}
		if err := rows.Scan(scanDst...); err != nil {
			return nil, err
		}
		row := AppliedMigration{Version: statusString(versionRaw)}
		if cfg.AppliedAtColumn != "" {
			appliedAt, err := statusTime(appliedAtRaw)
			if err != nil {
				return nil, err
			}
			row.AppliedAt = appliedAt
		}
		if cfg.DirtyColumn != "" {
			dirty, err := statusBool(dirtyRaw)
			if err != nil {
				return nil, err
			}
			row.Dirty = dirty
		}
		applied = append(applied, row)
	}
	return applied, rows.Err()
}

func statusString(src any) string {
	switch v := src.(type) {
	case nil:
		return ""
	case []byte:
		return string(v)
	case string:
		return v
	default:
		return fmt.Sprint(v)
	}
}

func statusBool(src any) (bool, error) {
	switch v := src.(type) {
	case nil:
		return false, nil
	case bool:
		return v, nil
	case int8:
		return v != 0, nil
	case int16:
		return v != 0, nil
	case int32:
		return v != 0, nil
	case int64:
		return v != 0, nil
	case int:
		return v != 0, nil
	case uint8:
		return v != 0, nil
	case uint16:
		return v != 0, nil
	case uint32:
		return v != 0, nil
	case uint64:
		return v != 0, nil
	case uint:
		return v != 0, nil
	case []byte:
		return statusBoolString(string(v))
	case string:
		return statusBoolString(v)
	default:
		return false, fmt.Errorf("goquent: cannot convert migration dirty value %T to bool", src)
	}
}

func statusBoolString(s string) (bool, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "", "0", "false", "f", "no", "n":
		return false, nil
	case "1", "true", "t", "yes", "y":
		return true, nil
	default:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return false, fmt.Errorf("goquent: cannot convert migration dirty value %q to bool", s)
		}
		return n != 0, nil
	}
}

func statusTime(src any) (*time.Time, error) {
	switch v := src.(type) {
	case nil:
		return nil, nil
	case time.Time:
		t := v
		return &t, nil
	case []byte:
		return parseStatusTime(string(v))
	case string:
		return parseStatusTime(v)
	default:
		return nil, fmt.Errorf("goquent: cannot convert migration applied-at value %T to time", src)
	}
}

func parseStatusTime(s string) (*time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05"} {
		t, err := time.Parse(layout, s)
		if err == nil {
			return &t, nil
		}
	}
	return nil, fmt.Errorf("goquent: cannot parse migration applied-at value %q", s)
}

func normalizeDesiredVersions(desired []string) []string {
	seen := make(map[string]struct{}, len(desired))
	out := make([]string, 0, len(desired))
	for _, version := range desired {
		version = strings.TrimSpace(version)
		if version == "" {
			continue
		}
		if _, ok := seen[version]; ok {
			continue
		}
		seen[version] = struct{}{}
		out = append(out, version)
	}
	return out
}

func pendingMigrationVersions(desired []string, applied []AppliedMigration) []string {
	appliedSet := make(map[string]struct{}, len(applied))
	for _, row := range applied {
		appliedSet[row.Version] = struct{}{}
	}
	pending := make([]string, 0, len(desired))
	for _, version := range desired {
		if _, ok := appliedSet[version]; !ok {
			pending = append(pending, version)
		}
	}
	return pending
}

func migrationStatusWarnings(desired []string, applied []AppliedMigration) []string {
	if len(desired) == 0 {
		return nil
	}
	desiredSet := make(map[string]struct{}, len(desired))
	for _, version := range desired {
		desiredSet[version] = struct{}{}
	}
	var warnings []string
	for _, row := range applied {
		if _, ok := desiredSet[row.Version]; ok {
			continue
		}
		warnings = append(warnings, fmt.Sprintf("applied migration %s is not present in desired versions; status is best-effort and drift is unknown", row.Version))
	}
	return warnings
}

func splitMySQLStatusTable(table string) (string, string) {
	parts := strings.Split(table, ".")
	if len(parts) == 1 {
		return "", strings.TrimSpace(parts[0])
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[len(parts)-1])
}

func quoteStatusIdentifierPath(dialect driver.Dialect, ident string) (string, error) {
	parts := strings.Split(ident, ".")
	if len(parts) == 0 {
		return "", fmt.Errorf("goquent: identifier path is required")
	}
	quoted := make([]string, len(parts))
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return "", fmt.Errorf("goquent: identifier path contains an empty part")
		}
		quoted[i] = dialect.QuoteIdent(part)
	}
	return strings.Join(quoted, "."), nil
}
