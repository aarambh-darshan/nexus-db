// Package sqlite provides SQLite dialect implementation.
package sqlite

import (
	"fmt"
	"strings"

	"github.com/nexus-db/nexus/pkg/core/schema"
)

// Dialect implements the SQLite dialect.
type Dialect struct{}

// New creates a new SQLite dialect.
func New() *Dialect {
	return &Dialect{}
}

// Name returns the dialect name.
func (d *Dialect) Name() string {
	return "sqlite"
}

// DriverName returns the Go sql driver name.
func (d *Dialect) DriverName() string {
	return "sqlite3"
}

// Quote quotes an identifier.
func (d *Dialect) Quote(identifier string) string {
	return `"` + identifier + `"`
}

// Placeholder returns the parameter placeholder.
func (d *Dialect) Placeholder(index int) string {
	return "?"
}

// TypeMapping maps schema field types to SQLite types.
func (d *Dialect) TypeMapping(field *schema.Field) string {
	switch field.Type {
	case schema.FieldTypeInt:
		return "INTEGER"
	case schema.FieldTypeBigInt:
		return "INTEGER"
	case schema.FieldTypeString:
		return "TEXT"
	case schema.FieldTypeText:
		return "TEXT"
	case schema.FieldTypeBool:
		return "INTEGER" // SQLite uses 0/1 for bool
	case schema.FieldTypeFloat:
		return "REAL"
	case schema.FieldTypeDecimal:
		return "REAL" // SQLite doesn't have native DECIMAL
	case schema.FieldTypeDateTime:
		return "TEXT" // ISO8601 string
	case schema.FieldTypeDate:
		return "TEXT"
	case schema.FieldTypeTime:
		return "TEXT"
	case schema.FieldTypeJSON:
		return "TEXT"
	case schema.FieldTypeBytes:
		return "BLOB"
	case schema.FieldTypeUUID:
		return "TEXT"
	default:
		return "TEXT"
	}
}

// CreateTableSQL generates CREATE TABLE statement.
func (d *Dialect) CreateTableSQL(model *schema.Model) string {
	var columns []string
	var constraints []string

	for _, field := range model.GetFields() {
		col := d.columnDefinition(field)
		columns = append(columns, col)
	}

	// Add indexes that should be inline (primary key is handled in column definition)
	for _, idx := range model.Indexes {
		if idx.Unique && len(idx.Fields) > 1 {
			quotedFields := make([]string, len(idx.Fields))
			for i, f := range idx.Fields {
				quotedFields[i] = d.Quote(f)
			}
			constraints = append(constraints, fmt.Sprintf("UNIQUE (%s)", strings.Join(quotedFields, ", ")))
		}
	}

	allParts := append(columns, constraints...)
	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n  %s\n)",
		d.Quote(model.Name),
		strings.Join(allParts, ",\n  "))
}

func (d *Dialect) columnDefinition(field *schema.Field) string {
	parts := []string{d.Quote(field.Name), d.TypeMapping(field)}

	if field.IsPrimaryKey {
		parts = append(parts, "PRIMARY KEY")
		if field.AutoIncrement {
			parts = append(parts, "AUTOINCREMENT")
		}
	}

	if !field.Nullable && !field.IsPrimaryKey {
		parts = append(parts, "NOT NULL")
	}

	if field.IsUnique && !field.IsPrimaryKey {
		parts = append(parts, "UNIQUE")
	}

	if field.DefaultExpr != "" {
		// Map common expressions to SQLite equivalents
		expr := field.DefaultExpr
		switch strings.ToUpper(expr) {
		case "NOW()":
			expr = "CURRENT_TIMESTAMP"
		case "UUID()":
			expr = "(lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))),2) || '-' || substr('89ab',abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))),2) || '-' || lower(hex(randomblob(6))))"
		}
		parts = append(parts, "DEFAULT "+expr)
	} else if field.DefaultValue != nil {
		switch v := field.DefaultValue.(type) {
		case string:
			parts = append(parts, fmt.Sprintf("DEFAULT '%s'", v))
		case bool:
			if v {
				parts = append(parts, "DEFAULT 1")
			} else {
				parts = append(parts, "DEFAULT 0")
			}
		default:
			parts = append(parts, fmt.Sprintf("DEFAULT %v", v))
		}
	}

	return strings.Join(parts, " ")
}

// DropTableSQL generates DROP TABLE statement.
func (d *Dialect) DropTableSQL(tableName string) string {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s", d.Quote(tableName))
}

// CreateIndexSQL generates CREATE INDEX statement.
func (d *Dialect) CreateIndexSQL(tableName string, index *schema.Index) string {
	unique := ""
	if index.Unique {
		unique = "UNIQUE "
	}

	quotedFields := make([]string, len(index.Fields))
	for i, f := range index.Fields {
		quotedFields[i] = d.Quote(f)
	}

	return fmt.Sprintf("CREATE %sINDEX IF NOT EXISTS %s ON %s (%s)",
		unique,
		d.Quote(index.Name),
		d.Quote(tableName),
		strings.Join(quotedFields, ", "))
}

// DropIndexSQL generates DROP INDEX statement.
func (d *Dialect) DropIndexSQL(tableName, indexName string) string {
	return fmt.Sprintf("DROP INDEX IF EXISTS %s", d.Quote(indexName))
}

// AddColumnSQL generates ALTER TABLE ADD COLUMN statement.
func (d *Dialect) AddColumnSQL(tableName string, field *schema.Field) string {
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s",
		d.Quote(tableName),
		d.columnDefinition(field))
}

// DropColumnSQL generates ALTER TABLE DROP COLUMN statement.
// Note: SQLite < 3.35.0 doesn't support DROP COLUMN.
func (d *Dialect) DropColumnSQL(tableName, columnName string) string {
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s",
		d.Quote(tableName),
		d.Quote(columnName))
}

// RenameColumnSQL generates ALTER TABLE RENAME COLUMN statement.
func (d *Dialect) RenameColumnSQL(tableName, oldName, newName string) string {
	return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
		d.Quote(tableName),
		d.Quote(oldName),
		d.Quote(newName))
}

// SupportsReturning returns true if RETURNING clause is supported.
// SQLite 3.35.0+ supports RETURNING.
func (d *Dialect) SupportsReturning() bool {
	return true
}

// SupportsUpsert returns true if upsert is supported.
func (d *Dialect) SupportsUpsert() bool {
	return true
}

// ExplainSQL wraps query with EXPLAIN QUERY PLAN for SQLite.
func (d *Dialect) ExplainSQL(query string, format string, analyze bool) string {
	// SQLite uses EXPLAIN QUERY PLAN for query plans
	// EXPLAIN gives bytecode which is less useful for optimization
	return "EXPLAIN QUERY PLAN " + query
}

// SupportsExplainFormat returns supported formats for SQLite.
func (d *Dialect) SupportsExplainFormat(format string) bool {
	// SQLite only supports text format
	return format == "text" || format == ""
}
