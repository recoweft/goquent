package orm

import (
	"fmt"
	"github.com/recoweft/goquent/orm/internal/querybridge"
	"github.com/recoweft/goquent/orm/internal/writeinput"
	"github.com/recoweft/goquent/orm/model"
	"reflect"
	"sort"
	"strings"
)

func buildInsertInput(db *DB, v any, o *writeOptions) (querybridge.Request, error) {
	if len(o.assignments) > 0 {
		return querybridge.Request{}, fmt.Errorf("assignment options are not supported for Insert")
	}
	val := reflect.ValueOf(v)
	if !val.IsValid() {
		return querybridge.Request{}, fmt.Errorf("unsupported type <nil>")
	}
	typ := val.Type()
	var table string
	var cols []string
	var args []any

	if isMapStringInterface(typ) {
		if o.table == "" {
			return querybridge.Request{}, fmt.Errorf("Table option required for map writes")
		}
		table = o.table
		iter := val.MapRange()
		for iter.Next() {
			col := iter.Key().String()
			if len(o.cols) > 0 {
				if _, ok := o.cols[col]; !ok {
					continue
				}
			}
			if _, ok := o.omit[col]; ok {
				continue
			}
			cols = append(cols, col)
		}
		sort.Strings(cols)
		for _, col := range cols {
			args = append(args, val.MapIndex(reflect.ValueOf(col).Convert(typ.Key())).Interface())
		}
	} else if typ.Kind() == reflect.Struct {
		table = o.table
		if table == "" {
			table = model.TableName(v)
		}
		var err error
		cols, args, err = insertStructColumnsAndArgs(val, o)
		if err != nil {
			return querybridge.Request{}, err
		}
	} else {
		return querybridge.Request{}, fmt.Errorf("unsupported type %s", typ)
	}
	if len(cols) == 0 {
		return querybridge.Request{}, fmt.Errorf("no columns to insert")
	}
	return newWriteInput("insert", table, cols, args, 1, nil, o)
}

func buildInsertManyInput[T any](db *DB, values []T, o *writeOptions) (querybridge.Request, error) {
	if len(values) == 0 {
		return querybridge.Request{}, fmt.Errorf("goquent: no rows to insert")
	}
	if err := validateInsertManyOptions(o); err != nil {
		return querybridge.Request{}, err
	}

	first := reflect.ValueOf(values[0])
	if !first.IsValid() {
		return querybridge.Request{}, fmt.Errorf("unsupported type <nil>")
	}
	typ := first.Type()

	var (
		table string
		cols  []string
		args  []any
		err   error
	)
	switch {
	case isMapStringInterface(typ):
		if o.table == "" {
			return querybridge.Request{}, fmt.Errorf("Table option required for map writes")
		}
		table = o.table
		cols, args, err = insertManyMapColumnsAndArgs(values, o)
	case typ.Kind() == reflect.Struct:
		table = o.table
		if table == "" {
			table = model.TableName(values[0])
		}
		cols, args, err = insertManyStructColumnsAndArgs(values, typ, o)
	default:
		return querybridge.Request{}, fmt.Errorf("unsupported type %s", typ)
	}
	if err != nil {
		return querybridge.Request{}, err
	}
	r, err := newWriteInput("insert", table, cols, args, len(values), nil, o)
	r.Batch = true
	return r, err
}

func buildUpsertManyInput[T any](db *DB, values []T, o *writeOptions) (querybridge.Request, error) {
	if len(values) == 0 {
		return querybridge.Request{}, fmt.Errorf("goquent: no rows to upsert")
	}
	if !o.wherePK && !o.hasConflictTarget() {
		return querybridge.Request{}, fmt.Errorf("UpsertMany[T] requires WherePK, ConflictColumns, or ConflictConstraint")
	}
	if o.conflictDoNothing && (len(o.assignments) > 0 || len(o.upsertUpdateCols) > 0) {
		return querybridge.Request{}, fmt.Errorf("ConflictDoNothing cannot be combined with update or assignment options")
	}

	first := reflect.ValueOf(values[0])
	if !first.IsValid() {
		return querybridge.Request{}, fmt.Errorf("unsupported type <nil>")
	}
	typ := first.Type()

	var (
		table  string
		cols   []string
		args   []any
		pkCols []string
		err    error
	)
	switch {
	case isMapStringInterface(typ):
		if o.table == "" {
			return querybridge.Request{}, fmt.Errorf("Table option required for map writes")
		}
		if o.wherePK && len(o.pkCols) == 0 {
			return querybridge.Request{}, fmt.Errorf("WherePK for map writes requires PK columns via PK option")
		}
		table = o.table
		cols, args, pkCols, err = upsertManyMapColumnsArgsAndPK(values, o)
	case typ.Kind() == reflect.Struct:
		table = o.table
		if table == "" {
			table = model.TableName(values[0])
		}
		cols, args, pkCols, err = upsertManyStructColumnsArgsAndPK(values, typ, o)
	default:
		return querybridge.Request{}, fmt.Errorf("unsupported type %s", typ)
	}
	if err != nil {
		return querybridge.Request{}, err
	}
	if o.wherePK && len(pkCols) == 0 {
		return querybridge.Request{}, fmt.Errorf("WherePK requires pk values")
	}
	if len(cols) == 0 {
		return querybridge.Request{}, fmt.Errorf("no columns to insert")
	}
	r, err := newWriteInput("upsert", table, cols, args, len(values), pkCols, o)
	r.Batch = true
	return r, err
}

func validateInsertManyOptions(o *writeOptions) error {
	if len(o.assignments) > 0 {
		return fmt.Errorf("assignment options are not supported for InsertMany")
	}
	if len(o.conflictCols) > 0 ||
		strings.TrimSpace(o.conflictWhere) != "" ||
		strings.TrimSpace(o.conflictConstraint) != "" ||
		strings.TrimSpace(o.conflictTargetRaw) != "" ||
		o.hasUpsertUpdates ||
		o.conflictDoNothing {
		return fmt.Errorf("conflict/upsert options are not supported for InsertMany")
	}
	return nil
}

func insertManyStructColumnsAndArgs[T any](values []T, typ reflect.Type, o *writeOptions) ([]string, []any, error) {
	var cols []string
	args := make([]any, 0, len(values))
	for i, row := range values {
		val := reflect.ValueOf(row)
		if !val.IsValid() {
			return nil, nil, fmt.Errorf("goquent: InsertMany row %d is nil", i)
		}
		if val.Type() != typ {
			return nil, nil, fmt.Errorf("goquent: InsertMany row %d has type %s, expected %s", i, val.Type(), typ)
		}
		rowCols, rowArgs, err := insertStructColumnsAndArgs(val, o)
		if err != nil {
			return nil, nil, err
		}
		if i == 0 {
			cols = rowCols
		} else if !sameColumns(cols, rowCols) {
			return nil, nil, fmt.Errorf("goquent: InsertMany requires identical columns in every row")
		}
		args = append(args, rowArgs...)
	}
	if len(cols) == 0 {
		return nil, nil, fmt.Errorf("no columns to insert")
	}
	return cols, args, nil
}

func upsertManyStructColumnsArgsAndPK[T any](values []T, typ reflect.Type, o *writeOptions) ([]string, []any, []string, error) {
	meta, err := getTypeMeta(typ)
	if err != nil {
		return nil, nil, nil, err
	}
	var cols []string
	args := make([]any, 0, len(values))
	for i, row := range values {
		val := reflect.ValueOf(row)
		if !val.IsValid() {
			return nil, nil, nil, fmt.Errorf("goquent: UpsertMany row %d is nil", i)
		}
		if val.Type() != typ {
			return nil, nil, nil, fmt.Errorf("goquent: UpsertMany row %d has type %s, expected %s", i, val.Type(), typ)
		}
		rowCols, rowArgs := upsertStructColumnsAndArgs(val, meta, o)
		if i == 0 {
			cols = rowCols
		} else if !sameColumns(cols, rowCols) {
			return nil, nil, nil, fmt.Errorf("goquent: UpsertMany requires identical columns in every row")
		}
		args = append(args, rowArgs...)
	}
	return cols, args, append([]string(nil), meta.PKCols...), nil
}

func upsertStructColumnsAndArgs(val reflect.Value, meta *typeMeta, o *writeOptions) ([]string, []any) {
	cols := make([]string, 0, len(meta.Fields))
	args := make([]any, 0, len(meta.Fields))
	for _, fm := range meta.Fields {
		fv := val.FieldByIndex(fm.IndexPath)
		if fm.PK {
			cols = append(cols, fm.Col)
			args = append(args, fv.Interface())
			continue
		}
		if o.isConflictColumn(fm.Col) {
			cols = append(cols, fm.Col)
			args = append(args, fv.Interface())
			continue
		}
		if fm.Readonly {
			continue
		}
		if len(o.cols) > 0 {
			if _, ok := o.cols[fm.Col]; !ok {
				continue
			}
		}
		if _, ok := o.omit[fm.Col]; ok {
			continue
		}
		if fm.OmitEmpty && fv.IsZero() {
			continue
		}
		cols = append(cols, fm.Col)
		args = append(args, fv.Interface())
	}
	return cols, args
}

func insertStructColumnsAndArgs(val reflect.Value, o *writeOptions) ([]string, []any, error) {
	meta, err := getTypeMeta(val.Type())
	if err != nil {
		return nil, nil, err
	}
	cols := make([]string, 0, len(meta.Fields))
	args := make([]any, 0, len(meta.Fields))
	for _, fm := range meta.Fields {
		if fm.Readonly {
			continue
		}
		if len(o.cols) > 0 {
			if _, ok := o.cols[fm.Col]; !ok {
				continue
			}
		}
		if _, ok := o.omit[fm.Col]; ok {
			continue
		}
		fv := val.FieldByIndex(fm.IndexPath)
		if fm.OmitEmpty && fv.IsZero() {
			continue
		}
		cols = append(cols, fm.Col)
		args = append(args, fv.Interface())
	}
	return cols, args, nil
}

func insertManyMapColumnsAndArgs[T any](values []T, o *writeOptions) ([]string, []any, error) {
	first := reflect.ValueOf(values[0])
	cols := mapInsertManyColumnSet(first, o)
	if len(cols) == 0 {
		return nil, nil, fmt.Errorf("no columns to insert")
	}
	requireExact := len(o.cols) == 0
	args := make([]any, 0, len(values)*len(cols))
	for i, row := range values {
		val := reflect.ValueOf(row)
		if !val.IsValid() {
			return nil, nil, fmt.Errorf("goquent: InsertMany map row %d is nil", i)
		}
		if !isMapStringInterface(val.Type()) {
			return nil, nil, fmt.Errorf("goquent: InsertMany row %d has type %s, expected %s", i, val.Type(), first.Type())
		}
		if requireExact && !sameColumns(cols, mapInsertManyColumnSet(val, o)) {
			return nil, nil, fmt.Errorf("goquent: InsertMany map row %d has inconsistent columns", i)
		}
		rowArgs, err := mapInsertManyRowArgs(val, cols, i)
		if err != nil {
			return nil, nil, err
		}
		args = append(args, rowArgs...)
	}
	return cols, args, nil
}

func upsertManyMapColumnsArgsAndPK[T any](values []T, o *writeOptions) ([]string, []any, []string, error) {
	first := reflect.ValueOf(values[0])
	cols := mapUpsertManyColumnSet(first, o)
	if len(cols) == 0 {
		return nil, nil, nil, fmt.Errorf("no columns to insert")
	}
	pkCols := sortedPKColumns(o)
	args := make([]any, 0, len(values)*len(cols))
	for i, row := range values {
		val := reflect.ValueOf(row)
		if !val.IsValid() {
			return nil, nil, nil, fmt.Errorf("goquent: UpsertMany map row %d is nil", i)
		}
		if !isMapStringInterface(val.Type()) {
			return nil, nil, nil, fmt.Errorf("goquent: UpsertMany row %d has type %s, expected %s", i, val.Type(), first.Type())
		}
		if !sameColumns(cols, mapUpsertManyColumnSet(val, o)) {
			return nil, nil, nil, fmt.Errorf("goquent: UpsertMany map row %d has inconsistent columns", i)
		}
		rowArgs, err := mapInsertManyRowArgs(val, cols, i)
		if err != nil {
			return nil, nil, nil, err
		}
		args = append(args, rowArgs...)
	}
	return cols, args, pkCols, nil
}

func mapUpsertManyColumnSet(val reflect.Value, o *writeOptions) []string {
	cols := make([]string, 0, val.Len())
	iter := val.MapRange()
	for iter.Next() {
		col := iter.Key().String()
		if o.isPK(col) || o.isConflictColumn(col) {
			cols = append(cols, col)
			continue
		}
		if len(o.cols) > 0 {
			if _, ok := o.cols[col]; !ok {
				continue
			}
		}
		if _, omitted := o.omit[col]; omitted {
			continue
		}
		cols = append(cols, col)
	}
	sort.Strings(cols)
	return cols
}

func sortedPKColumns(o *writeOptions) []string {
	if len(o.pkCols) == 0 {
		return nil
	}
	cols := make([]string, 0, len(o.pkCols))
	for col := range o.pkCols {
		cols = append(cols, col)
	}
	sort.Strings(cols)
	return cols
}

func mapInsertManyColumnSet(val reflect.Value, o *writeOptions) []string {
	if len(o.cols) > 0 {
		cols := make([]string, 0, len(o.cols))
		for col := range o.cols {
			if _, omitted := o.omit[col]; omitted {
				continue
			}
			cols = append(cols, col)
		}
		sort.Strings(cols)
		return cols
	}
	cols := make([]string, 0, val.Len())
	iter := val.MapRange()
	for iter.Next() {
		col := iter.Key().String()
		if _, omitted := o.omit[col]; omitted {
			continue
		}
		cols = append(cols, col)
	}
	sort.Strings(cols)
	return cols
}

func mapInsertManyRowArgs(val reflect.Value, cols []string, row int) ([]any, error) {
	args := make([]any, 0, len(cols))
	for _, col := range cols {
		mv := val.MapIndex(reflect.ValueOf(col))
		if !mv.IsValid() {
			return nil, fmt.Errorf("goquent: InsertMany map row %d missing column %s", row, col)
		}
		args = append(args, mv.Interface())
	}
	return args, nil
}

func sameColumns(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func buildUpdateInput(db *DB, v any, o *writeOptions) (querybridge.Request, error) {
	if !o.wherePK {
		return querybridge.Request{}, fmt.Errorf("Update[T] without WherePK is not allowed")
	}
	assignmentTargets := writeinput.Targets(o.assignments)
	val := reflect.ValueOf(v)
	if !val.IsValid() {
		return querybridge.Request{}, fmt.Errorf("unsupported type <nil>")
	}
	typ := val.Type()
	var table string
	var setCols []string
	var setArgs []any
	var whereCols []string
	var whereArgs []any

	if isMapStringInterface(typ) {
		if o.table == "" {
			return querybridge.Request{}, fmt.Errorf("Table option required for map writes")
		}
		if len(o.pkCols) == 0 {
			return querybridge.Request{}, fmt.Errorf("WherePK for map writes requires PK columns via PK option")
		}
		table = o.table
		iter := val.MapRange()
		seen := make(map[string]bool)
		for iter.Next() {
			col := iter.Key().String()
			v := iter.Value()
			seen[col] = true
			if o.isPK(col) {
				whereCols = append(whereCols, col)
				whereArgs = append(whereArgs, v.Interface())
				continue
			}
			if _, ok := assignmentTargets[col]; ok {
				continue
			}
			if len(o.cols) > 0 {
				if _, ok := o.cols[col]; !ok {
					continue
				}
			}
			if _, ok := o.omit[col]; ok {
				continue
			}
			setCols = append(setCols, col)
			setArgs = append(setArgs, v.Interface())
		}
		for pk := range o.pkCols {
			if !seen[pk] {
				return querybridge.Request{}, fmt.Errorf("WherePK requires pk column %s", pk)
			}
		}
	} else if typ.Kind() == reflect.Struct {
		table = o.table
		if table == "" {
			table = model.TableName(v)
		}
		meta, err := getTypeMeta(typ)
		if err != nil {
			return querybridge.Request{}, err
		}
		for _, fm := range meta.FieldsByName {
			fv := val.FieldByIndex(fm.IndexPath)
			if fm.PK {
				whereCols = append(whereCols, fm.Col)
				whereArgs = append(whereArgs, fv.Interface())
				continue
			}
			if fm.Readonly {
				continue
			}
			if _, ok := assignmentTargets[fm.Col]; ok {
				continue
			}
			if len(o.cols) > 0 {
				if _, ok := o.cols[fm.Col]; !ok {
					continue
				}
			}
			if _, ok := o.omit[fm.Col]; ok {
				continue
			}
			if fm.OmitEmpty && fv.IsZero() {
				continue
			}
			setCols = append(setCols, fm.Col)
			setArgs = append(setArgs, fv.Interface())
		}
	} else {
		return querybridge.Request{}, fmt.Errorf("unsupported type %s", typ)
	}
	if len(whereCols) == 0 {
		return querybridge.Request{}, fmt.Errorf("WherePK requires pk values")
	}
	if len(setCols) == 0 && len(o.assignments) == 0 {
		return querybridge.Request{}, fmt.Errorf("no columns to update")
	}
	// Map and metadata-map iteration must not change statement/argument order.
	sortWriteFields(setCols, setArgs)
	sortWriteFields(whereCols, whereArgs)
	r, err := newWriteInput("update", table, setCols, setArgs, 1, nil, o)
	r.WhereColumns = whereCols
	r.WhereValues = whereArgs
	return r, err
}

func sortWriteFields(columns []string, values []any) {
	type field struct {
		name  string
		value any
	}
	fields := make([]field, len(columns))
	for i, name := range columns {
		fields[i] = field{name, values[i]}
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].name < fields[j].name })
	for i, f := range fields {
		columns[i], values[i] = f.name, f.value
	}
}

func buildUpsertInput(db *DB, v any, o *writeOptions) (querybridge.Request, error) {
	if !o.wherePK && !o.hasConflictTarget() {
		return querybridge.Request{}, fmt.Errorf("Upsert[T] requires WherePK, ConflictColumns, or ConflictConstraint")
	}
	if o.conflictDoNothing && (len(o.assignments) > 0 || len(o.upsertUpdateCols) > 0) {
		return querybridge.Request{}, fmt.Errorf("ConflictDoNothing cannot be combined with update or assignment options")
	}
	val := reflect.ValueOf(v)
	if !val.IsValid() {
		return querybridge.Request{}, fmt.Errorf("unsupported type <nil>")
	}
	typ := val.Type()
	var table string
	var cols []string
	var args []any
	var pkCols []string

	if isMapStringInterface(typ) {
		if o.table == "" {
			return querybridge.Request{}, fmt.Errorf("Table option required for map writes")
		}
		if o.wherePK && len(o.pkCols) == 0 {
			return querybridge.Request{}, fmt.Errorf("WherePK for map writes requires PK columns via PK option")
		}
		table = o.table
		iter := val.MapRange()
		seen := make(map[string]bool)
		for iter.Next() {
			col := iter.Key().String()
			fv := iter.Value().Interface()
			seen[col] = true
			if o.isPK(col) {
				pkCols = append(pkCols, col)
				cols = append(cols, col)
				args = append(args, fv)
				continue
			}
			if o.isConflictColumn(col) {
				cols = append(cols, col)
				args = append(args, fv)
				continue
			}
			if len(o.cols) > 0 {
				if _, ok := o.cols[col]; !ok {
					continue
				}
			}
			if _, ok := o.omit[col]; ok {
				continue
			}
			cols = append(cols, col)
			args = append(args, fv)
		}
		if o.wherePK {
			for pk := range o.pkCols {
				if !seen[pk] {
					return querybridge.Request{}, fmt.Errorf("WherePK requires pk column %s", pk)
				}
			}
		}
	} else if typ.Kind() == reflect.Struct {
		table = o.table
		if table == "" {
			table = model.TableName(v)
		}
		meta, err := getTypeMeta(typ)
		if err != nil {
			return querybridge.Request{}, err
		}
		for _, fm := range meta.FieldsByName {
			fv := val.FieldByIndex(fm.IndexPath)
			if fm.PK {
				pkCols = append(pkCols, fm.Col)
				cols = append(cols, fm.Col)
				args = append(args, fv.Interface())
				continue
			}
			if o.isConflictColumn(fm.Col) {
				cols = append(cols, fm.Col)
				args = append(args, fv.Interface())
				continue
			}
			if fm.Readonly {
				continue
			}
			if len(o.cols) > 0 {
				if _, ok := o.cols[fm.Col]; !ok {
					continue
				}
			}
			if _, ok := o.omit[fm.Col]; ok {
				continue
			}
			if fm.OmitEmpty && fv.IsZero() {
				continue
			}
			cols = append(cols, fm.Col)
			args = append(args, fv.Interface())
		}
	} else {
		return querybridge.Request{}, fmt.Errorf("unsupported type %s", typ)
	}
	if o.wherePK && len(pkCols) == 0 {
		return querybridge.Request{}, fmt.Errorf("WherePK requires pk values")
	}
	if len(cols) == 0 {
		return querybridge.Request{}, fmt.Errorf("no columns to insert")
	}
	return newWriteInput("upsert", table, cols, args, 1, pkCols, o)
}

func conflictTargetColumns(o *writeOptions, pkCols []string) []string {
	if len(o.conflictCols) > 0 {
		return append([]string(nil), o.conflictCols...)
	}
	return append([]string(nil), pkCols...)
}

func upsertUpdateColumns(cols []string, targetCols []string, o *writeOptions) ([]string, error) {
	assignmentTargets := writeinput.Targets(o.assignments)
	if o.hasUpsertUpdates {
		updateCols := dedupeColumns(o.upsertUpdateCols)
		updateCols = filterColumns(updateCols, assignmentTargets)
		if err := ensureUpsertUpdateColumnsPresent(updateCols, cols); err != nil {
			return nil, err
		}
		return updateCols, nil
	}
	target := make(map[string]struct{}, len(targetCols))
	for _, col := range targetCols {
		target[col] = struct{}{}
	}
	updateCols := make([]string, 0, len(cols))
	for _, col := range cols {
		if _, ok := target[col]; ok {
			continue
		}
		if _, ok := assignmentTargets[col]; ok {
			continue
		}
		updateCols = append(updateCols, col)
	}
	return updateCols, nil
}

func filterColumns(cols []string, excluded map[string]struct{}) []string {
	if len(excluded) == 0 {
		return cols
	}
	out := make([]string, 0, len(cols))
	for _, col := range cols {
		if _, ok := excluded[col]; ok {
			continue
		}
		out = append(out, col)
	}
	return out
}

func dedupeColumns(cols []string) []string {
	seen := make(map[string]struct{}, len(cols))
	out := make([]string, 0, len(cols))
	for _, col := range cols {
		key := strings.TrimSpace(col)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func ensureUpsertUpdateColumnsPresent(updateCols []string, insertCols []string) error {
	if len(updateCols) == 0 {
		return nil
	}
	present := make(map[string]struct{}, len(insertCols))
	for _, col := range insertCols {
		present[col] = struct{}{}
	}
	for _, col := range updateCols {
		if _, ok := present[col]; !ok {
			return fmt.Errorf("UpdateColumns requires inserted column %s", col)
		}
	}
	return nil
}
