package sqlutils

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/stringutils"
)

// ExpandPositionalPlaceholders replaces positional placeholders found outside SQL
// literals/comments using the supplied placeholder factory.
func ExpandPositionalPlaceholders(sql string, expectedCount int, nextPlaceholder func() string) (string, error) {
	if expectedCount == 0 {
		return sql, nil
	}

	count := 0
	expanded, err := transformSQL(sql, func(src string, i int) (string, int, bool, error) {
		if src[i] != '?' {
			return "", 0, false, nil
		}

		count++
		return nextPlaceholder(), i + 1, true, nil
	})
	if err != nil {
		return "", err
	}
	if count != expectedCount {
		return "", fmt.Errorf("placeholder count does not match the number of arguments: %d != %d", count, expectedCount)
	}

	return expanded, nil
}

// ExpandNamedPlaceholders replaces named placeholders found outside SQL
// literals/comments while preserving appearance order and avoiding prefix
// collisions.
func ExpandNamedPlaceholders(sql string, values map[string]any, nextPlaceholder func() string) (string, []interface{}, error) {
	if len(values) == 0 {
		return sql, nil, nil
	}

	orderedValues := make([]interface{}, 0, len(values))
	expanded, err := transformSQL(sql, func(src string, i int) (string, int, bool, error) {
		if src[i] != ':' || i+1 >= len(src) || !isPlaceholderNameStart(src[i+1]) {
			return "", 0, false, nil
		}
		if i > 0 && (src[i-1] == ':' || isPlaceholderNameChar(src[i-1])) {
			return "", 0, false, nil
		}

		end := i + 2
		for end < len(src) && isPlaceholderNameChar(src[end]) {
			end++
		}

		name := src[i+1 : end]
		value, ok := values[name]
		if !ok {
			return "", 0, false, fmt.Errorf("missing named placeholder value: :%s", name)
		}

		orderedValues = append(orderedValues, value)
		return nextPlaceholder(), end, true, nil
	})
	if err != nil {
		return "", nil, err
	}

	return expanded, orderedValues, nil
}

// InlinePlaceholders replaces placeholder tokens with their formatted literal
// values for debugging output.
func InlinePlaceholders(sql string, args []interface{}) (string, error) {
	sequenceIndex := 0
	usedNumbered := make([]bool, len(args))

	inlined, err := transformSQL(sql, func(src string, i int) (string, int, bool, error) {
		switch src[i] {
		case '?':
			if sequenceIndex >= len(args) {
				return "", 0, false, fmt.Errorf("placeholder count does not match the number of arguments: %d != %d", sequenceIndex+1, len(args))
			}
			replacement, err := formatInlinePlaceholderValue(args[sequenceIndex])
			if err != nil {
				return "", 0, false, err
			}
			sequenceIndex++
			return replacement, i + 1, true, nil
		case '$':
			end := i + 1
			for end < len(src) && src[end] >= '0' && src[end] <= '9' {
				end++
			}
			if end == i+1 {
				return "", 0, false, nil
			}

			position, err := strconv.Atoi(src[i+1 : end])
			if err != nil {
				return "", 0, false, err
			}
			if position <= 0 || position > len(args) {
				return "", 0, false, fmt.Errorf("placeholder index out of range: $%d", position)
			}

			replacement, err := formatInlinePlaceholderValue(args[position-1])
			if err != nil {
				return "", 0, false, err
			}
			usedNumbered[position-1] = true
			return replacement, end, true, nil
		default:
			return "", 0, false, nil
		}
	})
	if err != nil {
		return "", err
	}

	if strings.IndexByte(sql, '?') >= 0 {
		if sequenceIndex != len(args) {
			return "", fmt.Errorf("placeholder count does not match the number of arguments: %d != %d", sequenceIndex, len(args))
		}
		return inlined, nil
	}

	usedCount := 0
	for _, used := range usedNumbered {
		if used {
			usedCount++
		}
	}
	if usedCount != len(args) {
		return "", fmt.Errorf("placeholder count does not match the number of arguments: %d != %d", usedCount, len(args))
	}

	return inlined, nil
}

func transformSQL(sql string, transform func(src string, i int) (string, int, bool, error)) (string, error) {
	var b strings.Builder
	b.Grow(len(sql))

	for i := 0; i < len(sql); {
		replacement, next, handled, err := transform(sql, i)
		if err != nil {
			return "", err
		}
		if handled {
			b.WriteString(replacement)
			i = next
			continue
		}

		switch sql[i] {
		case '\'':
			end := skipQuotedLiteral(sql, i, '\'')
			b.WriteString(sql[i:end])
			i = end
			continue
		case '"':
			end := skipQuotedLiteral(sql, i, '"')
			b.WriteString(sql[i:end])
			i = end
			continue
		case '`':
			end := skipQuotedLiteral(sql, i, '`')
			b.WriteString(sql[i:end])
			i = end
			continue
		case '-':
			if i+1 < len(sql) && sql[i+1] == '-' {
				end := skipLineComment(sql, i)
				b.WriteString(sql[i:end])
				i = end
				continue
			}
		case '/':
			if i+1 < len(sql) && sql[i+1] == '*' {
				end := skipBlockComment(sql, i)
				b.WriteString(sql[i:end])
				i = end
				continue
			}
		case '$':
			if end, ok := skipDollarQuotedLiteral(sql, i); ok {
				b.WriteString(sql[i:end])
				i = end
				continue
			}
		}

		b.WriteByte(sql[i])
		i++
	}

	return b.String(), nil
}

func skipQuotedLiteral(sql string, start int, quote byte) int {
	for i := start + 1; i < len(sql); i++ {
		switch sql[i] {
		case '\\':
			if i+1 < len(sql) {
				i++
			}
		case quote:
			if i+1 < len(sql) && sql[i+1] == quote {
				i++
				continue
			}
			return i + 1
		}
	}

	return len(sql)
}

func skipLineComment(sql string, start int) int {
	for i := start + 2; i < len(sql); i++ {
		if sql[i] == '\n' {
			return i + 1
		}
	}

	return len(sql)
}

func skipBlockComment(sql string, start int) int {
	for i := start + 2; i < len(sql)-1; i++ {
		if sql[i] == '*' && sql[i+1] == '/' {
			return i + 2
		}
	}

	return len(sql)
}

func skipDollarQuotedLiteral(sql string, start int) (int, bool) {
	tag, tagEnd, ok := parseDollarQuoteTag(sql, start)
	if !ok {
		return 0, false
	}

	closeOffset := strings.Index(sql[tagEnd:], tag)
	if closeOffset < 0 {
		return len(sql), true
	}

	return tagEnd + closeOffset + len(tag), true
}

func parseDollarQuoteTag(sql string, start int) (string, int, bool) {
	if sql[start] != '$' || start+1 >= len(sql) {
		return "", 0, false
	}

	if sql[start+1] == '$' {
		return "$$", start + 2, true
	}
	if !isPlaceholderNameStart(sql[start+1]) {
		return "", 0, false
	}

	end := start + 2
	for end < len(sql) && isPlaceholderNameChar(sql[end]) {
		end++
	}
	if end >= len(sql) || sql[end] != '$' {
		return "", 0, false
	}

	return sql[start : end+1], end + 1, true
}

func isPlaceholderNameStart(ch byte) bool {
	return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isPlaceholderNameChar(ch byte) bool {
	return isPlaceholderNameStart(ch) || (ch >= '0' && ch <= '9')
}

func formatInlinePlaceholderValue(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("'%s'", stringutils.EscapeString(v)), nil
	case []byte:
		return fmt.Sprintf("'%s'", stringutils.EscapeString(string(v))), nil
	case time.Time:
		return fmt.Sprintf("'%s'", v.Format(time.RFC3339Nano)), nil
	case bool:
		if v {
			return "TRUE", nil
		}
		return "FALSE", nil
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%v", v), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%v", v), nil
	case float32, float64:
		return fmt.Sprintf("%v", v), nil
	case nil:
		return "NULL", nil
	default:
		return "", fmt.Errorf("not supported type: %T", v)
	}
}
