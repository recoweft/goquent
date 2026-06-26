package sliceutils

import (
	"time"
)

func Contains[T comparable](elems []T, v T) bool {
	for i := range elems {
		if v == elems[i] {
			return true
		}
	}
	return false
}

func RemoveIfContains[T comparable](elems []T, v T) []T {
	for i := 0; i < len(elems); i++ {
		if elems[i] == v {
			elems = append(elems[:i], elems[i+1:]...)
			i--
		}
	}

	return elems
}

func Reverse[T comparable](s []T) []T {
	length := len(s)
	reversed := make([]T, length)

	for i, v := range s {
		reversed[length-1-i] = v
	}

	return reversed
}

func ToInterfaceSlice(slice interface{}) []interface{} {
	switch s := slice.(type) {
	case []int64:
		result := make([]interface{}, len(s))
		for i, v := range s {
			result[i] = v
		}
		return result
	case []string:
		result := make([]interface{}, len(s))
		for i, v := range s {
			result[i] = v
		}
		return result
	case []float64:
		result := make([]interface{}, len(s))
		for i, v := range s {
			result[i] = v
		}
		return result
	case []bool:
		result := make([]interface{}, len(s))
		for i, v := range s {
			result[i] = v
		}
		return result
	case []int:
		result := make([]interface{}, len(s))
		for i, v := range s {
			result[i] = v
		}
		return result
	case []int32:
		result := make([]interface{}, len(s))
		for i, v := range s {
			result[i] = v
		}
		return result
	case []uint:
		result := make([]interface{}, len(s))
		for i, v := range s {
			result[i] = v
		}
		return result
	case []uint32:
		result := make([]interface{}, len(s))
		for i, v := range s {
			result[i] = v
		}
		return result
	case []uint64:
		result := make([]interface{}, len(s))
		for i, v := range s {
			result[i] = v
		}
		return result
	case []float32:
		result := make([]interface{}, len(s))
		for i, v := range s {
			result[i] = v
		}
		return result
	case []time.Time:
		result := make([]interface{}, len(s))
		for i, v := range s {
			result[i] = v
		}
		return result
	default:
		return nil
	}
}

func AppendAndExtends[T any](slice []T, elems ...T) []T {
	for _, e := range elems {
		slice = append(slice, e)
	}

	return slice
}
