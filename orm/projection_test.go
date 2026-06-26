package orm

import (
	"context"
	"strings"
	"testing"

	"github.com/recoweft/goquent/orm/driver"
)

type projectionFlatRow struct {
	RowID     int64
	RowKey    string
	Position  int
	CellKey   string
	CellValue string
}

type projectionRow struct {
	ID       int64
	Key      string
	Position int
	Cells    []projectionCell
}

type projectionCell struct {
	Key   string
	Value string
}

type projectionAggregateParent struct {
	ID          int64
	Title       string
	OpenIssues  int
	NoticeCount int
}

type projectionAggregateRow struct {
	ParentID    int64
	OpenIssues  int
	NoticeCount int
}

type projectionWorkItemRow struct {
	ResourceType string `db:"resource_type"`
	ResourceID   string `db:"resource_id"`
	Title        string `db:"title"`
	NoticeCount  *int   `db:"notice_count"`
}

func TestProjectParentChildrenPreservesParentAndChildOrder(t *testing.T) {
	rows := []projectionFlatRow{
		{RowID: 10, RowKey: "r1", Position: 0, CellKey: "r1:c1", CellValue: "A"},
		{RowID: 10, RowKey: "r1", Position: 0, CellKey: "r1:c2", CellValue: "B"},
		{RowID: 11, RowKey: "r2", Position: 1, CellKey: "r2:c1", CellValue: "C"},
	}

	parents, err := ProjectParentChildren(rows, ParentChildProjection[projectionFlatRow, projectionRow, projectionCell, int64]{
		ParentKey: func(row projectionFlatRow) int64 {
			return row.RowID
		},
		Parent: func(row projectionFlatRow) projectionRow {
			return projectionRow{ID: row.RowID, Key: row.RowKey, Position: row.Position}
		},
		Child: func(row projectionFlatRow) (projectionCell, bool) {
			return projectionCell{Key: row.CellKey, Value: row.CellValue}, row.CellKey != ""
		},
		AppendChild: func(parent *projectionRow, child projectionCell) {
			parent.Cells = append(parent.Cells, child)
		},
	})
	if err != nil {
		t.Fatalf("project parent children: %v", err)
	}
	if len(parents) != 2 {
		t.Fatalf("expected two parents, got %+v", parents)
	}
	if parents[0].Key != "r1" || parents[1].Key != "r2" {
		t.Fatalf("unexpected parent order: %+v", parents)
	}
	if len(parents[0].Cells) != 2 || parents[0].Cells[0].Key != "r1:c1" || parents[0].Cells[1].Key != "r1:c2" {
		t.Fatalf("unexpected child order: %+v", parents[0].Cells)
	}
	if len(parents[1].Cells) != 1 || parents[1].Cells[0].Value != "C" {
		t.Fatalf("unexpected second parent children: %+v", parents[1].Cells)
	}
}

func TestProjectParentChildrenSkipsMissingLeftJoinChildren(t *testing.T) {
	rows := []projectionFlatRow{
		{RowID: 10, RowKey: "r1", Position: 0},
	}

	parents, err := ProjectParentChildren(rows, ParentChildProjection[projectionFlatRow, projectionRow, projectionCell, int64]{
		ParentKey: func(row projectionFlatRow) int64 { return row.RowID },
		Parent:    func(row projectionFlatRow) projectionRow { return projectionRow{ID: row.RowID, Key: row.RowKey} },
		Child: func(row projectionFlatRow) (projectionCell, bool) {
			return projectionCell{Key: row.CellKey}, row.CellKey != ""
		},
		AppendChild: func(parent *projectionRow, child projectionCell) {
			parent.Cells = append(parent.Cells, child)
		},
	})
	if err != nil {
		t.Fatalf("project parent children: %v", err)
	}
	if len(parents) != 1 || len(parents[0].Cells) != 0 {
		t.Fatalf("unexpected projection: %+v", parents)
	}
}

func TestProjectParentChildrenValidatesSpec(t *testing.T) {
	_, err := ProjectParentChildren([]projectionFlatRow{{RowID: 1}}, ParentChildProjection[projectionFlatRow, projectionRow, projectionCell, int64]{})
	if err == nil || !strings.Contains(err.Error(), "parent key") {
		t.Fatalf("expected parent key validation error, got %v", err)
	}
}

func TestHydrateAggregatesPreservesParentOrder(t *testing.T) {
	parents := []projectionAggregateParent{
		{ID: 10, Title: "first"},
		{ID: 20, Title: "second"},
		{ID: 30, Title: "third"},
	}
	aggregates := []projectionAggregateRow{
		{ParentID: 20, OpenIssues: 3, NoticeCount: 5},
		{ParentID: 10, OpenIssues: 1, NoticeCount: 2},
		{ParentID: 999, OpenIssues: 9, NoticeCount: 9},
	}

	hydrated, err := HydrateAggregates(parents, aggregates, AggregateHydration[projectionAggregateParent, projectionAggregateRow, int64]{
		ParentKey:    func(parent projectionAggregateParent) int64 { return parent.ID },
		AggregateKey: func(row projectionAggregateRow) int64 { return row.ParentID },
		Apply: func(parent *projectionAggregateParent, row projectionAggregateRow) {
			parent.OpenIssues = row.OpenIssues
			parent.NoticeCount = row.NoticeCount
		},
	})
	if err != nil {
		t.Fatalf("hydrate aggregates: %v", err)
	}
	if hydrated[0].ID != 10 || hydrated[0].OpenIssues != 1 || hydrated[0].NoticeCount != 2 {
		t.Fatalf("unexpected first parent: %+v", hydrated[0])
	}
	if hydrated[1].ID != 20 || hydrated[1].OpenIssues != 3 || hydrated[1].NoticeCount != 5 {
		t.Fatalf("unexpected second parent: %+v", hydrated[1])
	}
	if hydrated[2].ID != 30 || hydrated[2].OpenIssues != 0 || hydrated[2].NoticeCount != 0 {
		t.Fatalf("unexpected third parent: %+v", hydrated[2])
	}
	if parents[0].OpenIssues != 0 {
		t.Fatalf("expected input parents to be unchanged, got %+v", parents[0])
	}
}

func TestHydrateAggregatesRejectsDuplicateAggregateKeys(t *testing.T) {
	_, err := HydrateAggregates(
		[]projectionAggregateParent{{ID: 1}},
		[]projectionAggregateRow{{ParentID: 1}, {ParentID: 1}},
		AggregateHydration[projectionAggregateParent, projectionAggregateRow, int64]{
			ParentKey:    func(parent projectionAggregateParent) int64 { return parent.ID },
			AggregateKey: func(row projectionAggregateRow) int64 { return row.ParentID },
			Apply:        func(parent *projectionAggregateParent, row projectionAggregateRow) {},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "duplicate aggregate") {
		t.Fatalf("expected duplicate aggregate error, got %v", err)
	}
}

func TestHydrateAggregatesValidatesSpec(t *testing.T) {
	_, err := HydrateAggregates[projectionAggregateParent, projectionAggregateRow, int64](
		[]projectionAggregateParent{{ID: 1}},
		nil,
		AggregateHydration[projectionAggregateParent, projectionAggregateRow, int64]{},
	)
	if err == nil || !strings.Contains(err.Error(), "parent key") {
		t.Fatalf("expected parent key validation error, got %v", err)
	}
}

func TestGroupRepresentativeRowsCountsAndPreservesOrder(t *testing.T) {
	rows := []projectionFlatRow{
		{RowID: 10, RowKey: "client-a", CellValue: "first-a"},
		{RowID: 20, RowKey: "client-b", CellValue: "first-b"},
		{RowID: 11, RowKey: "client-a", CellValue: "second-a"},
		{RowID: 12, RowKey: "client-a", CellValue: "third-a"},
	}

	groups, err := GroupRepresentativeRows(rows, func(row projectionFlatRow) string {
		return row.RowKey
	})
	if err != nil {
		t.Fatalf("group representative rows: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected two groups, got %+v", groups)
	}
	if groups[0].Key != "client-a" || groups[0].Count != 3 || groups[0].Representative.CellValue != "first-a" {
		t.Fatalf("unexpected first group: %+v", groups[0])
	}
	if groups[1].Key != "client-b" || groups[1].Count != 1 || groups[1].Representative.CellValue != "first-b" {
		t.Fatalf("unexpected second group: %+v", groups[1])
	}
}

func TestGroupRepresentativeRowsValidatesKey(t *testing.T) {
	_, err := GroupRepresentativeRows[projectionFlatRow, string]([]projectionFlatRow{{RowID: 1}}, nil)
	if err == nil || !strings.Contains(err.Error(), "group key") {
		t.Fatalf("expected group key validation error, got %v", err)
	}
}

func TestApplyProjectionFillsMissingUnionColumns(t *testing.T) {
	db, _ := newScopeMockDB(t, driver.PostgresDialect{})

	clientBranch, err := ApplyProjection[projectionWorkItemRow](
		db.Table("clients"),
		map[string]ProjectionExpression{
			"resource_type": ProjectionSQL("'client'"),
			"resource_id":   ProjectionSQL("clients.id"),
			"title":         ProjectionSQL("clients.name"),
		},
	)
	if err != nil {
		t.Fatalf("apply client projection: %v", err)
	}
	noticeBranch, err := ApplyProjection[projectionWorkItemRow](
		db.Table("notices"),
		map[string]ProjectionExpression{
			"resource_type": ProjectionSQL("'notice'"),
			"resource_id":   ProjectionSQL("notices.id"),
			"title":         ProjectionSQL("notices.subject"),
			"notice_count":  ProjectionSQL("1"),
		},
	)
	if err != nil {
		t.Fatalf("apply notice projection: %v", err)
	}
	plan, err := clientBranch.UnionAll(noticeBranch).Plan(context.Background())
	if err != nil {
		t.Fatalf("plan union projection: %v", err)
	}
	for _, want := range []string{
		"'client' AS resource_type",
		"clients.id AS resource_id",
		"NULL AS notice_count",
		"UNION ALL",
		"1 AS notice_count",
	} {
		if !strings.Contains(plan.SQL, want) {
			t.Fatalf("expected %q in sql=%s", want, plan.SQL)
		}
	}
}

func TestApplyProjectionRejectsUnknownExpressionColumn(t *testing.T) {
	db, _ := newScopeMockDB(t, driver.PostgresDialect{})

	_, err := ApplyProjection[projectionWorkItemRow](
		db.Table("clients"),
		map[string]ProjectionExpression{
			"missing": ProjectionSQL("clients.id"),
		},
	)
	if err == nil || !strings.Contains(err.Error(), "not in destination type") {
		t.Fatalf("expected unknown projection column error, got %v", err)
	}
}
