package manifest

import (
	"fmt"
	"strings"
)

func (t *typedSkeleton) emitPatch(b *strings.Builder) {
	p := t.prefix
	fmt.Fprintf(b, "// %sPatch has independent, nominal column states. Its zero value changes nothing.\ntype %sPatch struct {\n", p, p)
	for _, c := range t.columns {
		if c.patchType != "" {
			fmt.Fprintf(b, "%s %s\n", c.field.Name, c.patchType)
		}
	}
	b.WriteString("}\n")
	opaqueType(b, p+"Patch")
	for _, c := range t.columns {
		if c.patchType == "" {
			continue
		}
		fmt.Fprintf(b, "type %s struct { state uint8; value %s }\n", c.patchType, c.scalar)
		opaqueType(b, c.patchType)
		fmt.Fprintf(b, "func(c %s) Set(value %s) %s { if !c.valid {return %s{state:3}};return %s{state:1,value:%s(value)} }\n", c.columnType, c.input, c.patchType, c.patchType, c.patchType, c.scalar)
		if c.column.NullableKnown && c.column.Nullable {
			fmt.Fprintf(b, "func(c %s) SetNull() %s {if !c.valid{return %s{state:3}};return %s{state:2}}\n", c.columnType, c.patchType, c.patchType, c.patchType)
		}
	}
	fmt.Fprintf(b, "// %sUpdate adds predicates without introducing write limits or ordering.\ntype %sUpdate struct {Filters []%sPredicate; AccessReason string}\n", p, p, p)
	opaqueType(b, p+"Update")
	fmt.Fprintf(b, "func(in %sUpdate) spec(patch %sPatch, returning []string) operation.UpdateSpec {\ns:=operation.UpdateSpec{Version:1,Model:%q,Returning:returning,AccessReason:in.AccessReason}\nfor _,f:=range in.Filters{s.Filters=append(s.Filters,f.filter)}\n", p, p, t.table.Name)
	for _, c := range t.columns {
		if c.patchType == "" {
			continue
		}
		fmt.Fprintf(b, "if patch.%s.state!=0 {a:=operation.UpdateAssignment{Column:%q};switch patch.%s.state {case 1:a.State=operation.UpdateValue;a.Value=patch.%s.value;a.ValuePresent=true;case 2:a.State=operation.UpdateNull;default:a.State=operation.UpdateState(\"invalid\")};s.Assignments=append(s.Assignments,a)}\n", c.field.Name, c.column.Name, c.field.Name, c.field.Name)
	}
	b.WriteString("return s}\n")
	if len(t.keys) == 0 {
		return
	}
	snapshot := privatePrefix(p) + "Snapshot"
	keyInput := privatePrefix(p) + "KeyInput"
	fmt.Fprintf(b, `func(r *%s) PlanUpdateByKey(ctx context.Context,key %sKey,patch %sPatch,input %sUpdate)(*orm.QueryPlan,operation.DiagnosticView,error){
 input.Filters=%s(key,%sRead{Filters:input.Filters}).Filters
 return r.db.CompileUpdateWithDiagnostics(ctx,input.spec(patch,nil),operation.Options{Manifest:%s()})
}
func(r *%s) UpdateByKey(ctx context.Context,key %sKey,patch %sPatch,input %sUpdate)(sql.Result,error){
 input.Filters=%s(key,%sRead{Filters:input.Filters}).Filters
 return orm.UpdateOperationBy(ctx,r.db,input.spec(patch,nil),operation.Options{Manifest:%s()})
}
`, t.repo, p, p, p, keyInput, p, snapshot, t.repo, p, p, p, keyInput, p, snapshot)
	for _, pr := range t.projections {
		fmt.Fprintf(b, `func(r *%s) PlanUpdate%sByKey(ctx context.Context,key %sKey,patch %sPatch,input %sUpdate)(*orm.QueryPlan,operation.DiagnosticView,error){
 input.Filters=%s(key,%sRead{Filters:input.Filters}).Filters
 return r.db.CompileUpdateWithDiagnostics(ctx,input.spec(patch,%s),operation.Options{Manifest:%s()})
}
func(r *%s) Update%sByKey(ctx context.Context,key %sKey,patch %sPatch,input %sUpdate)(%s,error){
 input.Filters=%s(key,%sRead{Filters:input.Filters}).Filters
 return orm.UpdateOperationReturningBy[%s](ctx,r.db,input.spec(patch,%s),operation.Options{Manifest:%s()})
}
`, t.repo, pr.name, p, p, p, keyInput, p, quotedColumns(pr.columns), snapshot, t.repo, pr.name, p, p, p, pr.row, keyInput, p, pr.row, quotedColumns(pr.columns), snapshot)
	}
}
