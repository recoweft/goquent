package migration

import (
	"bytes"
	"encoding/json"
	"github.com/recoweft/goquent/internal/inputjson"
	"reflect"
	"strings"
)

func sourceObject(b []byte, target any) error {
	fields, err := inputjson.Object(b, nil)
	if err != nil {
		return err
	}
	typ := reflect.TypeOf(target).Elem()
	known := map[string]string{}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.PkgPath != "" {
			continue
		}
		tag := strings.Split(f.Tag.Get("json"), ",")[0]
		known[strings.ToLower(tag)] = tag
	}
	for k, v := range fields {
		if canonical, ok := known[strings.ToLower(k)]; ok {
			if k != canonical || bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
				return inputjson.ErrInvalid
			}
		}
	}
	if json.Unmarshal(b, target) != nil {
		return inputjson.ErrInvalid
	}
	return nil
}
func (v *Schema) UnmarshalJSON(b []byte) error {
	type plain Schema
	var next plain
	if err := sourceObject(b, &next); err != nil {
		return err
	}
	*v = Schema(next)
	return nil
}
func (v *TableSchema) UnmarshalJSON(b []byte) error {
	type plain TableSchema
	var next plain
	if err := sourceObject(b, &next); err != nil {
		return err
	}
	*v = TableSchema(next)
	return nil
}
func (v *IndexSchema) UnmarshalJSON(b []byte) error {
	type plain IndexSchema
	var next plain
	if err := sourceObject(b, &next); err != nil {
		return err
	}
	*v = IndexSchema(next)
	return nil
}
func (v *SchemaContext) UnmarshalJSON(b []byte) error {
	type plain SchemaContext
	var next plain
	if err := sourceObject(b, &next); err != nil {
		return err
	}
	*v = SchemaContext(next)
	return nil
}
func (v *SchemaCoverage) UnmarshalJSON(b []byte) error {
	type plain SchemaCoverage
	var next plain
	if err := sourceObject(b, &next); err != nil {
		return err
	}
	*v = SchemaCoverage(next)
	return nil
}
func (v *ObjectIdentity) UnmarshalJSON(b []byte) error {
	type plain ObjectIdentity
	var next plain
	if err := sourceObject(b, &next); err != nil {
		return err
	}
	*v = ObjectIdentity(next)
	return nil
}
func (v *IndexKey) UnmarshalJSON(b []byte) error {
	type plain IndexKey
	var next plain
	if err := sourceObject(b, &next); err != nil {
		return err
	}
	*v = IndexKey(next)
	return nil
}
func (v *ExpressionDefinition) UnmarshalJSON(b []byte) error {
	type plain ExpressionDefinition
	var next plain
	if err := sourceObject(b, &next); err != nil {
		return err
	}
	*v = ExpressionDefinition(next)
	return nil
}
func (v *ForeignKeySchema) UnmarshalJSON(b []byte) error {
	type plain ForeignKeySchema
	var next plain
	if err := sourceObject(b, &next); err != nil {
		return err
	}
	*v = ForeignKeySchema(next)
	return nil
}
func (v *CheckSchema) UnmarshalJSON(b []byte) error {
	type plain CheckSchema
	var next plain
	if err := sourceObject(b, &next); err != nil {
		return err
	}
	*v = CheckSchema(next)
	return nil
}
