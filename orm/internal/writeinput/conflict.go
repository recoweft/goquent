package writeinput

import (
	"fmt"
	"github.com/recoweft/goquent/orm/driver"
	"strings"
)

func AppendConflict(d driver.Dialect, sql string, c *Conflict, assignments []Assignment, start int, literal bool, reference func(string) string) (string, []any, error) {
	parts, args, err := SetParts(d, nil, nil, assignments, start)
	if err != nil {
		return "", nil, err
	}
	quoteColumn := d.QuoteIdent
	if !literal && reference != nil {
		quoteColumn = reference
	}
	sep := "="
	if !literal {
		sep = " = "
	}
	switch d.(type) {
	case driver.MySQLDialect:
		if strings.TrimSpace(c.Where) != "" || strings.TrimSpace(c.Constraint) != "" || strings.TrimSpace(c.RawTarget) != "" {
			return "", nil, fmt.Errorf("ConflictWhere, ConflictConstraint, and ConflictTargetRaw are not supported on dialect: %T", d)
		}
		var updates []string
		for _, col := range c.Updates {
			updates = append(updates, quoteColumn(col)+sep+"VALUES("+quoteColumn(col)+")")
		}
		updates = append(updates, parts...)
		if len(updates) == 0 {
			return strings.Replace(sql, "INSERT", "INSERT IGNORE", 1), args, nil
		}
		return sql + " ON DUPLICATE KEY UPDATE " + strings.Join(updates, ", "), args, nil
	case driver.PostgresDialect:
		target, err := conflictTarget(d, c.Columns, c, quoteColumn)
		if err != nil {
			return "", nil, err
		}
		var updates []string
		for _, col := range c.Updates {
			updates = append(updates, quoteColumn(col)+sep+"EXCLUDED."+quoteColumn(col))
		}
		updates = append(updates, parts...)
		if len(updates) == 0 {
			return sql + " ON CONFLICT " + target + " DO NOTHING", args, nil
		}
		return sql + " ON CONFLICT " + target + " DO UPDATE SET " + strings.Join(updates, ", "), args, nil
	default:
		return "", nil, fmt.Errorf("upsert not supported on dialect: %T", d)
	}
}
func conflictTarget(d driver.Dialect, cols []string, o *Conflict, quoteColumn func(string) string) (string, error) {
	rawTarget := strings.TrimSpace(o.RawTarget)
	constraint := strings.TrimSpace(o.Constraint)
	predicate := strings.TrimSpace(o.Where)
	if rawTarget != "" {
		if o.ExplicitColumns {
			return "", fmt.Errorf("ConflictTargetRaw cannot be combined with ConflictColumns")
		}
		if constraint != "" {
			return "", fmt.Errorf("ConflictTargetRaw cannot be combined with ConflictConstraint")
		}
		if predicate != "" {
			return "", fmt.Errorf("ConflictTargetRaw cannot be combined with ConflictWhere")
		}
		if err := ValidateFragment(rawTarget); err != nil {
			return "", err
		}
		return rawTarget, nil
	}
	if constraint != "" {
		if o.ExplicitColumns {
			return "", fmt.Errorf("ConflictConstraint cannot be combined with ConflictColumns")
		}
		if predicate != "" {
			return "", fmt.Errorf("ConflictConstraint cannot be combined with ConflictWhere")
		}
		return "ON CONSTRAINT " + quote(d, constraint), nil
	}
	if len(cols) == 0 {
		return "", fmt.Errorf("Postgres upsert requires ConflictColumns or WherePK primary key columns")
	}
	quoted := make([]string, len(cols))
	for i, col := range cols {
		quoted[i] = quoteColumn(col)
	}
	target := "(" + strings.Join(quoted, ", ") + ")"
	if predicate != "" {
		if err := ValidateFragment(predicate); err != nil {
			return "", err
		}
		target += " WHERE " + predicate
	}
	return target, nil
}
