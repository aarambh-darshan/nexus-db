// Package mysql provides MySQL dialect implementation.
package mysql

import (
	"context"
	"database/sql"
	"strings"

	"github.com/nexus-db/nexus/pkg/core/migration"
)

// IntrospectTables returns all user table names in the database.
func (d *Dialect) IntrospectTables(ctx context.Context, db *sql.DB) ([]string, error) {
	query := `SELECT table_name 
		FROM information_schema.tables 
		WHERE table_schema = DATABASE()
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
		column_name,
		data_type,
		is_nullable,
		column_default,
		column_key,
		extra
	FROM information_schema.columns
	WHERE table_name = ? AND table_schema = DATABASE()
	ORDER BY ordinal_position`

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
		var columnKey string
		var extra string

		if err := rows.Scan(&name, &colType, &nullable, &defaultVal, &columnKey, &extra); err != nil {
			return nil, err
		}

		col := &migration.ColumnInfo{
			Name:         name,
			Type:         colType,
			Nullable:     nullable == "YES",
			IsPrimaryKey: columnKey == "PRI",
			IsUnique:     columnKey == "UNI",
			Default:      defaultVal.String,
			AutoInc:      strings.Contains(extra, "auto_increment"),
		}

		columns = append(columns, col)
	}

	return columns, rows.Err()
}

// IntrospectIndexes returns index metadata for a table.
func (d *Dialect) IntrospectIndexes(ctx context.Context, db *sql.DB, tableName string) ([]*migration.IndexInfo, error) {
	query := `SHOW INDEX FROM ` + d.Quote(tableName)

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexMap := make(map[string]*migration.IndexInfo)
	for rows.Next() {
		var table, keyName, colName, indexType string
		var nonUnique, seqInIndex int
		var collation, cardinality, subPart, packed, null, comment, indexComment, visible, expression sql.NullString

		// SHOW INDEX has many columns, we only care about a few
		if err := rows.Scan(
			&table, &nonUnique, &keyName, &seqInIndex, &colName,
			&collation, &cardinality, &subPart, &packed, &null,
			&indexType, &comment, &indexComment, &visible, &expression,
		); err != nil {
			// Try simpler scan for older MySQL versions
			if err := rows.Scan(
				&table, &nonUnique, &keyName, &seqInIndex, &colName,
				&collation, &cardinality, &subPart, &packed, &null,
				&indexType, &comment, &indexComment,
			); err != nil {
				return nil, err
			}
		}

		// Skip PRIMARY key
		if keyName == "PRIMARY" {
			continue
		}

		if _, exists := indexMap[keyName]; !exists {
			indexMap[keyName] = &migration.IndexInfo{
				Name:   keyName,
				Unique: nonUnique == 0,
			}
		}
		indexMap[keyName].Columns = append(indexMap[keyName].Columns, colName)
	}

	var indexes []*migration.IndexInfo
	for _, idx := range indexMap {
		indexes = append(indexes, idx)
	}

	return indexes, rows.Err()
}
