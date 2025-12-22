// Package sqlite provides SQLite dialect implementation.
package sqlite

import (
	"context"
	"database/sql"
	"strings"

	"github.com/nexus-db/nexus/pkg/core/migration"
)

// IntrospectTables returns all user table names in the database.
func (d *Dialect) IntrospectTables(ctx context.Context, db *sql.DB) ([]string, error) {
	query := `SELECT name FROM sqlite_master 
		WHERE type='table' 
		AND name NOT LIKE 'sqlite_%'
		ORDER BY name`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}

	return tables, rows.Err()
}

// IntrospectColumns returns column metadata for a table.
func (d *Dialect) IntrospectColumns(ctx context.Context, db *sql.DB, tableName string) ([]*migration.ColumnInfo, error) {
	query := `PRAGMA table_info("` + tableName + `")`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []*migration.ColumnInfo
	for rows.Next() {
		var cid int
		var name string
		var colType string
		var notNull int
		var defaultVal sql.NullString
		var pk int

		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultVal, &pk); err != nil {
			return nil, err
		}

		col := &migration.ColumnInfo{
			Name:         name,
			Type:         colType,
			Nullable:     notNull == 0,
			IsPrimaryKey: pk > 0,
			Default:      defaultVal.String,
		}

		// Check if primary key is autoincrement
		if pk > 0 && strings.ToUpper(colType) == "INTEGER" {
			col.AutoInc = true // SQLite INTEGER PRIMARY KEY is auto-increment
		}

		columns = append(columns, col)
	}

	// Check for unique constraints
	uniqueQuery := `PRAGMA index_list("` + tableName + `")`
	indexRows, err := db.QueryContext(ctx, uniqueQuery)
	if err != nil {
		return columns, nil // Return columns even if we can't get unique info
	}
	defer indexRows.Close()

	uniqueColumns := make(map[string]bool)
	for indexRows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int

		if err := indexRows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			continue
		}

		if unique == 1 && origin == "u" { // Unique constraint from CREATE TABLE
			// Get the columns in this index
			infoQuery := `PRAGMA index_info("` + name + `")`
			infoRows, err := db.QueryContext(ctx, infoQuery)
			if err != nil {
				continue
			}

			for infoRows.Next() {
				var seqNo, cid int
				var colName string
				if err := infoRows.Scan(&seqNo, &cid, &colName); err != nil {
					continue
				}
				uniqueColumns[colName] = true
			}
			infoRows.Close()
		}
	}

	// Update columns with unique info
	for _, col := range columns {
		if uniqueColumns[col.Name] {
			col.IsUnique = true
		}
	}

	return columns, rows.Err()
}

// IntrospectIndexes returns index metadata for a table.
func (d *Dialect) IntrospectIndexes(ctx context.Context, db *sql.DB, tableName string) ([]*migration.IndexInfo, error) {
	query := `PRAGMA index_list("` + tableName + `")`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []*migration.IndexInfo
	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int

		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, err
		}

		// Skip auto-generated indexes
		if strings.HasPrefix(name, "sqlite_autoindex_") {
			continue
		}

		idx := &migration.IndexInfo{
			Name:   name,
			Unique: unique == 1,
		}

		// Get columns in this index
		infoQuery := `PRAGMA index_info("` + name + `")`
		infoRows, err := db.QueryContext(ctx, infoQuery)
		if err != nil {
			continue
		}

		for infoRows.Next() {
			var seqNo, cid int
			var colName string
			if err := infoRows.Scan(&seqNo, &cid, &colName); err != nil {
				continue
			}
			idx.Columns = append(idx.Columns, colName)
		}
		infoRows.Close()

		indexes = append(indexes, idx)
	}

	return indexes, rows.Err()
}
