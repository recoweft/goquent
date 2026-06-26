package tests

import (
	"context"
	"testing"

	"github.com/recoweft/goquent/orm"
	"github.com/recoweft/goquent/orm/query"
)

type nestedMySQLDocumentTable struct {
	ID       int64  `db:"id,pk"`
	TenantID string `db:"tenant_id"`
	Title    string `db:"title"`
	Revision int64  `db:"revision"`
}

func (nestedMySQLDocumentTable) TableName() string { return "document_tables" }

type nestedMySQLDocumentRow struct {
	ID           int64  `db:"id,pk,omitempty"`
	TenantID     string `db:"tenant_id"`
	TableID      int64  `db:"table_id"`
	StableRowKey string `db:"stable_row_key"`
	Position     int    `db:"position"`
}

func (nestedMySQLDocumentRow) TableName() string { return "document_table_rows" }

type nestedMySQLDocumentCell struct {
	TenantID      string `db:"tenant_id"`
	TableID       int64  `db:"table_id"`
	RowID         int64  `db:"row_id"`
	StableCellKey string `db:"stable_cell_key"`
	ValueJSON     string `db:"value_json"`
}

func (nestedMySQLDocumentCell) TableName() string { return "document_table_cells" }

func nestedMySQLWhere(col string, value any) orm.Scope {
	return func(q *query.Query) *query.Query {
		return q.Where(col, value)
	}
}

func setupNestedWriteTables(t testing.TB, db *orm.DB) {
	t.Helper()
	stdDB := db.SQLDB()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS document_tables (
			id BIGINT PRIMARY KEY,
			tenant_id VARCHAR(64) NOT NULL,
			title VARCHAR(255),
			revision BIGINT NOT NULL DEFAULT 0,
			UNIQUE KEY document_tables_tenant_id_id_key (tenant_id, id)
		)`,
		`CREATE TABLE IF NOT EXISTS document_table_rows (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			tenant_id VARCHAR(64) NOT NULL,
			table_id BIGINT NOT NULL,
			stable_row_key VARCHAR(64) NOT NULL,
			position INT NOT NULL,
			UNIQUE KEY document_table_rows_stable_key (tenant_id, table_id, stable_row_key)
		)`,
		`CREATE TABLE IF NOT EXISTS document_table_cells (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			tenant_id VARCHAR(64) NOT NULL,
			table_id BIGINT NOT NULL,
			row_id BIGINT NOT NULL,
			stable_cell_key VARCHAR(64) NOT NULL,
			value_json TEXT,
			UNIQUE KEY document_table_cells_stable_key (tenant_id, table_id, stable_cell_key)
		)`,
		`TRUNCATE TABLE document_table_cells`,
		`TRUNCATE TABLE document_table_rows`,
		`TRUNCATE TABLE document_tables`,
	}
	for _, stmt := range stmts {
		if _, err := stdDB.Exec(stmt); err != nil {
			t.Fatalf("nested write setup statement %q: %v", stmt, err)
		}
	}
}

func nestedWriteDeleteSteps(tenantID string, tableID int64) []orm.NestedDelete {
	return []orm.NestedDelete{
		{Table: "document_table_cells", Scopes: []orm.Scope{orm.TenantScope(tenantID), nestedMySQLWhere("table_id", tableID)}},
		{Table: "document_table_rows", Scopes: []orm.Scope{orm.TenantScope(tenantID), nestedMySQLWhere("table_id", tableID)}},
	}
}

func nestedWriteSpec(tenantID string, tableID int64, title string, revision int64, rows []nestedMySQLDocumentRow) orm.NestedCollectionReplace[nestedMySQLDocumentTable, nestedMySQLDocumentRow, nestedMySQLDocumentCell] {
	return orm.NestedCollectionReplace[nestedMySQLDocumentTable, nestedMySQLDocumentRow, nestedMySQLDocumentCell]{
		Parent: nestedMySQLDocumentTable{ID: tableID, TenantID: tenantID, Title: title, Revision: revision},
		ParentOpts: []orm.WriteOpt{
			orm.WherePK(),
			orm.UpdateColumns("tenant_id", "title", "revision"),
		},
		DeleteBefore:  nestedWriteDeleteSteps(tenantID, tableID),
		Children:      rows,
		ChildIDColumn: "id",
		Grandchildren: func(_ int, row nestedMySQLDocumentRow, rowID int64) ([]nestedMySQLDocumentCell, error) {
			return []nestedMySQLDocumentCell{{
				TenantID:      row.TenantID,
				TableID:       row.TableID,
				RowID:         rowID,
				StableCellKey: row.StableRowKey + ":c1",
				ValueJSON:     `{"text":"` + row.StableRowKey + `"}`,
			}}, nil
		},
		GrandchildMode: orm.NestedWriteUpsert,
		GrandchildOpts: []orm.WriteOpt{
			orm.ConflictColumns("tenant_id", "table_id", "stable_cell_key"),
			orm.UpdateColumns("row_id", "value_json"),
		},
	}
}

func TestMySQLReplaceNestedCollectionTx(t *testing.T) {
	dsn, explicit := lookupTestDSN("TEST_MYSQL_DSN", defaultMySQLTestDSN)
	db := openTestDB(t, orm.MySQL, dsn, explicit)
	defer db.Close()
	setupNestedWriteTables(t, db)

	ctx := context.Background()
	tenantID := "tenant-nested"
	tableID := int64(100)
	firstRows := []nestedMySQLDocumentRow{
		{TenantID: tenantID, TableID: tableID, StableRowKey: "r1", Position: 0},
		{TenantID: tenantID, TableID: tableID, StableRowKey: "r2", Position: 1},
	}
	result, err := orm.ReplaceNestedCollectionTx(ctx, db, nestedWriteSpec(tenantID, tableID, "first", 1, firstRows))
	if err != nil {
		t.Fatalf("replace nested collection tx first: %v", err)
	}
	if len(result.ChildIDs) != 2 || result.ChildIDs[0] == 0 || result.ChildIDs[1] != result.ChildIDs[0]+1 {
		t.Fatalf("unexpected generated row ids: %#v", result.ChildIDs)
	}
	if result.GrandchildCount != 2 {
		t.Fatalf("expected two cells, got %d", result.GrandchildCount)
	}

	secondRows := []nestedMySQLDocumentRow{
		{TenantID: tenantID, TableID: tableID, StableRowKey: "r2", Position: 0},
	}
	if _, err := orm.ReplaceNestedCollectionTx(ctx, db, nestedWriteSpec(tenantID, tableID, "second", 2, secondRows)); err != nil {
		t.Fatalf("replace nested collection tx second: %v", err)
	}

	var revision int64
	if err := rawQueryRow(t, db, ctx, "SELECT revision FROM document_tables WHERE tenant_id = ? AND id = ?", tenantID, tableID).Scan(&revision); err != nil {
		t.Fatalf("select table revision: %v", err)
	}
	if revision != 2 {
		t.Fatalf("expected revision 2, got %d", revision)
	}
	var rowCount int
	if err := rawQueryRow(t, db, ctx, "SELECT COUNT(*) FROM document_table_rows WHERE tenant_id = ? AND table_id = ?", tenantID, tableID).Scan(&rowCount); err != nil {
		t.Fatalf("select row count: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("expected one row after replacement, got %d", rowCount)
	}
	var cellCount int
	if err := rawQueryRow(t, db, ctx, "SELECT COUNT(*) FROM document_table_cells WHERE tenant_id = ? AND table_id = ?", tenantID, tableID).Scan(&cellCount); err != nil {
		t.Fatalf("select cell count: %v", err)
	}
	if cellCount != 1 {
		t.Fatalf("expected one cell after replacement, got %d", cellCount)
	}
	var cellRowKey string
	if err := rawQueryRow(
		t,
		db,
		ctx,
		`SELECT r.stable_row_key
		   FROM document_table_cells c
		   JOIN document_table_rows r ON r.id = c.row_id
		  WHERE c.tenant_id = ? AND c.table_id = ? AND c.stable_cell_key = ?`,
		tenantID,
		tableID,
		"r2:c1",
	).Scan(&cellRowKey); err != nil {
		t.Fatalf("select cell row key: %v", err)
	}
	if cellRowKey != "r2" {
		t.Fatalf("expected cell to point at r2, got %s", cellRowKey)
	}
}
