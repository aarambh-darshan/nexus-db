// Package query provides a fluent query builder for database operations.
package query

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/nexus-db/nexus/pkg/core/schema"
	"github.com/nexus-db/nexus/pkg/dialects"
)

// Builder is the main query builder.
type Builder struct {
	conn      *dialects.Connection
	tableName string
	schema    *schema.Schema
}

// New creates a new query builder for the given table.
func New(conn *dialects.Connection, tableName string) *Builder {
	return &Builder{
		conn:      conn,
		tableName: tableName,
	}
}

// NewWithSchema creates a query builder with schema awareness for eager loading.
func NewWithSchema(conn *dialects.Connection, tableName string, sch *schema.Schema) *Builder {
	return &Builder{
		conn:      conn,
		tableName: tableName,
		schema:    sch,
	}
}

// Select creates a SELECT query builder.
func (b *Builder) Select(columns ...string) *SelectBuilder {
	return &SelectBuilder{
		conn:      b.conn,
		tableName: b.tableName,
		columns:   columns,
		schema:    b.schema,
	}
}

// Insert creates an INSERT query builder.
func (b *Builder) Insert(data map[string]interface{}) *InsertBuilder {
	return &InsertBuilder{
		conn:      b.conn,
		tableName: b.tableName,
		data:      data,
	}
}

// Update creates an UPDATE query builder.
func (b *Builder) Update(data map[string]interface{}) *UpdateBuilder {
	return &UpdateBuilder{
		conn:      b.conn,
		tableName: b.tableName,
		data:      data,
	}
}

// Delete creates a DELETE query builder.
func (b *Builder) Delete() *DeleteBuilder {
	return &DeleteBuilder{
		conn:      b.conn,
		tableName: b.tableName,
		schema:    b.schema,
	}
}

// Condition represents a WHERE condition.
type Condition struct {
	Column   string
	Operator string
	Value    interface{}
	Raw      string // For raw SQL conditions
}

// Eq creates an equality condition.
func Eq(column string, value interface{}) Condition {
	return Condition{Column: column, Operator: "=", Value: value}
}

// Neq creates a not-equal condition.
func Neq(column string, value interface{}) Condition {
	return Condition{Column: column, Operator: "!=", Value: value}
}

// Gt creates a greater-than condition.
func Gt(column string, value interface{}) Condition {
	return Condition{Column: column, Operator: ">", Value: value}
}

// Gte creates a greater-than-or-equal condition.
func Gte(column string, value interface{}) Condition {
	return Condition{Column: column, Operator: ">=", Value: value}
}

// Lt creates a less-than condition.
func Lt(column string, value interface{}) Condition {
	return Condition{Column: column, Operator: "<", Value: value}
}

// Lte creates a less-than-or-equal condition.
func Lte(column string, value interface{}) Condition {
	return Condition{Column: column, Operator: "<=", Value: value}
}

// Like creates a LIKE condition.
func Like(column string, pattern string) Condition {
	return Condition{Column: column, Operator: "LIKE", Value: pattern}
}

// In creates an IN condition.
func In(column string, values ...interface{}) Condition {
	return Condition{Column: column, Operator: "IN", Value: values}
}

// IsNull creates an IS NULL condition.
func IsNull(column string) Condition {
	return Condition{Column: column, Operator: "IS NULL", Value: nil}
}

// IsNotNull creates an IS NOT NULL condition.
func IsNotNull(column string) Condition {
	return Condition{Column: column, Operator: "IS NOT NULL", Value: nil}
}

// RawSQL creates a raw SQL condition for WHERE clauses.
func RawSQL(sql string) Condition {
	return Condition{Raw: sql}
}

// OrderDirection represents sort direction.
type OrderDirection int

const (
	Asc OrderDirection = iota
	Desc
)

// String returns the SQL representation.
func (d OrderDirection) String() string {
	if d == Desc {
		return "DESC"
	}
	return "ASC"
}

// OrderBy represents an ORDER BY clause item.
type OrderBy struct {
	Column    string
	Direction OrderDirection
}

// buildWhere builds the WHERE clause and returns SQL fragment and args.
func buildWhere(dialect dialects.Dialect, conditions []Condition, startIndex int) (string, []interface{}) {
	if len(conditions) == 0 {
		return "", nil
	}

	var parts []string
	var args []interface{}
	argIndex := startIndex

	for _, cond := range conditions {
		if cond.Raw != "" {
			parts = append(parts, cond.Raw)
			continue
		}

		quotedCol := dialect.Quote(cond.Column)

		switch cond.Operator {
		case "IS NULL", "IS NOT NULL":
			parts = append(parts, fmt.Sprintf("%s %s", quotedCol, cond.Operator))
		case "IN":
			values := cond.Value.([]interface{})
			placeholders := make([]string, len(values))
			for i, v := range values {
				placeholders[i] = dialect.Placeholder(argIndex)
				args = append(args, v)
				argIndex++
			}
			parts = append(parts, fmt.Sprintf("%s IN (%s)", quotedCol, strings.Join(placeholders, ", ")))
		case "IN_SUBQUERY":
			// Handle subquery IN
			if subquery, ok := cond.Value.(*SelectBuilder); ok {
				subSQL, subArgs := subquery.Build()
				parts = append(parts, fmt.Sprintf("%s IN (%s)", quotedCol, subSQL))
				args = append(args, subArgs...)
				argIndex += len(subArgs)
			}
		case "NOT_IN_SUBQUERY":
			// Handle subquery NOT IN
			if subquery, ok := cond.Value.(*SelectBuilder); ok {
				subSQL, subArgs := subquery.Build()
				parts = append(parts, fmt.Sprintf("%s NOT IN (%s)", quotedCol, subSQL))
				args = append(args, subArgs...)
				argIndex += len(subArgs)
			}
		case "EXISTS":
			// Handle EXISTS subquery
			if subquery, ok := cond.Value.(*SelectBuilder); ok {
				subSQL, subArgs := subquery.Build()
				parts = append(parts, fmt.Sprintf("EXISTS (%s)", subSQL))
				args = append(args, subArgs...)
				argIndex += len(subArgs)
			}
		case "NOT_EXISTS":
			// Handle NOT EXISTS subquery
			if subquery, ok := cond.Value.(*SelectBuilder); ok {
				subSQL, subArgs := subquery.Build()
				parts = append(parts, fmt.Sprintf("NOT EXISTS (%s)", subSQL))
				args = append(args, subArgs...)
				argIndex += len(subArgs)
			}
		default:
			parts = append(parts, fmt.Sprintf("%s %s %s", quotedCol, cond.Operator, dialect.Placeholder(argIndex)))
			args = append(args, cond.Value)
			argIndex++
		}
	}

	return "WHERE " + strings.Join(parts, " AND "), args
}

// Result represents a query result row.
type Result map[string]interface{}

// Results represents multiple query result rows.
type Results []Result

// scanRows scans sql.Rows into Results.
func scanRows(rows *sql.Rows) (Results, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results Results
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(Result)
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	return results, rows.Err()
}

// Transaction runs a function within a transaction.
func Transaction(ctx context.Context, conn *dialects.Connection, fn func(tx *dialects.Tx) error) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}
