package stringutils

import (
	"strconv"
	"strings"
)

// EscapeString escapes a string for use in a SQL query
func EscapeString(str string) string {
	return strings.ReplaceAll(strings.ReplaceAll(str, "'", "''"), "\\", "\\\\")
}

func ToString(value interface{}) string {
	var str string
	switch v := value.(type) {
	case int, int8, int16, int32, int64:
		str = strconv.FormatInt(v.(int64), 10)
	case uint, uint8, uint16, uint32, uint64:
		str = strconv.FormatUint(v.(uint64), 10)
	case float32, float64:
		str = strconv.FormatFloat(v.(float64), 'f', -1, 64)
	case bool:
		str = strconv.FormatBool(v)
	case string:
		str = v
	case []byte:
		str = string(v)
	case nil:
		str = "NULL"
	}
	return str
}
