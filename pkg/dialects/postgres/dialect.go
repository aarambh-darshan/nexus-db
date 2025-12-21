// Package postgres provides PostgreSQL dialect implementation.
package postgres

import (
	"fmt"
	"strings"

	"github.com/nexus-db/nexus/pkg/core/schema"
)

// Dialect implements the PostgreSQL dialect.
type Dialect struct{}

// New creates a new PostgreSQL dialect.
func New() *Dialect {
	return &Dialect{}
}

// Name returns the dialect name.
func (d *Dialect) Name() string {
	return "postgres"
}

// DriverName returns the Go sql driver name.
func (d *Dialect) DriverName() string {
	return "postgres"
}

// Quote quotes an identifier.
func (d *Dialect) Quote(identifier string) string {
	return `"` + identifier + `"`
}

// Placeholder returns the parameter placeholder.
func (d *Dialect) Placeholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

// TypeMapping maps schema field types to PostgreSQL types.
func (d *Dialect) TypeMapping(field *schema.Field) string {
	switch field.Type {
	case schema.FieldTypeInt:
		if field.AutoIncrement {
			return "SERIAL"
		}
		return "INTEGER"
	case schema.FieldTypeBigInt:
		if field.AutoIncrement {
			return "BIGSERIAL"
		}
		return "BIGINT"
	case schema.FieldTypeString:
		if field.Length > 0 {
			return fmt.Sprintf("VARCHAR(%d)", field.Length)
		}
		return "VARCHAR(255)"
	case schema.FieldTypeText:
		return "TEXT"
	case schema.FieldTypeBool:
		return "BOOLEAN"
	case schema.FieldTypeFloat:
		return "DOUBLE PRECISION"
	case schema.FieldTypeDecimal:
		return fmt.Sprintf("NUMERIC(%d,%d)", field.Precision, field.Scale)
	case schema.FieldTypeDateTime:
		return "TIMESTAMP WITH TIME ZONE"
	case schema.FieldTypeDate:
		return "DATE"
	case schema.FieldTypeTime:
		return "TIME"
	case schema.FieldTypeJSON:
		return "JSONB"
	case schema.FieldTypeBytes:
		return "BYTEA"
	case schema.FieldTypeUUID:
		return "UUID"
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

	// Handle composite unique constraints
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
	}

	if !field.Nullable && !field.IsPrimaryKey {
		parts = append(parts, "NOT NULL")
	}

	if field.IsUnique && !field.IsPrimaryKey {
		parts = append(parts, "UNIQUE")
	}

	if field.DefaultExpr != "" {
		expr := field.DefaultExpr
		switch strings.ToUpper(expr) {
		case "NOW()":
			expr = "NOW()"
		case "UUID()":
			expr = "gen_random_uuid()"
		}
		parts = append(parts, "DEFAULT "+expr)
	} else if field.DefaultValue != nil {
		switch v := field.DefaultValue.(type) {
		case string:
			parts = append(parts, fmt.Sprintf("DEFAULT '%s'", v))
		case bool:
			if v {
				parts = append(parts, "DEFAULT TRUE")
			} else {
				parts = append(parts, "DEFAULT FALSE")
			}
		default:
			parts = append(parts, fmt.Sprintf("DEFAULT %v", v))
		}
	}

	return strings.Join(parts, " ")
}

// DropTableSQL generates DROP TABLE statement.
func (d *Dialect) DropTableSQL(tableName string) string {
	return fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", d.Quote(tableName))
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
func (d *Dialect) SupportsReturning() bool {
	return true
}

// SupportsUpsert returns true if upsert is supported.
func (d *Dialect) SupportsUpsert() bool {
	return true
}
