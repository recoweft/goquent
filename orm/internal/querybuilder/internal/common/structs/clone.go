package structs

import "github.com/recoweft/goquent/orm/internal/valuecopy"

func (c *cloneContext) CloneQuery(q *Query) *Query {
	if q == nil {
		return nil
	}
	if err := ValidateQuery(q); err != nil {
		return &Query{PredicateError: err}
	}
	out := *q
	out.WhereTree = c.values.Node(q.WhereTree)
	out.HavingTree = c.values.Node(q.HavingTree)
	out.Unions = make([]Union, len(q.Unions))
	for i, u := range q.Unions {
		out.Unions[i] = u
		out.Unions[i].Query = c.CloneQuery(u.Query)
	}
	out.Columns = c.cloneColumnsPtr(q.Columns)
	out.Joins = c.CloneJoins(q.Joins)
	out.ConditionGroups = c.cloneWhereGroups(q.ConditionGroups)
	out.Conditions = c.cloneWherePtr(q.Conditions)
	out.Order = c.cloneOrdersPtr(q.Order)
	out.Group = c.cloneGroupBy(q.Group)
	out.Lock = c.cloneLock(q.Lock)
	return &out
}

func (c *cloneContext) CloneJoins(joins *Joins) *Joins {
	if joins == nil {
		return nil
	}
	out := *joins
	out.TargetNameMap = c.cloneStringMap(joins.TargetNameMap)
	out.Joins = c.cloneJoinPtr(joins.Joins)
	out.JoinClauses = c.cloneJoinClausePtr(joins.JoinClauses)
	out.LateralJoins = c.cloneJoinPtr(joins.LateralJoins)
	return &out
}

func (c *cloneContext) CloneOrdersPtr(orders *[]Order) *[]Order {
	return c.cloneOrdersPtr(orders)
}

func (c *cloneContext) cloneColumnsPtr(cols *[]Column) *[]Column {
	if cols == nil {
		return nil
	}
	out := make([]Column, len(*cols))
	for i, col := range *cols {
		out[i] = col
		out[i].Values = c.cloneInterfaces(col.Values)
	}
	return &out
}

func (c *cloneContext) cloneWherePtr(wheres *[]Where) *[]Where {
	if wheres == nil {
		return nil
	}
	out := make([]Where, len(*wheres))
	for i, where := range *wheres {
		out[i] = c.cloneWhere(where)
	}
	return &out
}

func (c *cloneContext) cloneWhereGroups(groups []WhereGroup) []WhereGroup {
	if groups == nil {
		return nil
	}
	out := make([]WhereGroup, len(groups))
	for i, group := range groups {
		out[i] = group
		out[i].Conditions = make([]Where, len(group.Conditions))
		for j, where := range group.Conditions {
			out[i].Conditions[j] = c.cloneWhere(where)
		}
	}
	return out
}

func (c *cloneContext) cloneWhere(where Where) Where {
	out := where
	out.Nested = c.cloneWhereGroups(where.Nested)
	out.Value = c.cloneInterfaces(where.Value)
	out.ValueMap = c.cloneAnyMap(where.ValueMap)
	out.Query = c.CloneQuery(where.Query)
	out.Between = c.cloneWhereBetween(where.Between)
	out.Exists = c.cloneExists(where.Exists)
	out.FullText = c.cloneFullText(where.FullText)
	out.JsonContains = c.cloneJSONContains(where.JsonContains)
	out.JsonLength = c.cloneJSONLength(where.JsonLength)
	return out
}

func (c *cloneContext) cloneWhereBetween(v *WhereBetween) *WhereBetween {
	if v == nil {
		return nil
	}
	out := *v
	out.From, _ = c.values.Copy(v.From)
	out.To, _ = c.values.Copy(v.To)
	return &out
}

func (c *cloneContext) cloneExists(v *Exists) *Exists {
	if v == nil {
		return nil
	}
	out := *v
	out.Query = c.CloneQuery(v.Query)
	return &out
}

func (c *cloneContext) cloneFullText(v *FullText) *FullText {
	if v == nil {
		return nil
	}
	out := *v
	out.Columns = c.cloneStrings(v.Columns)
	out.Options = c.cloneInterfaceMap(v.Options)
	return &out
}

func (c *cloneContext) cloneJSONContains(v *JsonContains) *JsonContains {
	if v == nil {
		return nil
	}
	out := *v
	out.Values = c.cloneInterfaces(v.Values)
	return &out
}

func (c *cloneContext) cloneJSONLength(v *JsonLength) *JsonLength {
	if v == nil {
		return nil
	}
	out := *v
	out.Value, _ = c.values.Copy(v.Value)
	return &out
}

func (c *cloneContext) cloneJoinPtr(joins *[]Join) *[]Join {
	if joins == nil {
		return nil
	}
	out := make([]Join, len(*joins))
	for i, join := range *joins {
		out[i] = join
		out[i].TargetNameMap = c.cloneStringMap(join.TargetNameMap)
		out[i].Query = c.CloneQuery(join.Query)
	}
	return &out
}

func (c *cloneContext) cloneJoinClausePtr(clauses *[]JoinClause) *[]JoinClause {
	if clauses == nil {
		return nil
	}
	out := make([]JoinClause, len(*clauses))
	for i, clause := range *clauses {
		out[i] = clause
		out[i].On = c.cloneOnPtr(clause.On)
		out[i].ConditionGroups = c.cloneWhereGroupsPtr(clause.ConditionGroups)
		out[i].Conditions = c.cloneWherePtr(clause.Conditions)
		out[i].TargetNameMap = c.cloneStringMap(clause.TargetNameMap)
		out[i].Query = c.CloneQuery(clause.Query)
	}
	return &out
}

func (c *cloneContext) cloneOnPtr(ons *[]On) *[]On {
	if ons == nil {
		return nil
	}
	out := make([]On, len(*ons))
	copy(out, *ons)
	for i := range out {
		out[i].Value, _ = c.values.Copy(out[i].Value)
	}
	return &out
}

func (c *cloneContext) cloneWhereGroupsPtr(groups *[]WhereGroup) *[]WhereGroup {
	if groups == nil {
		return nil
	}
	out := c.cloneWhereGroups(*groups)
	return &out
}

func (c *cloneContext) cloneOrdersPtr(orders *[]Order) *[]Order {
	if orders == nil {
		return nil
	}
	out := make([]Order, len(*orders))
	copy(out, *orders)
	return &out
}

func (c *cloneContext) cloneGroupBy(group *GroupBy) *GroupBy {
	if group == nil {
		return nil
	}
	out := *group
	out.Columns = c.cloneStrings(group.Columns)
	out.Having = c.cloneHavingPtr(group.Having)
	return &out
}

func (c *cloneContext) cloneHavingPtr(having *[]Having) *[]Having {
	if having == nil {
		return nil
	}
	out := make([]Having, len(*having))
	copy(out, *having)
	for i := range out {
		out[i].Value, _ = c.values.Copy(out[i].Value)
	}
	return &out
}

func (c *cloneContext) cloneLock(lock *Lock) *Lock {
	if lock == nil {
		return nil
	}
	out := *lock
	return &out
}

func (c *cloneContext) cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func (c *cloneContext) cloneAnyMap(src map[string]any) map[string]any {
	v, _ := c.values.Copy(src)
	return v.(map[string]any)
}
func (c *cloneContext) cloneInterfaceMap(src map[string]interface{}) map[string]interface{} {
	return c.cloneAnyMap(src)
}

func (c *cloneContext) cloneStrings(src []string) []string {
	if src == nil {
		return nil
	}
	out := make([]string, len(src))
	copy(out, src)
	return out
}

func (c *cloneContext) cloneInterfaces(src []interface{}) []interface{} { return c.values.Slice(src) }

type cloneContext struct{ values *valuecopy.Copier }

func CloneQuery(q *Query) *Query         { return (&cloneContext{valuecopy.New()}).CloneQuery(q) }
func CloneJoins(j *Joins) *Joins         { return (&cloneContext{valuecopy.New()}).CloneJoins(j) }
func CloneOrdersPtr(o *[]Order) *[]Order { return (&cloneContext{valuecopy.New()}).CloneOrdersPtr(o) }
