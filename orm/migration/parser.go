package migration

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/recoweft/goquent/orm/query"
)

type sqlStatement struct {
	SQL  string
	Line int
}

var (
	createTableRE = regexp.MustCompile(`(?is)^\s*CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([` + "`" + `"\w.]+)`)
	dropTableRE   = regexp.MustCompile(`(?is)^\s*DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?([` + "`" + `"\w.]+)`)
	addColumnRE   = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+([` + "`" + `"\w.]+)\s+ADD\s+(?:COLUMN\s+)?(?:IF\s+NOT\s+EXISTS\s+)?([` + "`" + `"\w.]+)\s+(.+)$`)
	dropColumnRE  = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+([` + "`" + `"\w.]+)\s+DROP\s+(?:COLUMN\s+)?(?:IF\s+EXISTS\s+)?([` + "`" + `"\w.]+)`)
	renameColRE   = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+([` + "`" + `"\w.]+)\s+RENAME\s+COLUMN\s+([` + "`" + `"\w.]+)\s+TO\s+([` + "`" + `"\w.]+)`)
	alterTypeRE   = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+([` + "`" + `"\w.]+)\s+ALTER\s+(?:COLUMN\s+)?([` + "`" + `"\w.]+)\s+TYPE\s+(.+)$`)
	setNotNullRE  = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+([` + "`" + `"\w.]+)\s+ALTER\s+(?:COLUMN\s+)?([` + "`" + `"\w.]+)\s+SET\s+NOT\s+NULL`)
	dropNotNullRE = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+([` + "`" + `"\w.]+)\s+ALTER\s+(?:COLUMN\s+)?([` + "`" + `"\w.]+)\s+DROP\s+NOT\s+NULL`)
	modifyColRE   = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+([` + "`" + `"\w.]+)\s+MODIFY\s+(?:COLUMN\s+)?([` + "`" + `"\w.]+)\s+(.+)$`)
	changeColRE   = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+([` + "`" + `"\w.]+)\s+CHANGE\s+(?:COLUMN\s+)?([` + "`" + `"\w.]+)\s+([` + "`" + `"\w.]+)\s+(.+)$`)
	createIndexRE = regexp.MustCompile(`(?is)^\s*CREATE\s+(?:UNIQUE\s+)?INDEX\s+(CONCURRENTLY\s+)?(?:IF\s+NOT\s+EXISTS\s+)?([` + "`" + `"\w.]+)\s+ON\s+([` + "`" + `"\w.]+)`)
	dropIndexRE   = regexp.MustCompile(`(?is)^\s*DROP\s+INDEX\s+(CONCURRENTLY\s+)?(?:IF\s+EXISTS\s+)?([` + "`" + `"\w.]+)`)

	notNullRE = regexp.MustCompile(`(?is)\bNOT\s+NULL\b`)
	defaultRE = regexp.MustCompile(`(?is)\bDEFAULT\b`)
)

func splitSQLStatements(sqlText string) []sqlStatement {
	var statements []sqlStatement
	var b strings.Builder
	line := 1
	startLine := 1
	inSingle := false
	inDouble := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false

	flush := func() {
		sql := strings.TrimSpace(b.String())
		if sql != "" {
			statements = append(statements, sqlStatement{SQL: sql, Line: startLine})
		}
		b.Reset()
	}

	for i := 0; i < len(sqlText); i++ {
		ch := sqlText[i]
		next := byte(0)
		if i+1 < len(sqlText) {
			next = sqlText[i+1]
		}

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
				line++
			}
			continue
		}
		if inBlockComment {
			if ch == '\n' {
				line++
			}
			if ch == '*' && next == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if b.Len() == 0 && isSpaceByte(ch) {
			if ch == '\n' {
				line++
			}
			continue
		}
		if b.Len() == 0 && !isSpaceByte(ch) {
			startLine = line
		}
		if !inSingle && !inDouble && !inBacktick {
			if ch == '-' && next == '-' {
				inLineComment = true
				i++
				continue
			}
			if ch == '/' && next == '*' {
				inBlockComment = true
				i++
				continue
			}
		}

		switch ch {
		case '\'':
			if !inDouble && !inBacktick {
				if inSingle && next == '\'' {
					b.WriteByte(ch)
					b.WriteByte(next)
					i++
					continue
				}
				inSingle = !inSingle
			}
		case '"':
			if !inSingle && !inBacktick {
				inDouble = !inDouble
			}
		case '`':
			if !inSingle && !inDouble {
				inBacktick = !inBacktick
			}
		case ';':
			if !inSingle && !inDouble && !inBacktick {
				flush()
				continue
			}
		case '\n':
			line++
		}
		b.WriteByte(ch)
	}
	flush()
	return statements
}

func parseMigrationStatement(statement sqlStatement) MigrationStep {
	sql := strings.TrimSpace(statement.SQL)
	if sql == "" {
		return MigrationStep{}
	}
	step := MigrationStep{
		SQL:               sql,
		Line:              statement.Line,
		RiskLevel:         query.RiskLow,
		AnalysisPrecision: query.AnalysisPrecise,
	}
	if m := createTableRE.FindStringSubmatch(sql); len(m) > 0 {
		step.Type = AddTable
		step.Table = cleanIdentifier(m[1])
		return step
	}
	if m := dropTableRE.FindStringSubmatch(sql); len(m) > 0 {
		step.Type = DropTable
		step.Table = cleanIdentifier(m[1])
		return step
	}
	if m := addColumnRE.FindStringSubmatch(sql); len(m) > 0 {
		step.Type = AddColumn
		step.Table = cleanIdentifier(m[1])
		step.Column = cleanIdentifier(m[2])
		step.ColumnType = extractColumnType(m[3])
		nullable := !notNullRE.MatchString(m[3])
		step.Nullable = &nullable
		step.HasDefault = defaultRE.MatchString(m[3])
		if step.HasDefault {
			step.DefaultExpression = extractDefaultExpression(m[3])
		}
		return step
	}
	if m := dropColumnRE.FindStringSubmatch(sql); len(m) > 0 {
		step.Type = DropColumn
		step.Table = cleanIdentifier(m[1])
		step.Column = cleanIdentifier(m[2])
		return step
	}
	if m := renameColRE.FindStringSubmatch(sql); len(m) > 0 {
		step.Type = RenameColumn
		step.Table = cleanIdentifier(m[1])
		step.FromName = cleanIdentifier(m[2])
		step.ToName = cleanIdentifier(m[3])
		step.Column = step.FromName
		return step
	}
	if m := alterTypeRE.FindStringSubmatch(sql); len(m) > 0 {
		step.Type = AlterColumnType
		step.Table = cleanIdentifier(m[1])
		step.Column = cleanIdentifier(m[2])
		step.NewType = extractColumnType(m[3])
		step.AnalysisPrecision = query.AnalysisPartial
		return step
	}
	if m := setNotNullRE.FindStringSubmatch(sql); len(m) > 0 {
		nullable := false
		step.Type = AlterNullability
		step.Table = cleanIdentifier(m[1])
		step.Column = cleanIdentifier(m[2])
		step.Nullable = &nullable
		return step
	}
	if m := dropNotNullRE.FindStringSubmatch(sql); len(m) > 0 {
		nullable := true
		step.Type = AlterNullability
		step.Table = cleanIdentifier(m[1])
		step.Column = cleanIdentifier(m[2])
		step.Nullable = &nullable
		return step
	}
	if m := modifyColRE.FindStringSubmatch(sql); len(m) > 0 {
		step.Type = AlterColumnType
		step.Table = cleanIdentifier(m[1])
		step.Column = cleanIdentifier(m[2])
		step.NewType = extractColumnType(m[3])
		step.AnalysisPrecision = query.AnalysisPartial
		return step
	}
	if m := changeColRE.FindStringSubmatch(sql); len(m) > 0 {
		step.Type = RenameColumn
		step.Table = cleanIdentifier(m[1])
		step.FromName = cleanIdentifier(m[2])
		step.ToName = cleanIdentifier(m[3])
		step.Column = step.FromName
		step.NewType = extractColumnType(m[4])
		step.AnalysisPrecision = query.AnalysisPartial
		return step
	}
	if m := createIndexRE.FindStringSubmatch(sql); len(m) > 0 {
		step.Type = AddIndex
		step.Concurrent = strings.TrimSpace(m[1]) != ""
		step.Index = cleanIdentifier(m[2])
		step.Table = cleanIdentifier(m[3])
		return step
	}
	if m := dropIndexRE.FindStringSubmatch(sql); len(m) > 0 {
		step.Type = DropIndex
		step.Concurrent = strings.TrimSpace(m[1]) != ""
		step.Index = cleanIdentifier(m[2])
		return step
	}
	if looksLikeDDL(sql) {
		step.Type = UnsupportedStep
		step.AnalysisPrecision = query.AnalysisUnsupported
		return step
	}
	step.Type = UnsupportedStep
	step.AnalysisPrecision = query.AnalysisUnsupported
	return step
}

func classifyStep(step *MigrationStep) {
	if step == nil {
		return
	}
	switch step.Type {
	case AddTable:
		step.RiskLevel = query.RiskLow
	case DropTable:
		step.RiskLevel = query.RiskDestructive
		step.Warnings = append(step.Warnings, newWarning(
			WarningMigrationDropTable,
			query.RiskDestructive,
			"migration drops a table",
			"confirm table ownership, usage, backups, and rollback plan before applying",
			false,
			step.Line,
		))
		step.Preflight = dropTablePreflight(step.Table)
	case AddColumn:
		nullable := true
		if step.Nullable != nil {
			nullable = *step.Nullable
		}
		switch {
		case nullable:
			step.RiskLevel = query.RiskLow
		case step.HasDefault:
			step.RiskLevel = query.RiskMedium
			step.Warnings = append(step.Warnings, newWarning(
				WarningMigrationAddNotNullColumn,
				query.RiskMedium,
				"migration adds a NOT NULL column with a default",
				"review lock/backfill behavior for the target database before applying",
				true,
				step.Line,
			))
		default:
			step.RiskLevel = query.RiskHigh
			step.Warnings = append(step.Warnings, newWarning(
				WarningMigrationAddNotNullColumn,
				query.RiskHigh,
				"migration adds a NOT NULL column without a default",
				"add the column nullable, backfill it, then enforce NOT NULL in a later migration",
				true,
				step.Line,
			))
			step.Preflight = addNotNullColumnPreflight(step.Table, step.Column)
		}
	case DropColumn:
		step.RiskLevel = query.RiskDestructive
		step.Warnings = append(step.Warnings, newWarning(
			WarningMigrationDropColumn,
			query.RiskDestructive,
			"migration drops a column",
			"deploy code that no longer reads or writes the column before dropping it",
			false,
			step.Line,
		))
		step.Preflight = dropColumnPreflight(step.Table, step.Column)
	case RenameColumn:
		step.RiskLevel = query.RiskHigh
		step.Warnings = append(step.Warnings, newWarning(
			WarningMigrationRenameColumn,
			query.RiskHigh,
			"migration renames a column",
			"use a backward-compatible expand/backfill/contract sequence when possible",
			true,
			step.Line,
		))
		step.Preflight = renameColumnPreflight(step.Table, step.FromName, step.ToName)
	case AlterColumnType:
		level, code, message, hint := classifyTypeChange(step.OldType, step.NewType)
		step.RiskLevel = level
		step.Warnings = append(step.Warnings, newWarning(code, level, message, hint, level != query.RiskDestructive, step.Line))
		if compareRisk(level, query.RiskHigh) >= 0 {
			step.Preflight = alterTypePreflight(step.Table, step.Column)
		}
	case AlterNullability:
		nullable := true
		if step.Nullable != nil {
			nullable = *step.Nullable
		}
		if nullable {
			step.RiskLevel = query.RiskLow
			return
		}
		step.RiskLevel = query.RiskHigh
		step.Warnings = append(step.Warnings, newWarning(
			WarningMigrationSetNotNull,
			query.RiskHigh,
			"migration enforces NOT NULL on an existing column",
			"backfill existing NULL values before enforcing NOT NULL",
			true,
			step.Line,
		))
		step.Preflight = setNotNullPreflight(step.Table, step.Column)
	case AddIndex:
		if step.Concurrent {
			step.RiskLevel = query.RiskLow
			return
		}
		step.RiskLevel = query.RiskMedium
		step.Warnings = append(step.Warnings, newWarning(
			WarningMigrationAddIndexNonConcurrent,
			query.RiskMedium,
			"migration adds an index without CONCURRENTLY",
			"use concurrent or online index creation when supported by the database",
			true,
			step.Line,
		))
	case DropIndex:
		step.RiskLevel = query.RiskMedium
		step.Warnings = append(step.Warnings, newWarning(
			WarningMigrationDropIndex,
			query.RiskMedium,
			"migration drops an index",
			"confirm no critical query plan depends on this index before applying",
			true,
			step.Line,
		))
	case UnsupportedStep:
		step.RiskLevel = query.RiskBlocked
		step.Warnings = append(step.Warnings, newWarning(
			WarningMigrationUnsupported,
			query.RiskBlocked,
			"migration statement could not be classified",
			"review this SQL manually and add a structured migration plan before applying",
			false,
			step.Line,
		))
	}
}

func classifyTypeChange(oldType, newType string) (query.RiskLevel, string, string, string) {
	oldType = normalizeType(oldType)
	newType = normalizeType(newType)
	if oldType == "" {
		return query.RiskHigh, WarningMigrationAlterColumnType,
			"migration alters a column type without known previous type",
			"verify whether the change narrows data and test conversion on production-like data"
	}
	if typeChangeNarrows(oldType, newType) {
		return query.RiskDestructive, WarningMigrationTypeNarrowing,
			"migration appears to narrow a column type",
			"avoid narrowing in-place; back up data and use a staged conversion"
	}
	if typeChangeExpands(oldType, newType) {
		return query.RiskMedium, WarningMigrationAlterColumnType,
			"migration appears to expand a column type",
			"confirm the database can apply this type change without long locks"
	}
	return query.RiskHigh, WarningMigrationAlterColumnType,
		"migration alters a column type with unknown conversion risk",
		"verify data compatibility and locking behavior before applying"
}

func cleanIdentifier(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimRight(s, ",;")
	s = strings.Trim(s, "`\"")
	return s
}

func extractColumnType(definition string) string {
	fields := strings.Fields(strings.TrimSpace(definition))
	var parts []string
	for _, field := range fields {
		normalized := strings.ToUpper(strings.Trim(field, ","))
		switch normalized {
		case "NOT", "NULL", "DEFAULT", "PRIMARY", "UNIQUE", "CHECK", "REFERENCES", "CONSTRAINT", "COLLATE", "COMMENT":
			return strings.TrimSpace(strings.Join(parts, " "))
		}
		parts = append(parts, strings.TrimRight(field, ","))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func extractDefaultExpression(definition string) string {
	loc := defaultRE.FindStringIndex(definition)
	if loc == nil {
		return ""
	}
	expr := strings.TrimSpace(definition[loc[1]:])
	if expr == "" {
		return ""
	}
	fields := strings.Fields(expr)
	var parts []string
	for _, field := range fields {
		normalized := strings.ToUpper(strings.Trim(field, ","))
		switch normalized {
		case "NOT", "NULL", "PRIMARY", "UNIQUE", "CHECK", "REFERENCES", "CONSTRAINT", "COLLATE", "COMMENT":
			return strings.TrimSpace(strings.Join(parts, " "))
		}
		parts = append(parts, strings.TrimRight(field, ","))
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func normalizeType(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(s)), " "))
}

func typeChangeNarrows(oldType, newType string) bool {
	oldBase, oldSize := parseTypeSize(oldType)
	newBase, newSize := parseTypeSize(newType)
	if oldBase == newBase && oldSize > 0 && newSize > 0 {
		return newSize < oldSize
	}
	rank := map[string]int{
		"smallint": 1,
		"integer":  2,
		"int":      2,
		"bigint":   3,
		"decimal":  4,
		"numeric":  4,
		"text":     5,
	}
	oldRank, oldOK := rank[oldBase]
	newRank, newOK := rank[newBase]
	return oldOK && newOK && newRank < oldRank
}

func typeChangeExpands(oldType, newType string) bool {
	oldBase, oldSize := parseTypeSize(oldType)
	newBase, newSize := parseTypeSize(newType)
	if oldBase == newBase && oldSize > 0 && newSize > 0 {
		return newSize >= oldSize
	}
	rank := map[string]int{
		"smallint": 1,
		"integer":  2,
		"int":      2,
		"bigint":   3,
		"decimal":  4,
		"numeric":  4,
		"text":     5,
	}
	oldRank, oldOK := rank[oldBase]
	newRank, newOK := rank[newBase]
	return oldOK && newOK && newRank > oldRank
}

func parseTypeSize(s string) (string, int) {
	s = normalizeType(s)
	if strings.HasPrefix(s, "character varying") {
		s = strings.Replace(s, "character varying", "varchar", 1)
	}
	idx := strings.IndexByte(s, '(')
	if idx < 0 {
		return strings.TrimSpace(s), 0
	}
	base := strings.TrimSpace(s[:idx])
	end := strings.IndexByte(s[idx+1:], ')')
	if end < 0 {
		return base, 0
	}
	sizePart := strings.TrimSpace(s[idx+1 : idx+1+end])
	if comma := strings.IndexByte(sizePart, ','); comma >= 0 {
		sizePart = sizePart[:comma]
	}
	size, _ := strconv.Atoi(sizePart)
	return base, size
}

func looksLikeDDL(sql string) bool {
	upper := strings.ToUpper(sql)
	for _, token := range []string{"CREATE", "ALTER", "DROP", "RENAME", "GRANT", "REVOKE"} {
		if containsSQLWord(upper, token) {
			return true
		}
	}
	return false
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

func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func dropTablePreflight(table string) []string {
	return []string{
		"confirm no application code references " + table,
		"confirm the table has been unused for the agreed retention window",
		"backup the table before migration",
		"deploy code that stops writing this table before dropping it",
	}
}

func dropColumnPreflight(table, column string) []string {
	name := table + "." + column
	return []string{
		"confirm no application code references " + name,
		"confirm the column has been unused for the agreed retention window",
		"backup the table before migration",
		"deploy code that stops reading and writing this column before dropping it",
	}
}

func addNotNullColumnPreflight(table, column string) []string {
	return []string{
		"confirm existing rows in " + table + " can be backfilled for " + column,
		"add the column nullable first when the table already has rows",
		"backfill existing rows before enforcing NOT NULL",
		"verify lock behavior on the target database",
	}
}

func renameColumnPreflight(table, fromName, toName string) []string {
	return []string{
		"confirm no deployed application code still references " + table + "." + fromName,
		"prefer add new column, dual write, backfill, switch reads, then drop old column",
		"verify rollback behavior while both column names may be in use",
		"coordinate deploy order with migration application",
	}
}

func alterTypePreflight(table, column string) []string {
	return []string{
		"check every existing value in " + table + "." + column + " can be converted",
		"test conversion on production-like data",
		"backup affected data before migration",
		"verify lock duration and rollback behavior on the target database",
	}
}

func setNotNullPreflight(table, column string) []string {
	return []string{
		"check for existing NULL values in " + table + "." + column,
		"backfill existing rows before enforcing NOT NULL",
		"verify writes already provide " + table + "." + column,
		"verify lock behavior on the target database",
	}
}
