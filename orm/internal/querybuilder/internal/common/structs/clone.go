package structs

func CloneQuery(q *Query) *Query {
	if q == nil {
		return nil
	}
	out := *q
	out.Columns = cloneColumnsPtr(q.Columns)
	out.Joins = CloneJoins(q.Joins)
	out.ConditionGroups = cloneWhereGroups(q.ConditionGroups)
	out.Conditions = cloneWherePtr(q.Conditions)
	out.Order = cloneOrdersPtr(q.Order)
	out.Group = cloneGroupBy(q.Group)
	out.Lock = cloneLock(q.Lock)
	return &out
}

func CloneJoins(joins *Joins) *Joins {
	if joins == nil {
		return nil
	}
	out := *joins
	out.TargetNameMap = cloneStringMap(joins.TargetNameMap)
	out.Joins = cloneJoinPtr(joins.Joins)
	out.JoinClauses = cloneJoinClausePtr(joins.JoinClauses)
	out.LateralJoins = cloneJoinPtr(joins.LateralJoins)
	return &out
}

func CloneOrdersPtr(orders *[]Order) *[]Order {
	return cloneOrdersPtr(orders)
}

func cloneColumnsPtr(cols *[]Column) *[]Column {
	if cols == nil {
		return nil
	}
	out := make([]Column, len(*cols))
	for i, col := range *cols {
		out[i] = col
		out[i].Values = cloneInterfaces(col.Values)
	}
	return &out
}

func cloneWherePtr(wheres *[]Where) *[]Where {
	if wheres == nil {
		return nil
	}
	out := make([]Where, len(*wheres))
	for i, where := range *wheres {
		out[i] = cloneWhere(where)
	}
	return &out
}

func cloneWhereGroups(groups []WhereGroup) []WhereGroup {
	if groups == nil {
		return nil
	}
	out := make([]WhereGroup, len(groups))
	for i, group := range groups {
		out[i] = group
		out[i].Conditions = make([]Where, len(group.Conditions))
		for j, where := range group.Conditions {
			out[i].Conditions[j] = cloneWhere(where)
		}
	}
	return out
}

func cloneWhere(where Where) Where {
	out := where
	out.Value = cloneInterfaces(where.Value)
	out.ValueMap = cloneAnyMap(where.ValueMap)
	out.Query = CloneQuery(where.Query)
	out.Between = cloneWhereBetween(where.Between)
	out.Exists = cloneExists(where.Exists)
	out.FullText = cloneFullText(where.FullText)
	out.JsonContains = cloneJSONContains(where.JsonContains)
	out.JsonLength = cloneJSONLength(where.JsonLength)
	return out
}

func cloneWhereBetween(v *WhereBetween) *WhereBetween {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

func cloneExists(v *Exists) *Exists {
	if v == nil {
		return nil
	}
	out := *v
	out.Query = CloneQuery(v.Query)
	return &out
}

func cloneFullText(v *FullText) *FullText {
	if v == nil {
		return nil
	}
	out := *v
	out.Columns = cloneStrings(v.Columns)
	out.Options = cloneInterfaceMap(v.Options)
	return &out
}

func cloneJSONContains(v *JsonContains) *JsonContains {
	if v == nil {
		return nil
	}
	out := *v
	out.Values = cloneInterfaces(v.Values)
	return &out
}

func cloneJSONLength(v *JsonLength) *JsonLength {
	if v == nil {
		return nil
	}
	out := *v
	return &out
}

func cloneJoinPtr(joins *[]Join) *[]Join {
	if joins == nil {
		return nil
	}
	out := make([]Join, len(*joins))
	for i, join := range *joins {
		out[i] = join
		out[i].TargetNameMap = cloneStringMap(join.TargetNameMap)
		out[i].Query = CloneQuery(join.Query)
	}
	return &out
}

func cloneJoinClausePtr(clauses *[]JoinClause) *[]JoinClause {
	if clauses == nil {
		return nil
	}
	out := make([]JoinClause, len(*clauses))
	for i, clause := range *clauses {
		out[i] = clause
		out[i].On = cloneOnPtr(clause.On)
		out[i].ConditionGroups = cloneWhereGroupsPtr(clause.ConditionGroups)
		out[i].Conditions = cloneWherePtr(clause.Conditions)
		out[i].TargetNameMap = cloneStringMap(clause.TargetNameMap)
		out[i].Query = CloneQuery(clause.Query)
	}
	return &out
}

func cloneOnPtr(ons *[]On) *[]On {
	if ons == nil {
		return nil
	}
	out := make([]On, len(*ons))
	copy(out, *ons)
	return &out
}

func cloneWhereGroupsPtr(groups *[]WhereGroup) *[]WhereGroup {
	if groups == nil {
		return nil
	}
	out := cloneWhereGroups(*groups)
	return &out
}

func cloneOrdersPtr(orders *[]Order) *[]Order {
	if orders == nil {
		return nil
	}
	out := make([]Order, len(*orders))
	copy(out, *orders)
	return &out
}

func cloneGroupBy(group *GroupBy) *GroupBy {
	if group == nil {
		return nil
	}
	out := *group
	out.Columns = cloneStrings(group.Columns)
	out.Having = cloneHavingPtr(group.Having)
	return &out
}

func cloneHavingPtr(having *[]Having) *[]Having {
	if having == nil {
		return nil
	}
	out := make([]Having, len(*having))
	copy(out, *having)
	return &out
}

func cloneLock(lock *Lock) *Lock {
	if lock == nil {
		return nil
	}
	out := *lock
	return &out
}

func cloneStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func cloneAnyMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func cloneInterfaceMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	out := make(map[string]interface{}, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func cloneStrings(src []string) []string {
	if src == nil {
		return nil
	}
	out := make([]string, len(src))
	copy(out, src)
	return out
}

func cloneInterfaces(src []interface{}) []interface{} {
	if src == nil {
		return nil
	}
	out := make([]interface{}, len(src))
	copy(out, src)
	return out
}
