// Package migration provides database migration functionality.
package migration

import (
	"context"
	"database/sql"
)

// ColumnInfo represents metadata about a database column.
type ColumnInfo struct {
	Name         string
	Type         string // The SQL type as returned by the database
	Nullable     bool
	IsPrimaryKey bool
	IsUnique     bool
	Default      string
	AutoInc      bool
}

// IndexInfo represents metadata about a database index.
type IndexInfo struct {
	Name    string
	Unique  bool
	Columns []string
}

// TableInfo represents metadata about a database table.
type TableInfo struct {
	Name    string
	Columns map[string]*ColumnInfo
	Indexes map[string]*IndexInfo
}

// DatabaseSnapshot represents the current state of the database.
type DatabaseSnapshot struct {
	Tables map[string]*TableInfo
}

// NewDatabaseSnapshot creates an empty snapshot.
func NewDatabaseSnapshot() *DatabaseSnapshot {
	return &DatabaseSnapshot{
		Tables: make(map[string]*TableInfo),
	}
}

// Introspector defines the interface for database introspection.
// Each dialect must implement this to enable schema diff detection.
type Introspector interface {
	// IntrospectTables returns all user table names in the database.
	IntrospectTables(ctx context.Context, db *sql.DB) ([]string, error)

	// IntrospectColumns returns column metadata for a table.
	IntrospectColumns(ctx context.Context, db *sql.DB, tableName string) ([]*ColumnInfo, error)

	// IntrospectIndexes returns index metadata for a table.
	IntrospectIndexes(ctx context.Context, db *sql.DB, tableName string) ([]*IndexInfo, error)
}

// IntrospectDatabase reads the current database schema using the provided introspector.
func IntrospectDatabase(ctx context.Context, db *sql.DB, introspector Introspector) (*DatabaseSnapshot, error) {
	snapshot := NewDatabaseSnapshot()

	// Get all tables
	tableNames, err := introspector.IntrospectTables(ctx, db)
	if err != nil {
		return nil, err
	}

	for _, tableName := range tableNames {
		tableInfo := &TableInfo{
			Name:    tableName,
			Columns: make(map[string]*ColumnInfo),
			Indexes: make(map[string]*IndexInfo),
		}

		// Get columns
		columns, err := introspector.IntrospectColumns(ctx, db, tableName)
		if err != nil {
			return nil, err
		}
		for _, col := range columns {
			tableInfo.Columns[col.Name] = col
		}

		// Get indexes
		indexes, err := introspector.IntrospectIndexes(ctx, db, tableName)
		if err != nil {
			return nil, err
		}
		for _, idx := range indexes {
			tableInfo.Indexes[idx.Name] = idx
		}

		snapshot.Tables[tableName] = tableInfo
	}

	return snapshot, nil
}
