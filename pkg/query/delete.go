package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-db/nexus/pkg/dialects"
)

// DeleteBuilder builds DELETE queries.
type DeleteBuilder struct {
	conn       *dialects.Connection
	tableName  string
	conditions []Condition
	returning  []string
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
func (d *DeleteBuilder) Exec(ctx context.Context) (int64, error) {
	query, args := d.Build()
	result, err := d.conn.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
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
