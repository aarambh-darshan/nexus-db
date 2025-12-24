package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-db/nexus/pkg/dialects"
)

// UpdateBuilder builds UPDATE queries.
type UpdateBuilder struct {
	conn       *dialects.Connection
	tableName  string
	data       map[string]interface{}
	conditions []Condition
	returning  []string
	profiler   *Profiler
}

// Where adds a WHERE condition.
func (u *UpdateBuilder) Where(conditions ...Condition) *UpdateBuilder {
	u.conditions = append(u.conditions, conditions...)
	return u
}

// Returning specifies columns to return after update.
func (u *UpdateBuilder) Returning(columns ...string) *UpdateBuilder {
	u.returning = columns
	return u
}

// Set adds or updates a column value.
func (u *UpdateBuilder) Set(column string, value interface{}) *UpdateBuilder {
	if u.data == nil {
		u.data = make(map[string]interface{})
	}
	u.data[column] = value
	return u
}

// Build generates the SQL query and arguments.
func (u *UpdateBuilder) Build() (string, []interface{}) {
	dialect := u.conn.Dialect
	var args []interface{}
	argIndex := 1

	// Build SET clause
	sets := make([]string, 0, len(u.data))
	for col, val := range u.data {
		sets = append(sets, fmt.Sprintf("%s = %s", dialect.Quote(col), dialect.Placeholder(argIndex)))
		args = append(args, val)
		argIndex++
	}

	sql := fmt.Sprintf("UPDATE %s SET %s",
		dialect.Quote(u.tableName),
		strings.Join(sets, ", "))

	// WHERE clause
	if len(u.conditions) > 0 {
		whereSQL, whereArgs := buildWhere(dialect, u.conditions, argIndex)
		sql += " " + whereSQL
		args = append(args, whereArgs...)
	}

	// RETURNING clause
	if len(u.returning) > 0 && dialect.SupportsReturning() {
		retCols := make([]string, len(u.returning))
		for i, c := range u.returning {
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

// Exec executes the update and returns the number of affected rows.
func (u *UpdateBuilder) Exec(ctx context.Context) (int64, error) {
	query, args := u.Build()

	// Start profiling if enabled
	var profile *QueryProfile
	if u.profiler != nil && u.profiler.IsEnabled() {
		profile = u.profiler.StartQuery(query, args)
	}

	result, err := u.conn.Exec(ctx, query, args...)

	// Record profiling data
	if profile != nil {
		if err == nil {
			affected, _ := result.RowsAffected()
			profile.RowsAffected = affected
		}
		u.profiler.EndQuery(profile, err)
	}

	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// All executes the update and returns all affected rows (requires RETURNING).
func (u *UpdateBuilder) All(ctx context.Context) (Results, error) {
	if !u.conn.Dialect.SupportsReturning() {
		return nil, fmt.Errorf("dialect %s does not support RETURNING clause", u.conn.Dialect.Name())
	}

	if len(u.returning) == 0 {
		u.returning = []string{"*"}
	}

	query, args := u.Build()
	rows, err := u.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRows(rows)
}

// One executes the update and returns the first affected row (requires RETURNING).
func (u *UpdateBuilder) One(ctx context.Context) (Result, error) {
	results, err := u.All(ctx)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}
