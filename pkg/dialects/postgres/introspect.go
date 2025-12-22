// Package postgres provides PostgreSQL dialect implementation.
package postgres

import (
	"context"
	"database/sql"

	"github.com/nexus-db/nexus/pkg/core/migration"
)

// IntrospectTables returns all user table names in the database.
func (d *Dialect) IntrospectTables(ctx context.Context, db *sql.DB) ([]string, error) {
	query := `SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = 'public' 
		AND table_type = 'BASE TABLE'
		ORDER BY table_name`

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
	query := `SELECT 
		c.column_name,
		c.data_type,
		c.is_nullable,
		c.column_default,
		CASE WHEN pk.column_name IS NOT NULL THEN true ELSE false END as is_primary_key,
		CASE WHEN c.column_default LIKE 'nextval%' THEN true ELSE false END as is_auto_inc
	FROM information_schema.columns c
	LEFT JOIN (
		SELECT ku.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage ku 
			ON tc.constraint_name = ku.constraint_name
		WHERE tc.table_name = $1 
		AND tc.constraint_type = 'PRIMARY KEY'
	) pk ON c.column_name = pk.column_name
	WHERE c.table_name = $1 AND c.table_schema = 'public'
	ORDER BY c.ordinal_position`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []*migration.ColumnInfo
	for rows.Next() {
		var name string
		var colType string
		var nullable string
		var defaultVal sql.NullString
		var isPK bool
		var isAutoInc bool

		if err := rows.Scan(&name, &colType, &nullable, &defaultVal, &isPK, &isAutoInc); err != nil {
			return nil, err
		}

		col := &migration.ColumnInfo{
			Name:         name,
			Type:         colType,
			Nullable:     nullable == "YES",
			IsPrimaryKey: isPK,
			Default:      defaultVal.String,
			AutoInc:      isAutoInc,
		}

		columns = append(columns, col)
	}

	// Check for unique constraints
	uniqueQuery := `SELECT column_name 
		FROM information_schema.constraint_column_usage ccu
		JOIN information_schema.table_constraints tc 
			ON ccu.constraint_name = tc.constraint_name
		WHERE tc.table_name = $1 
		AND tc.constraint_type = 'UNIQUE'
		AND tc.table_schema = 'public'`

	uniqueRows, err := db.QueryContext(ctx, uniqueQuery, tableName)
	if err != nil {
		return columns, nil
	}
	defer uniqueRows.Close()

	uniqueColumns := make(map[string]bool)
	for uniqueRows.Next() {
		var colName string
		if err := uniqueRows.Scan(&colName); err != nil {
			continue
		}
		uniqueColumns[colName] = true
	}

	for _, col := range columns {
		if uniqueColumns[col.Name] {
			col.IsUnique = true
		}
	}

	return columns, rows.Err()
}

// IntrospectIndexes returns index metadata for a table.
func (d *Dialect) IntrospectIndexes(ctx context.Context, db *sql.DB, tableName string) ([]*migration.IndexInfo, error) {
	query := `SELECT 
		i.relname as index_name,
		ix.indisunique as is_unique,
		array_agg(a.attname ORDER BY array_position(ix.indkey, a.attnum)) as column_names
	FROM pg_class t
	JOIN pg_index ix ON t.oid = ix.indrelid
	JOIN pg_class i ON i.oid = ix.indexrelid
	JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = ANY(ix.indkey)
	WHERE t.relname = $1
	AND t.relkind = 'r'
	AND NOT ix.indisprimary
	GROUP BY i.relname, ix.indisunique
	ORDER BY i.relname`

	rows, err := db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []*migration.IndexInfo
	for rows.Next() {
		var name string
		var unique bool
		var columnsStr string

		if err := rows.Scan(&name, &unique, &columnsStr); err != nil {
			return nil, err
		}

		idx := &migration.IndexInfo{
			Name:   name,
			Unique: unique,
		}

		// Parse the array string {col1,col2}
		columnsStr = columnsStr[1 : len(columnsStr)-1] // Remove { }
		if columnsStr != "" {
			for _, col := range splitArray(columnsStr) {
				idx.Columns = append(idx.Columns, col)
			}
		}

		indexes = append(indexes, idx)
	}

	return indexes, rows.Err()
}

// splitArray splits a PostgreSQL array string like "col1,col2" into parts
func splitArray(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	var current string
	inQuote := false

	for _, r := range s {
		switch r {
		case '"':
			inQuote = !inQuote
		case ',':
			if !inQuote {
				result = append(result, current)
				current = ""
				continue
			}
			current += string(r)
		default:
			current += string(r)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
