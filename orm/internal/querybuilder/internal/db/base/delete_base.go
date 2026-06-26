package base

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/memutils"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/sqlutils"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/interfaces"
)

type DeleteBaseBuilder struct {
	u interfaces.SQLUtils
}

func NewDeleteBaseBuilder(util interfaces.SQLUtils, iq *structs.DeleteQuery) *DeleteBaseBuilder {
	return &DeleteBaseBuilder{
		u: util,
	}
}

func (m *DeleteBaseBuilder) Delete(q *structs.DeleteQuery) *DeleteBaseBuilder {
	return m
}

// DeleteBatch builds the Delete query for Delete.
func (m *DeleteBaseBuilder) BuildDelete(q *structs.DeleteQuery) (string, []interface{}, error) {
	//values := make([]interface{}, 0)

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

	// DELETE
	sb = append(sb, "DELETE"...)
	if q.Query.Joins != nil &&
		q.Query.Joins.Joins != nil &&
		(len(*q.Query.Joins.Joins) > 0 || (q.Query.Joins.JoinClauses != nil && len(*q.Query.Joins.JoinClauses) > 0)) {
		sb = append(sb, " "...)
		sb = m.u.EscapeReference(sb, sqlutils.RelationSelectReference(q.Table))
	}

	// FROM
	sb = append(sb, " FROM "...)
	sb = m.u.EscapeRelation(sb, q.Table)

	// JOIN
	jb := NewJoinBaseBuilder(m.u, q.Query.Joins)
	jb.Join(&sb, q.Query.Joins)

	// WHERE
	if len(q.Query.ConditionGroups) > 0 {
		wb := NewWhereBaseBuilder(m.u, q.Query.ConditionGroups)
		whereValues, err := wb.Where(&sb, q.Query.ConditionGroups)
		if err != nil {
			return "", nil, err
		}
		values = append(values, whereValues...)
	}

	// ORDER BY
	if len(*q.Query.Order) > 0 {
		ob := NewOrderByBaseBuilder(m.u, q.Query.Order)
		ob.OrderBy(&sb, q.Query.Order)
	}

	// LIMIT

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
