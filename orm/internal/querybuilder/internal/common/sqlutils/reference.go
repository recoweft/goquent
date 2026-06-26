package sqlutils

import "strings"

type ParsedReference struct {
	Parts []string
	Alias string
}

func ParseRelationReference(value string) (ParsedReference, bool) {
	return parseAliasedReference(strings.TrimSpace(value), 2, false)
}

func ParseReference(value string) (ParsedReference, bool) {
	parts, ok := parseQualifiedReference(strings.TrimSpace(value), 3, true)
	if !ok {
		return ParsedReference{}, false
	}

	return ParsedReference{Parts: parts}, true
}

func ParseAliasedValue(value string) (ParsedReference, bool) {
	return parseAliasedReference(strings.TrimSpace(value), 3, true)
}

func RelationSelectReference(value string) string {
	trimmed := strings.TrimSpace(value)
	ref, ok := ParseRelationReference(trimmed)
	if !ok {
		return trimmed
	}
	if ref.Alias != "" {
		return ref.Alias
	}
	return strings.Join(ref.Parts, ".")
}

func AppendEscapedRelation(sb []byte, value string, quote byte) []byte {
	trimmed := strings.TrimSpace(value)
	ref, ok := ParseRelationReference(trimmed)
	if !ok {
		return appendQuotedIdentifier(sb, trimmed, quote)
	}

	sb = appendEscapedQualifiedReference(sb, ref.Parts, quote)
	if ref.Alias == "" {
		return sb
	}

	sb = append(sb, " as "...)
	return appendQuotedIdentifier(sb, ref.Alias, quote)
}

func AppendEscapedReference(sb []byte, value string, quote byte) []byte {
	trimmed := strings.TrimSpace(value)
	ref, ok := ParseReference(trimmed)
	if !ok {
		return appendQuotedIdentifier(sb, trimmed, quote)
	}

	return appendEscapedQualifiedReference(sb, ref.Parts, quote)
}

func AppendEscapedAliasedValue(sb []byte, value string, quote byte) []byte {
	trimmed := strings.TrimSpace(value)
	ref, ok := ParseAliasedValue(trimmed)
	if !ok {
		return appendQuotedIdentifier(sb, trimmed, quote)
	}

	sb = appendEscapedQualifiedReference(sb, ref.Parts, quote)
	if ref.Alias == "" {
		return sb
	}

	sb = append(sb, " as "...)
	return appendQuotedIdentifier(sb, ref.Alias, quote)
}

func appendEscapedQualifiedReference(sb []byte, parts []string, quote byte) []byte {
	for i, part := range parts {
		if i > 0 {
			sb = append(sb, '.')
		}
		if part == "*" {
			sb = append(sb, '*')
			continue
		}
		sb = appendQuotedIdentifier(sb, part, quote)
	}
	return sb
}

func appendQuotedIdentifier(sb []byte, value string, quote byte) []byte {
	sb = append(sb, quote)
	for i := 0; i < len(value); i++ {
		sb = append(sb, value[i])
		if value[i] == quote {
			sb = append(sb, quote)
		}
	}
	sb = append(sb, quote)
	return sb
}

func parseAliasedReference(value string, maxParts int, allowWildcard bool) (ParsedReference, bool) {
	if value == "" {
		return ParsedReference{}, false
	}

	tokens, ok := splitTopLevelWhitespace(value)
	if !ok || len(tokens) == 0 {
		return ParsedReference{}, false
	}

	if len(tokens) != 1 && len(tokens) != 3 {
		return ParsedReference{}, false
	}

	parts, ok := parseQualifiedReference(tokens[0], maxParts, allowWildcard)
	if !ok {
		return ParsedReference{}, false
	}

	ref := ParsedReference{Parts: parts}
	if len(tokens) == 1 {
		return ref, true
	}

	if !asciiEqualFold(tokens[1], "as") {
		return ParsedReference{}, false
	}

	aliasParts, ok := parseQualifiedReference(tokens[2], 1, false)
	if !ok || len(aliasParts) != 1 {
		return ParsedReference{}, false
	}
	ref.Alias = aliasParts[0]
	return ref, true
}

func parseQualifiedReference(value string, maxParts int, allowWildcard bool) ([]string, bool) {
	if value == "" {
		return nil, false
	}

	parts := make([]string, 0, maxParts)
	for start := 0; start < len(value); {
		part, next, ok := parseReferencePart(value, start)
		if !ok {
			return nil, false
		}
		if part == "*" {
			if !allowWildcard || next != len(value) {
				return nil, false
			}
		}
		parts = append(parts, part)
		if len(parts) > maxParts {
			return nil, false
		}
		if next == len(value) {
			return parts, true
		}
		if value[next] != '.' {
			return nil, false
		}
		start = next + 1
		if start >= len(value) {
			return nil, false
		}
	}

	return parts, len(parts) > 0
}

func parseReferencePart(value string, start int) (string, int, bool) {
	if start >= len(value) {
		return "", 0, false
	}

	switch value[start] {
	case '"', '`':
		return parseQuotedReferencePart(value, start)
	}

	end := start
	for end < len(value) && value[end] != '.' {
		if !isBareIdentifierChar(value[end]) && value[end] != '*' {
			return "", 0, false
		}
		end++
	}

	part := value[start:end]
	if part == "" {
		return "", 0, false
	}
	if strings.IndexByte(part, '*') >= 0 && part != "*" {
		return "", 0, false
	}
	return part, end, true
}

func parseQuotedReferencePart(value string, start int) (string, int, bool) {
	quote := value[start]
	pos := start + 1
	segmentStart := pos
	var b strings.Builder

	for pos < len(value) {
		if value[pos] != quote {
			pos++
			continue
		}

		if pos+1 < len(value) && value[pos+1] == quote {
			if b.Cap() == 0 {
				b.Grow(len(value) - start)
			}
			b.WriteString(value[segmentStart:pos])
			b.WriteByte(quote)
			pos += 2
			segmentStart = pos
			continue
		}

		if b.Cap() == 0 {
			return value[start+1 : pos], pos + 1, true
		}
		b.WriteString(value[segmentStart:pos])
		return b.String(), pos + 1, true
	}

	return "", 0, false
}

func splitTopLevelWhitespace(value string) ([]string, bool) {
	tokens := make([]string, 0, 3)
	for i := 0; i < len(value); {
		for i < len(value) && isWhitespace(value[i]) {
			i++
		}
		if i >= len(value) {
			break
		}

		start := i
		for i < len(value) {
			switch value[i] {
			case '"', '`':
				next, ok := skipQuotedToken(value, i)
				if !ok {
					return nil, false
				}
				i = next
			default:
				if isWhitespace(value[i]) {
					tokens = append(tokens, value[start:i])
					goto nextToken
				}
				i++
			}
		}
		tokens = append(tokens, value[start:i])

	nextToken:
		if len(tokens) > 3 {
			return nil, false
		}
	}
	return tokens, true
}

func skipQuotedToken(value string, start int) (int, bool) {
	quote := value[start]
	for i := start + 1; i < len(value); i++ {
		if value[i] != quote {
			continue
		}
		if i+1 < len(value) && value[i+1] == quote {
			i++
			continue
		}
		return i + 1, true
	}
	return 0, false
}

func asciiEqualFold(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := 0; i < len(left); i++ {
		l := left[i]
		r := right[i]
		if 'A' <= l && l <= 'Z' {
			l += 'a' - 'A'
		}
		if 'A' <= r && r <= 'Z' {
			r += 'a' - 'A'
		}
		if l != r {
			return false
		}
	}
	return true
}

func isWhitespace(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\r', '\f', '\v':
		return true
	default:
		return false
	}
}

func isBareIdentifierChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '_' ||
		ch == '$'
}
