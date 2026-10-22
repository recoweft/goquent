package scanner

import (
	"database/sql"

	"github.com/recoweft/goquent/orm/internal/rowscan"
)

// Struct scans one row, preserving the concrete database/sql API.
func Struct(dest any, rows *sql.Rows) error         { return rowscan.Struct(dest, rows) }
func Map(rows *sql.Rows) (map[string]any, error)    { return rowscan.Map(rows) }
func Maps(rows *sql.Rows) ([]map[string]any, error) { return rowscan.Maps(rows) }
func Structs(dest any, rows *sql.Rows) error        { return rowscan.Structs(dest, rows) }
