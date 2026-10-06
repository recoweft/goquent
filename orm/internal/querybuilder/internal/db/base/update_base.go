package base

import (
	"github.com/recoweft/goquent/orm/driver"
	"github.com/recoweft/goquent/orm/internal/writeinput"
	"sort"
	"strings"

	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/consts"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/jsonutils"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/memutils"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type UpdateBaseBuilder struct {
	u interfaces.SQLUtils
}

func formatJSONUpdateExpression(sb []byte, u interfaces.SQLUtils, field string, path []string, placeholder string) []byte {
	switch u.Dialect() {
	case consts.DialectMySQL:
		sb = u.EscapeReference(sb, field)
		sb = append(sb, " = JSON_SET("...)
		sb = u.EscapeReference(sb, field)
		sb = append(sb, ", '$."+strings.Join(path, ".")+"', "...)
		sb = append(sb, placeholder...)
		sb = append(sb, ')')
	case consts.DialectPostgreSQL:
		sb = u.EscapeReference(sb, field)
		sb = append(sb, " = jsonb_set("...)
		sb = u.EscapeReference(sb, field)
		sb = append(sb, ", '{"+strings.Join(path, ",")+"}', "...)
		sb = append(sb, placeholder...)
		sb = append(sb, ')')
	default:
		sb = u.EscapeReference(sb, field)
		sb = append(sb, " = "...)
		sb = append(sb, placeholder...)
	}
	return sb
}

func NewUpdateBaseBuilder(util interfaces.SQLUtils, iq *structs.UpdateQuery) *UpdateBaseBuilder {
	return &UpdateBaseBuilder{
		u: util,
	}
}

func (m *UpdateBaseBuilder) Update(q *structs.UpdateQuery) *UpdateBaseBuilder {
	return m
}

// UpdateBatch builds the Update query for Update.
func (m *UpdateBaseBuilder) BuildUpdate(q *structs.UpdateQuery) (string, []interface{}, error) {
	if err := structs.ValidateQuery(q.Query); err != nil {
		return "", nil, err
	}
	q.Query.WhereTree = nil
	ptr := poolBytes.Get().(*[]byte)
	sb := *ptr
	if len(sb) > 0 {
		sb = sb[:0]
	}

	vPtr := poolValues.Get().(*[]interface{})
	values := *vPtr
	if len(values) > 0 {
		values = values[0:0]
	}

	// UPDATE
	sb = append(sb, "UPDATE "...)
	sb = writeTable(sb, m.u, q.Table, q.Options)

	// JOIN
	b := NewJoinBaseBuilder(m.u, q.Query.Joins)
	joinValues := b.Join(&sb, q.Query.Joins)
	values = append(values, joinValues...)

	// SET
	sb = append(sb, " SET "...)
	columns := make([]string, 0, len(q.Values))
	for column := range q.Values {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	if q.Options.Columns != nil {
		columns = q.Options.Columns
	}
	for i, column := range columns {
		if !q.Options.Literal && strings.Contains(column, "->") {
			field, path := jsonutils.ParseJsonFieldAndPath(column)
			sb = formatJSONUpdateExpression(sb, m.u, field, path, m.u.GetPlaceholder())
		} else {
			sb = writeColumn(sb, m.u, column, q.Options.Literal)
			sep := " = "
			if q.Options.Literal {
				sep = "="
			}
			sb = append(sb, sep+m.u.GetPlaceholder()...)
		}
		if i < len(columns)-1 {
			sb = append(sb, ", "...)
		}
		values = append(values, q.Values[column])
	}

	if len(q.Options.Assignments) > 0 {
		var d driver.Dialect = driver.MySQLDialect{}
		if m.u.Dialect() == consts.DialectPostgreSQL {
			d = driver.PostgresDialect{}
		}
		parts, args, err := writeinput.SetParts(d, nil, nil, q.Options.Assignments, len(values)+1)
		if err != nil {
			return "", nil, err
		}
		if len(columns) > 0 {
			sb = append(sb, ", "...)
		}
		sb = append(sb, strings.Join(parts, ", ")...)
		values = append(values, args...)
		for range args {
			m.u.GetPlaceholder()
		}
	}

	// WHERE
	if len(q.Query.ConditionGroups) > 0 {
		wb := NewWhereBaseBuilder(m.u, q.Query.ConditionGroups)
		whereValues, tree, err := RenderPredicates(&sb, q.Query.ConditionGroups, len(values), wb.RenderLeaf)
		q.Query.WhereTree = tree
		if err != nil {
			return "", nil, err
		}
		values = append(values, whereValues...)
	}

	if len(*q.Query.Order) > 0 {
		ob := NewOrderByBaseBuilder(m.u, q.Query.Order)
		ob.OrderBy(&sb, q.Query.Order)
	}

	query := string(sb)

	retVals := append([]interface{}(nil), values...)

	memutils.ZeroBytes(sb)
	sb = sb[:0]
	*ptr = sb
	poolBytes.Put(ptr)

	memutils.ZeroInterfaces(values)
	values = values[:0]
	*vPtr = values
	poolValues.Put(vPtr)

	return query, retVals, nil
}
