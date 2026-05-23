package orm

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/faciam-dev/goquent/orm/internal/stringutil"
)

type decoderFn func(dst reflect.Value, src any, pol BoolScanPolicy) error

type fieldMeta struct {
	Col        string
	IndexPath  []int
	PK         bool
	Readonly   bool
	OmitEmpty  bool
	BoolPolicy *BoolScanPolicy
	Decoder    decoderFn
}

func newFieldMeta(col string, index []int) *fieldMeta {
	return &fieldMeta{
		Col:       col,
		IndexPath: index,
	}
}

type typeMeta struct {
	FieldsByName map[string]*fieldMeta
	FieldsByNorm map[string]*fieldMeta
	Fields       []*fieldMeta
	PKCols       []string
}

var metaCache sync.Map // map[reflect.Type]*typeMeta

func normalize(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "_", "")
}

func getTypeMeta(t reflect.Type) (*typeMeta, error) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("type %s is not struct", t)
	}
	if m, ok := metaCache.Load(t); ok {
		return m.(*typeMeta), nil
	}
	m := &typeMeta{
		FieldsByName: make(map[string]*fieldMeta),
		FieldsByNorm: make(map[string]*fieldMeta),
	}
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" { // unexported
			continue
		}
		tag := sf.Tag.Get("db")
		if tag == "-" {
			continue
		}
		col := ""
		var opts []string
		if tag != "" {
			parts := strings.Split(tag, ",")
			col = parts[0]
			if len(parts) > 1 {
				opts = parts[1:]
			}
		}
		if col == "" {
			col = stringutil.ToSnake(sf.Name)
		}
		fm := newFieldMeta(col, sf.Index)
		for _, o := range opts {
			switch o {
			case "pk":
				fm.PK = true
				m.PKCols = append(m.PKCols, col)
			case "readonly":
				fm.Readonly = true
			case "omitempty":
				fm.OmitEmpty = true
			case "boolstrict":
				p := BoolStrict
				fm.BoolPolicy = &p
			case "boollenient":
				p := BoolLenient
				fm.BoolPolicy = &p
			}
		}
		// assign decoder based on field type
		switch sf.Type {
		case reflect.TypeOf(true):
			fm.Decoder = decodeBool
		case reflect.TypeOf(sql.NullBool{}):
			fm.Decoder = decodeNullBool
		default:
			if sf.Type.Kind() == reflect.Ptr && sf.Type.Elem().Kind() == reflect.Bool {
				fm.Decoder = decodePtrBool
			}
		}
		m.FieldsByName[col] = fm
		m.FieldsByNorm[normalize(col)] = fm
		m.Fields = append(m.Fields, fm)
	}
	metaCache.Store(t, m)
	return m, nil
}

func structColumnNames(t reflect.Type) ([]string, error) {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("type %s is not struct", t)
	}
	cols := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if sf.PkgPath != "" {
			continue
		}
		tag := sf.Tag.Get("db")
		if tag == "-" {
			continue
		}
		col := ""
		if tag != "" {
			col = strings.Split(tag, ",")[0]
		}
		if col == "" {
			col = stringutil.ToSnake(sf.Name)
		}
		cols = append(cols, col)
	}
	return cols, nil
}

// ResetMetaCache clears cached reflection metadata. Intended for tests.
func ResetMetaCache() {
	metaCache = sync.Map{}
}
