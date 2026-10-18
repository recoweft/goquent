package api

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/common/structs"
)

// PerformanceShape contains only the bounded structural inputs used by the
// explicit collector. It is internal, never an executable public artifact.
type PerformanceShape struct {
	Table   string
	Columns []string
	Values  map[string][]any
}

var ErrPerformanceShape = errors.New("PERFORMANCE_UNSUPPORTED")

func performanceIdent(s string) bool {
	if len(s) == 0 || len(s) > 63 {
		return false
	}
	for i, c := range []byte(s) {
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// BuildPerformanceSnapshot checks the entire frozen structure before rendering.
// The normal BuildSnapshot path and its supported domain are unchanged.
func (qb *SelectQueryBuilder) BuildPerformanceSnapshot() (string, []any, QuerySnapshot, PerformanceShape, error) {
	var shape PerformanceShape
	fail := func() (string, []any, QuerySnapshot, PerformanceShape, error) {
		return "", nil, QuerySnapshot{}, PerformanceShape{}, ErrPerformanceShape
	}
	src := qb.snapshotQuery()
	if src == nil || src.PredicateError != nil || src.Limit.Limit < 1 || src.Limit.Limit > 10000 || src.Offset.Offset < 0 || src.Offset.Offset > 10000 || len(src.Unions) > 0 {
		return fail()
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
	shape = PerformanceShape{Table: src.Table.Name, Values: map[string][]any{}}
	column := func(s string) string {
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
		shape.Columns = append(shape.Columns, s)
		return true
	}
	for _, c := range *src.Columns {
		if c.Raw != "" || c.Function != "" || c.Count || c.Distinct || len(c.Values) > 0 || !add(c.Name) {
			return fail()
		}
	}
	if src.Order != nil {
		if len(*src.Order) > 128 {
			return fail()
		}
		for _, o := range *src.Order {
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
			nodes++
			if nodes > 256 || g.IsNot || g.Operator < 0 || g.Operator > 1 {
				return false
			}
			for _, c := range g.Conditions {
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
				name := column(c.Column)
				shape.Values[name] = append(shape.Values[name], vs...)
			}
		}
		return true
	}
	if !groups(src.ConditionGroups, 0) || src.Conditions != nil && len(*src.Conditions) > 0 {
		return fail()
	}
	// Install exactly the validated detached state; BuildSnapshot clones it for
	// the existing renderer. No caller or public plan is consulted afterwards.
	qb.builder.ApplyQueryState(src)
	sql, argsOut, snapshot, err := qb.BuildSnapshot()
	if err != nil || len(sql) > 64<<10 || len(argsOut) > 1024 {
		return fail()
	}
	return sql, argsOut, snapshot, shape, nil
}
