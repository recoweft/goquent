package query

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"regexp"
	"strconv"
	"time"
)

func performanceNow() time.Time { return time.Now().UTC() }
func performanceNumber(n float64) bool {
	return n >= 0 && n <= 1e15 && !math.IsNaN(n) && !math.IsInf(n, 0)
}

// performanceJSON validates the complete input before extracting any estimate.
// Database JSON keys are case sensitive, but duplicate keys are never accepted.
func performanceJSON(b []byte) (any, error) { return performanceJSONBounded(b, 1<<20, 32, 4096) }

func performanceJSONBounded(b []byte, maxBytes, maxDepth, maxNodes int) (any, error) {
	if len(b) > maxBytes {
		return nil, ErrPerformanceLimit
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	nodes := 0
	var read func(int) (any, error)
	read = func(depth int) (any, error) {
		nodes++
		if depth > maxDepth || nodes > maxNodes {
			return nil, ErrPerformanceLimit
		}
		t, e := d.Token()
		if e != nil {
			return nil, ErrPerformanceResult
		}
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				m := map[string]any{}
				for d.More() {
					key, e := d.Token()
					if e != nil {
						return nil, ErrPerformanceResult
					}
					k, ok := key.(string)
					if !ok {
						return nil, ErrPerformanceResult
					}
					if _, ok := m[k]; ok {
						return nil, ErrPerformanceResult
					}
					v, e := read(depth + 1)
					if e != nil {
						return nil, e
					}
					m[k] = v
				}
				end, e := d.Token()
				if e != nil || end != json.Delim('}') {
					return nil, ErrPerformanceResult
				}
				return m, nil
			case '[':
				var a []any
				for d.More() {
					v, e := read(depth + 1)
					if e != nil {
						return nil, e
					}
					a = append(a, v)
				}
				end, e := d.Token()
				if e != nil || end != json.Delim(']') {
					return nil, ErrPerformanceResult
				}
				return a, nil
			default:
				return nil, ErrPerformanceResult
			}
		}
		if n, ok := t.(json.Number); ok {
			f, e := n.Float64()
			if e != nil || math.IsNaN(f) || math.IsInf(f, 0) {
				return nil, ErrPerformanceResult
			}
		}
		return t, nil
	}
	v, e := read(0)
	if e != nil {
		return nil, e
	}
	if _, e = d.Token(); e != io.EOF {
		return nil, ErrPerformanceResult
	}
	return v, nil
}

var performanceNumericLexeme = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)

func performanceMetric(m map[string]any, key string) (*float64, error) {
	x, ok := m[key]
	if !ok {
		return nil, nil
	}
	s := ""
	switch n := x.(type) {
	case json.Number:
		s = n.String()
	case string:
		s = n
	default:
		return nil, ErrPerformanceResult
	}
	if !performanceNumericLexeme.MatchString(s) {
		return nil, ErrPerformanceResult
	}
	n, e := strconv.ParseFloat(s, 64)
	if e != nil || !performanceNumber(n) {
		return nil, ErrPerformanceResult
	}
	if (key == "Plan Rows" || key == "rows_examined_per_scan" || key == "rows_produced_per_join") && math.Trunc(n) != n {
		return nil, ErrPerformanceResult
	}
	return &n, nil
}
func parsePerformance(dialect string, b []byte) ([]PerformanceNode, bool, error) {
	root, e := performanceJSON(b)
	if e != nil {
		return nil, true, e
	}
	nodes := []PerformanceNode{}
	unknown, e := performanceStructure(dialect, root)
	if e != nil {
		return nil, true, e
	}
	if dialect == "postgres" {
		a, ok := root.([]any)
		if !ok || len(a) != 1 {
			return nil, true, ErrPerformanceResult
		}
		m, ok := a[0].(map[string]any)
		if !ok {
			return nil, true, ErrPerformanceResult
		}
		plan, ok := m["Plan"].(map[string]any)
		if !ok {
			return nil, true, nil
		}
		var walk func(map[string]any) error
		walk = func(m map[string]any) error {
			n := PerformanceNode{Scan: "unknown"}
			typ, _ := m["Node Type"].(string)
			switch typ {
			case "Seq Scan":
				n.Scan = "sequential"
			case "Index Scan":
				n.Scan = "index"
			case "Index Only Scan":
				n.Scan = "index_only"
			case "Bitmap Heap Scan", "Bitmap Index Scan":
				n.Scan = "bitmap"
			case "Limit":
				n.Scan = "limit"
			case "Sort", "Incremental Sort":
				n.Scan = "sort"
			case "Result":
				n.Scan = "result"
			}
			n.Index, _ = m["Index Name"].(string)
			var e error
			n.Rows, e = performanceMetric(m, "Plan Rows")
			if e != nil {
				return e
			}
			n.Cost, e = performanceMetric(m, "Total Cost")
			if e != nil {
				return e
			}
			if _, e = performanceMetric(m, "Startup Cost"); e != nil {
				return e
			}
			n.Unknown = n.Scan == "unknown" || n.Rows == nil || n.Cost == nil
			unknown = unknown || n.Unknown
			nodes = append(nodes, n)
			if children, ok := m["Plans"]; ok {
				a, ok := children.([]any)
				if !ok {
					return ErrPerformanceResult
				}
				for _, child := range a {
					m, ok := child.(map[string]any)
					if !ok {
						return ErrPerformanceResult
					}
					if e := walk(m); e != nil {
						return e
					}
				}
			}
			return nil
		}
		e = walk(plan)
	} else if dialect == "mysql" {
		m, ok := root.(map[string]any)
		if !ok {
			return nil, true, ErrPerformanceResult
		}
		q, ok := m["query_block"].(map[string]any)
		if !ok {
			return nil, true, nil
		}
		var walk func(map[string]any) error
		walk = func(m map[string]any) error {
			if t, ok := m["table"]; ok {
				table, ok := t.(map[string]any)
				if !ok {
					return ErrPerformanceResult
				}
				n := PerformanceNode{Scan: "unknown"}
				typ, _ := table["access_type"].(string)
				switch typ {
				case "ALL":
					n.Scan = "sequential"
				case "index", "ref", "eq_ref":
					n.Scan = "index"
				case "range":
					n.Scan = "range"
				case "const", "system":
					n.Scan = "constant"
				}
				n.Index, _ = table["key"].(string)
				var e error
				n.Rows, e = performanceMetric(table, "rows_examined_per_scan")
				if e != nil {
					return e
				}
				if _, e = performanceMetric(table, "rows_produced_per_join"); e != nil {
					return e
				}
				if c, ok := table["cost_info"].(map[string]any); ok {
					n.Cost, e = performanceMetric(c, "prefix_cost")
					if e != nil {
						return e
					}
					for _, k := range []string{"read_cost", "eval_cost"} {
						if _, e = performanceMetric(c, k); e != nil {
							return e
						}
					}
				}
				n.Unknown = n.Scan == "unknown" || n.Rows == nil || n.Cost == nil
				unknown = unknown || n.Unknown
				nodes = append(nodes, n)
			}
			if c, ok := m["cost_info"].(map[string]any); ok {
				if _, e := performanceMetric(c, "query_cost"); e != nil {
					return e
				}
			}
			for k, v := range m {
				switch k {
				case "ordering_operation", "grouping_operation", "duplicates_removal", "buffer_result":
					c, ok := v.(map[string]any)
					if !ok {
						return ErrPerformanceResult
					}
					if k != "ordering_operation" {
						unknown = true
					}
					if e := walk(c); e != nil {
						return e
					}
				case "nested_loop":
					unknown = true
					a, ok := v.([]any)
					if !ok {
						return ErrPerformanceResult
					}
					for _, item := range a {
						c, ok := item.(map[string]any)
						if !ok {
							return ErrPerformanceResult
						}
						if e := walk(c); e != nil {
							return e
						}
					}
				case "table", "cost_info", "select_id", "using_filesort", "message":
				}
			}
			return nil
		}
		e = walk(q)
	} else {
		return nil, true, ErrPerformanceUnsupported
	}
	if e != nil {
		return nil, true, e
	}
	if len(nodes) == 0 {
		unknown = true
	}
	return nodes, unknown, nil
}
