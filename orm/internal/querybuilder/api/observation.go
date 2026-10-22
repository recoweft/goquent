package api

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
	"math"
	"strconv"
	"strings"
	"time"
)

// ObservationShape reads the already-built private structure; it never rerenders SQL.
func (qb *SelectQueryBuilder) ObservationShape() string {
	var shape PerformanceShape
	var tokens []string
	size := 0
	limited := false
	addToken := func(v string) {
		if len(v) > 2048-size {
			limited = true
			return
		}
		size += len(v) + 1
		if size > 2048 {
			limited = true
			return
		}
		tokens = append(tokens, v)
	}
	fail := func() string {
		return ""
	}
	src := qb.builder.BuiltQuery
	if src == nil || src.PredicateError != nil || src.Limit.Limit < 1 || src.Limit.Limit > 10000 || src.Offset.Offset < 0 || src.Offset.Offset > 10000 || len(src.Unions) > 0 {
		return fail()
	}
	if len(src.Table.Name) > 127 {
		return ""
	}
	parts := strings.Split(src.Table.Name, ".")
	if len(parts) > 2 {
		return fail()
	}
	for _, p := range parts {
		if !performanceIdent(p) {
			return fail()
		}
	}
	if src.Lock != nil && src.Lock.LockType != "" {
		return fail()
	}
	if src.Group != nil && (len(src.Group.Columns) > 0 || src.Group.Having != nil && len(*src.Group.Having) > 0) {
		return fail()
	}
	if j := src.Joins; j != nil && (j.Name != "" || len(j.TargetNameMap) > 0 || j.Joins != nil && len(*j.Joins) > 0 || j.JoinClauses != nil && len(*j.JoinClauses) > 0 || j.LateralJoins != nil && len(*j.LateralJoins) > 0) {
		return fail()
	}
	if src.Columns == nil || len(*src.Columns) == 0 || len(*src.Columns) > 128 {
		return fail()
	}
	shape = PerformanceShape{Table: src.Table.Name}
	addToken(src.Table.Name)
	addToken(strconv.FormatInt(src.Limit.Limit, 10))
	addToken(strconv.FormatInt(src.Offset.Offset, 10))
	column := func(s string) string {
		if len(s) > 191 {
			return ""
		}
		p := strings.Split(s, ".")
		if len(p) > 1 {
			prefix := strings.Join(p[:len(p)-1], ".")
			if prefix != shape.Table && prefix != parts[len(parts)-1] {
				return ""
			}
		}
		if !performanceIdent(p[len(p)-1]) {
			return ""
		}
		return p[len(p)-1]
	}
	add := func(s string) bool {
		s = column(s)
		if !performanceIdent(s) {
			return false
		}
		addToken(s)
		return true
	}
	addToken("projections")
	addToken(strconv.Itoa(len(*src.Columns)))
	for _, c := range *src.Columns {
		if c.Raw != "" || c.Function != "" || c.Count || c.Distinct || len(c.Values) > 0 || !add(c.Name) {
			return fail()
		}
	}
	addToken("ordering")
	if src.Order != nil {
		if len(*src.Order) > 128 {
			return fail()
		}
		addToken(strconv.Itoa(len(*src.Order)))
		for _, o := range *src.Order {
			addToken("order")
			addToken(strconv.FormatBool(o.IsAsc))
			if o.Raw != "" || !add(o.Column) {
				return fail()
			}
		}
	}
	nodes, args, bytes := 0, 0, 0
	scalar := func(v any) bool {
		args++
		if args > 1024 {
			return false
		}
		switch v := v.(type) {
		case nil, bool, int, int8, int16, int32, int64, uint8, uint16, uint32:
			bytes += 8
		case uint:
			if uint64(v) > math.MaxInt64 {
				return false
			}
			bytes += 8
		case uint64:
			if v > math.MaxInt64 {
				return false
			}
			bytes += 8
		case float32:
			if math.IsInf(float64(v), 0) || math.IsNaN(float64(v)) {
				return false
			}
			bytes += 8
		case float64:
			if math.IsInf(v, 0) || math.IsNaN(v) {
				return false
			}
			bytes += 8
		case string:
			bytes += len(v)
		case []byte:
			bytes += len(v)
		case time.Time:
			bytes += 32
		default:
			return false
		}
		return bytes <= 1<<20
	}
	var groups func([]structs.WhereGroup, int) bool
	groups = func(gs []structs.WhereGroup, d int) bool {
		if d > 32 {
			return false
		}
		for _, g := range gs {
			addToken("group")
			addToken(strconv.Itoa(int(g.Operator)))
			addToken(strconv.FormatBool(g.IsDummyGroup))
			nodes++
			if nodes > 256 || g.IsNot || g.Operator < 0 || g.Operator > 1 {
				return false
			}
			for _, c := range g.Conditions {
				if len(c.Condition) > 16 {
					return false
				}
				addToken("condition")
				addToken(strconv.Itoa(int(c.Operator)))
				addToken(c.Condition)
				addToken(strconv.FormatBool(c.LiteralColumn))
				nodes++
				if nodes > 256 || c.Operator < 0 || c.Operator > 1 || c.Raw != "" || c.Function != "" || c.Query != nil || c.Exists != nil || c.FullText != nil || c.JsonContains != nil || c.JsonLength != nil || c.ValueColumn != "" || len(c.ValueMap) > 0 {
					return false
				}
				if c.Nested != nil {
					if c.Column != "" || len(c.Value) > 0 || c.Between != nil || !groups(c.Nested, d+1) {
						return false
					}
					continue
				}
				if c.LiteralColumn && !performanceIdent(c.Column) {
					return false
				}
				if !add(c.Column) {
					return false
				}
				vs := c.Value
				if c.Between != nil {
					if c.Between.IsColumn || c.Between.IsNot || len(vs) > 0 {
						return false
					}
					vs = []any{c.Between.From, c.Between.To}
				} else {
					switch strings.ToUpper(strings.TrimSpace(c.Condition)) {
					case "=", "<>", "<", "<=", ">", ">=":
						if len(vs) != 1 {
							return false
						}
					case "IN":
						if len(vs) == 0 {
							return false
						}
					case "IS NULL", "IS NOT NULL":
						if len(vs) != 0 {
							return false
						}
					default:
						return false
					}
				}
				for _, v := range vs {
					if !scalar(v) {
						return false
					}
				}
				addToken(strconv.Itoa(len(vs)))
				addToken(strconv.FormatBool(c.Between != nil))
			}
			addToken("end-group")
		}
		return true
	}
	if !groups(src.ConditionGroups, 0) || src.Conditions != nil && len(*src.Conditions) > 0 {
		return fail()
	}
	if limited {
		return ""
	}
	key := strings.Join(tokens, "\x00")
	if len(key) > 2048 {
		return ""
	}
	return key
}
