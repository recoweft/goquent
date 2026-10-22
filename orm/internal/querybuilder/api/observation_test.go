package api

import (
	"github.com/recoweft/goquent/orm/internal/querybuilder/internal/db/base"
	"strings"
	"testing"
)

type observationOpaque struct{}

func (observationOpaque) String() string { panic("shape must not invoke values") }
func TestObservationShapeValueIndependenceAndBounds(t *testing.T) {
	makeBuilder := func(v any) *SelectQueryBuilder {
		b := NewSelectQueryBuilder(base.NewBaseQueryBuilder()).Table("users").Select("id").Where("id", "=", v).Limit(1)
		if _, _, _, e := b.BuildSnapshot(); e != nil {
			t.Fatal("build")
		}
		return b
	}
	a := makeBuilder(1)
	b := makeBuilder(2)
	if a.ObservationShape() == "" || a.ObservationShape() != b.ObservationShape() {
		t.Fatal("shape depends on values")
	}
	b.builder.BuiltQuery.ConditionGroups[0].IsDummyGroup = !b.builder.BuiltQuery.ConditionGroups[0].IsDummyGroup
	if a.ObservationShape() == b.ObservationShape() {
		t.Fatal("group collision")
	}
	b = makeBuilder(observationOpaque{})
	if b.ObservationShape() != "" {
		t.Fatal("opaque shape accepted")
	}
	b = makeBuilder(1)
	b.builder.BuiltQuery.ConditionGroups[0].Conditions[0].Condition = strings.Repeat(" ", 4096) + "="
	if b.ObservationShape() != "" {
		t.Fatal("unbounded key")
	}
	b = makeBuilder(1)
	(*b.builder.BuiltQuery.Columns)[0].Raw = "private SQL"
	if b.ObservationShape() != "" {
		t.Fatal("raw accepted")
	}
	b = makeBuilder(1)
	b.builder.BuiltQuery.Table.Name = strings.Repeat("x.", 100000)
	if b.ObservationShape() != "" {
		t.Fatal("unbounded target")
	}
	b = makeBuilder(1)
	(*b.builder.BuiltQuery.Columns)[0].Name = strings.Repeat("x.", 100000)
	if b.ObservationShape() != "" {
		t.Fatal("unbounded column")
	}

}
