package writeinput

import (
	"fmt"
	"github.com/recoweft/goquent/orm/driver"
	"strings"
)

type AssignmentKind int

const (
	Raw AssignmentKind = iota
	Expr
	Column
	Increment
)

type Assignment struct {
	Column       string
	Expression   string
	Args         []any
	SourceColumn string
	Kind         AssignmentKind
}

func quote(d driver.Dialect, ident string) string { return d.QuoteIdent(ident) }

func quoteIdentifierPath(d driver.Dialect, ident string) (string, error) {
	return quoteIdentifierPathParts(d, strings.Split(ident, "."))
}

func quoteIdentifierPathParts(d driver.Dialect, parts []string) (string, error) {
	if len(parts) == 0 {
		return "", fmt.Errorf("goquent: identifier path is required")
	}
	quoted := make([]string, len(parts))
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return "", fmt.Errorf("goquent: identifier path contains an empty part")
		}
		quoted[i] = quote(d, part)
	}
	return strings.Join(quoted, "."), nil
}

func ValidateFragment(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fmt.Errorf("goquent: raw SQL fragment is required")
	}
	if strings.ContainsAny(trimmed, ";\x00") ||
		strings.Contains(trimmed, "--") ||
		strings.Contains(trimmed, "/*") ||
		strings.Contains(trimmed, "*/") {
		return fmt.Errorf("goquent: raw SQL fragment contains a statement separator or comment")
	}
	upper := strings.ToUpper(trimmed)
	for _, token := range []string{"ALTER", "CREATE", "DELETE", "DROP", "GRANT", "INSERT", "REVOKE", "TRUNCATE", "UPDATE"} {
		if containsSQLWord(upper, token) {
			return fmt.Errorf("goquent: raw SQL fragment contains disallowed SQL token %q", token)
		}
	}
	return nil
}

func Targets(assignments []Assignment) map[string]struct{} {
	targets := make(map[string]struct{}, len(assignments))
	for _, assignment := range assignments {
		column := strings.TrimSpace(assignment.Column)
		if column == "" {
			continue
		}
		targets[column] = struct{}{}
	}
	return targets
}

func ValidateAssignments(assignments []Assignment) error {
	seen := make(map[string]struct{}, len(assignments))
	for _, assignment := range assignments {
		column := strings.TrimSpace(assignment.Column)
		if column == "" {
			return fmt.Errorf("goquent: assignment column is required")
		}
		if _, ok := seen[column]; ok {
			return fmt.Errorf("goquent: duplicate assignment for column %s", column)
		}
		seen[column] = struct{}{}
	}
	return nil
}

func SetParts(d driver.Dialect, setCols []string, setArgs []any, assignments []Assignment, start int) ([]string, []any, error) {
	if err := ValidateAssignments(assignments); err != nil {
		return nil, nil, err
	}
	setParts := make([]string, 0, len(setCols)+len(assignments))
	args := make([]any, 0, len(setArgs)+len(assignments))
	argPos := start
	for i, col := range setCols {
		setParts = append(setParts, fmt.Sprintf("%s=%s", quote(d, col), d.Placeholder(argPos)))
		args = append(args, setArgs[i])
		argPos++
	}
	for _, assignment := range assignments {
		target, err := quoteIdentifierPath(d, assignment.Column)
		if err != nil {
			return nil, nil, err
		}
		expr, exprArgs, err := RenderAssignment(d, assignment, argPos)
		if err != nil {
			return nil, nil, err
		}
		setParts = append(setParts, fmt.Sprintf("%s=%s", target, expr))
		args = append(args, exprArgs...)
		argPos += len(exprArgs)
	}
	return setParts, args, nil
}

func RenderAssignment(d driver.Dialect, assignment Assignment, start int) (string, []any, error) {
	switch assignment.Kind {
	case Raw:
		if len(assignment.Args) > 0 {
			return "", nil, fmt.Errorf("goquent: SetRaw does not accept args")
		}
		expr := strings.TrimSpace(assignment.Expression)
		if err := ValidateFragment(expr); err != nil {
			return "", nil, err
		}
		if strings.Contains(expr, "?") {
			return "", nil, fmt.Errorf("goquent: SetRaw expression contains placeholders; use SetExpr")
		}
		return expr, nil, nil
	case Expr:
		expr := strings.TrimSpace(assignment.Expression)
		if err := ValidateFragment(expr); err != nil {
			return "", nil, err
		}
		rendered, err := RenderPlaceholders(d, expr, len(assignment.Args), start)
		if err != nil {
			return "", nil, err
		}
		return rendered, append([]any(nil), assignment.Args...), nil
	case Column:
		if len(assignment.Args) > 0 {
			return "", nil, fmt.Errorf("goquent: SetColumn does not accept args")
		}
		expr, err := quoteIdentifierPath(d, assignment.SourceColumn)
		return expr, nil, err
	case Increment:
		if len(assignment.Args) != 1 {
			return "", nil, fmt.Errorf("goquent: Increment requires one delta arg")
		}
		target, err := quoteIdentifierPath(d, assignment.Column)
		if err != nil {
			return "", nil, err
		}
		return fmt.Sprintf("%s + %s", target, d.Placeholder(start)), append([]any(nil), assignment.Args...), nil
	default:
		return "", nil, fmt.Errorf("goquent: unknown assignment kind")
	}
}

func RenderPlaceholders(d driver.Dialect, expr string, argCount int, start int) (string, error) {
	if strings.Count(expr, "?") != argCount {
		return "", fmt.Errorf("goquent: SetExpr placeholder count does not match args")
	}
	if argCount == 0 {
		return expr, nil
	}
	var b strings.Builder
	b.Grow(len(expr) + argCount*2)
	argIndex := 0
	for i := 0; i < len(expr); i++ {
		if expr[i] != '?' {
			b.WriteByte(expr[i])
			continue
		}
		b.WriteString(d.Placeholder(start + argIndex))
		argIndex++
	}
	return b.String(), nil
}

func containsSQLWord(upperSQL, token string) bool {
	for i := 0; i+len(token) <= len(upperSQL); i++ {
		if upperSQL[i:i+len(token)] != token {
			continue
		}
		beforeOK := i == 0 || !isSQLWordByte(upperSQL[i-1])
		after := i + len(token)
		afterOK := after >= len(upperSQL) || !isSQLWordByte(upperSQL[after])
		if beforeOK && afterOK {
			return true
		}
	}
	return false
}

func isSQLWordByte(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}
