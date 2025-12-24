package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-db/nexus/pkg/core/schema"
	"github.com/nexus-db/nexus/pkg/dialects"
)

// DeleteBuilder builds DELETE queries.
type DeleteBuilder struct {
	conn       *dialects.Connection
	tableName  string
	conditions []Condition
	returning  []string
	schema     *schema.Schema
	cascade    bool
	profiler   *Profiler
}

// Where adds a WHERE condition.
func (d *DeleteBuilder) Where(conditions ...Condition) *DeleteBuilder {
	d.conditions = append(d.conditions, conditions...)
	return d
}

// Returning specifies columns to return from deleted rows.
func (d *DeleteBuilder) Returning(columns ...string) *DeleteBuilder {
	d.returning = columns
	return d
}

// WithSchema attaches schema for cascade operations.
func (d *DeleteBuilder) WithSchema(sch *schema.Schema) *DeleteBuilder {
	d.schema = sch
	return d
}

// Cascade enables cascade delete of related records.
// Requires schema to be set via WithSchema or NewWithSchema.
func (d *DeleteBuilder) Cascade() *DeleteBuilder {
	d.cascade = true
	return d
}

// Build generates the SQL query and arguments.
func (d *DeleteBuilder) Build() (string, []interface{}) {
	dialect := d.conn.Dialect
	var args []interface{}
	argIndex := 1

	sql := fmt.Sprintf("DELETE FROM %s", dialect.Quote(d.tableName))

	// WHERE clause
	if len(d.conditions) > 0 {
		whereSQL, whereArgs := buildWhere(dialect, d.conditions, argIndex)
		sql += " " + whereSQL
		args = append(args, whereArgs...)
	}

	// RETURNING clause
	if len(d.returning) > 0 && dialect.SupportsReturning() {
		retCols := make([]string, len(d.returning))
		for i, c := range d.returning {
			if c == "*" {
				retCols[i] = "*"
			} else {
				retCols[i] = dialect.Quote(c)
			}
		}
		sql += " RETURNING " + strings.Join(retCols, ", ")
	}

	return sql, args
}

// Exec executes the delete and returns the number of affected rows.
// If Cascade() is enabled and schema is set, related records are also deleted/nullified.
func (d *DeleteBuilder) Exec(ctx context.Context) (int64, error) {
	// For cascade, we need to fetch the rows first to know what to cascade
	if d.cascade && d.schema != nil {
		return d.execWithCascade(ctx)
	}

	query, args := d.Build()

	// Start profiling if enabled
	var profile *QueryProfile
	if d.profiler != nil && d.profiler.IsEnabled() {
		profile = d.profiler.StartQuery(query, args)
	}

	result, err := d.conn.Exec(ctx, query, args...)

	// Record profiling data
	if profile != nil {
		if err == nil {
			affected, _ := result.RowsAffected()
			profile.RowsAffected = affected
		}
		d.profiler.EndQuery(profile, err)
	}

	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// execWithCascade performs delete with cascade to related records.
func (d *DeleteBuilder) execWithCascade(ctx context.Context) (int64, error) {
	dialect := d.conn.Dialect

	// First, SELECT the rows to be deleted
	selectQuery, selectArgs := d.buildSelect()
	rows, err := d.conn.Query(ctx, selectQuery, selectArgs...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	toDelete, err := scanRows(rows)
	if err != nil {
		return 0, err
	}

	if len(toDelete) == 0 {
		return 0, nil
	}

	// Cascade to related records first
	if err := cascadeDelete(ctx, d.conn, d.schema, d.tableName, toDelete); err != nil {
		return 0, err
	}

	// Now delete the actual rows
	// Build WHERE clause matching the fetched primary keys
	model := findModelByTable(d.schema, d.tableName)
	pkField := "id"
	if model != nil {
		for _, field := range model.GetFields() {
			if field.IsPrimaryKey {
				pkField = field.Name
				break
			}
		}
	}

	pkValues := collectFieldValues(toDelete, pkField)
	placeholders := make([]string, len(pkValues))
	for i := range pkValues {
		placeholders[i] = dialect.Placeholder(i + 1)
	}

	deleteQuery := fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)",
		dialect.Quote(d.tableName),
		dialect.Quote(pkField),
		strings.Join(placeholders, ", "))

	result, err := d.conn.Exec(ctx, deleteQuery, pkValues...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// buildSelect builds a SELECT query with the same conditions.
func (d *DeleteBuilder) buildSelect() (string, []interface{}) {
	dialect := d.conn.Dialect
	var args []interface{}
	argIndex := 1

	sql := fmt.Sprintf("SELECT * FROM %s", dialect.Quote(d.tableName))

	if len(d.conditions) > 0 {
		whereSQL, whereArgs := buildWhere(dialect, d.conditions, argIndex)
		sql += " " + whereSQL
		args = append(args, whereArgs...)
	}

	return sql, args
}

// All executes the delete and returns all deleted rows (requires RETURNING).
func (d *DeleteBuilder) All(ctx context.Context) (Results, error) {
	if !d.conn.Dialect.SupportsReturning() {
		return nil, fmt.Errorf("dialect %s does not support RETURNING clause", d.conn.Dialect.Name())
	}

	if len(d.returning) == 0 {
		d.returning = []string{"*"}
	}

	query, args := d.Build()
	rows, err := d.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRows(rows)
}

// One executes the delete and returns the first deleted row (requires RETURNING).
func (d *DeleteBuilder) One(ctx context.Context) (Result, error) {
	results, err := d.All(ctx)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}
