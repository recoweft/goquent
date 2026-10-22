package query

import (
	"encoding/json"
	"math"
	"regexp"
	"strings"
)

// These schemas describe the bounded non-ANALYZE JSON subset, not a SQL or
// expression validator. Identifier/expression strings remain sensitive source
// metadata and never establish execution permission. Every unclassified field
// (including an entire extension subtree) makes the aggregate evidence unknown.
var performanceStructures = map[string]map[string]string{
	"pg_root": {"Plan": "pg_node"},
	"pg_node": {
		"Node Type": "string", "Parent Relationship": "relationship",
		"Parallel Aware": "bool", "Async Capable": "bool",
		"Startup Cost": "number", "Total Cost": "number", "Plan Rows": "integer", "Plan Width": "integer",
		"Relation Name": "string", "Alias": "string", "Schema": "string", "Index Name": "string",
		"Scan Direction": "direction", "Index Cond": "string", "Filter": "string", "Recheck Cond": "string",
		"Sort Key": "strings", "Presorted Key": "strings", "Output": "strings", "Plans": "pg_nodes",
	},
	"mysql_root": {"query_block": "mysql_query"},
	"mysql_query": {
		"select_id": "integer", "cost_info": "mysql_query_cost", "table": "mysql_table",
		"ordering_operation": "mysql_operation", "grouping_operation": "unsupported_operation",
		"duplicates_removal": "unsupported_operation", "buffer_result": "unsupported_operation",
		"nested_loop": "mysql_nodes", "message": "message",
	},
	"mysql_operation": {
		"using_filesort": "bool", "using_temporary_table": "bool", "cost_info": "mysql_sort_cost",
		"table": "mysql_table", "nested_loop": "mysql_nodes", "ordering_operation": "mysql_operation",
		"grouping_operation": "unsupported_operation", "duplicates_removal": "unsupported_operation",
		"buffer_result": "unsupported_operation",
	},
	"mysql_table": {
		"table_name": "string", "access_type": "string", "possible_keys": "strings", "key": "string",
		"used_key_parts": "strings", "key_length": "key_lengths", "ref": "strings",
		"rows_examined_per_scan": "integer", "rows_produced_per_join": "integer", "filtered": "percent",
		"cost_info": "mysql_table_cost", "used_columns": "strings", "attached_condition": "string",
		"index_condition": "string", "using_index": "bool", "backward_index_scan": "bool",
	},
	"mysql_query_cost": {"query_cost": "numeric_text"},
	"mysql_sort_cost":  {"sort_cost": "numeric_text"},
	"mysql_table_cost": {
		"read_cost": "numeric_text", "eval_cost": "numeric_text", "prefix_cost": "numeric_text",
		"data_read_per_join": "bytes_text",
	},
}

// MySQL's data_read_per_join is a decimal magnitude with a binary unit suffix,
// unlike costs. Validate it independently; it is not a published cost metric.
var performanceBytesLexeme = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?([KMGTPE])?$`)

func performanceStructure(dialect string, root any) (bool, error) {
	unknown := false
	var check func(any, string) error
	check = func(v any, rule string) error {
		switch rule {
		case "key_lengths":
			s, ok := v.(string)
			if !ok {
				return ErrPerformanceResult
			}
			parts := strings.Split(s, ",")
			if len(parts) != 1 {
				unknown = true // Index-merge key lists are not interpreted.
			}
			for _, part := range parts {
				if e := check(part, "integer_text"); e != nil {
					return e
				}
			}
		case "unsupported_operation":
			unknown = true
			return check(v, "mysql_operation")
		case "pg_nodes", "mysql_nodes", "strings":
			a, ok := v.([]any)
			if !ok {
				return ErrPerformanceResult
			}
			child := "string"
			if rule == "pg_nodes" {
				child = "pg_node"
			} else if rule == "mysql_nodes" {
				child = "mysql_query"
				unknown = true // Joins are outside the interpreted subset.
			}
			for _, x := range a {
				if e := check(x, child); e != nil {
					return e
				}
			}
		case "string", "message", "relationship", "direction", "bytes_text":
			s, ok := v.(string)
			if !ok {
				return ErrPerformanceResult
			}
			switch rule {
			case "message":
				unknown = true // Free-form server messages are not interpreted.
			case "relationship":
				if s != "Outer" && s != "Inner" {
					unknown = true
				}
			case "direction":
				if s != "Forward" && s != "Backward" && s != "NoMovement" {
					unknown = true
				}
			case "bytes_text":
				if !performanceBytesLexeme.MatchString(s) {
					return ErrPerformanceResult
				}
				magnitude := strings.TrimRight(s, "KMGTPE")
				n, e := performanceMetric(map[string]any{"bytes": magnitude}, "bytes")
				if e != nil {
					return e
				}
				if magnitude != s {
					power := strings.IndexByte("KMGTPE", s[len(s)-1]) + 1
					if !performanceNumber(*n * math.Pow(1024, float64(power))) {
						return ErrPerformanceResult
					}
				}
			}
		case "bool":
			if _, ok := v.(bool); !ok {
				return ErrPerformanceResult
			}
		case "number", "integer", "numeric_text", "integer_text", "percent":
			// PostgreSQL emits JSON numbers. MySQL emits numeric strings for
			// costs/key lengths/filtered, and JSON numbers for row counts/IDs.
			if rule == "numeric_text" || rule == "integer_text" {
				if _, ok := v.(string); !ok {
					return ErrPerformanceResult
				}
			} else if rule != "percent" {
				if _, ok := v.(json.Number); !ok {
					return ErrPerformanceResult
				}
			}
			n, e := performanceMetric(map[string]any{"metric": v}, "metric")
			if e != nil {
				return e
			}
			if ((rule == "integer" || rule == "integer_text") && math.Trunc(*n) != *n) || (rule == "percent" && *n > 100) {
				return ErrPerformanceResult
			}
		default:
			m, ok := v.(map[string]any)
			if !ok {
				return ErrPerformanceResult
			}
			for k, x := range m {
				r, known := performanceStructures[rule][k]
				if !known {
					unknown = true
					continue
				}
				if e := check(x, r); e != nil {
					return e
				}
			}
		}
		return nil
	}
	var e error
	switch dialect {
	case "postgres":
		a, ok := root.([]any)
		if !ok || len(a) != 1 {
			return true, ErrPerformanceResult
		}
		e = check(a[0], "pg_root")
	case "mysql":
		e = check(root, "mysql_root")
	default:
		return true, ErrPerformanceUnsupported
	}
	return unknown, e
}
