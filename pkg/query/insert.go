package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-db/nexus/pkg/dialects"
)

// InsertBuilder builds INSERT queries.
type InsertBuilder struct {
	conn       *dialects.Connection
	tableName  string
	data       map[string]interface{}
	returning  []string
	onConflict *conflictClause
	batchData  []map[string]interface{}
	profiler   *Profiler
}

type conflictClause struct {
	columns   []string
	doNothing bool
	doUpdate  map[string]interface{}
}

// Returning specifies columns to return after insert.
func (i *InsertBuilder) Returning(columns ...string) *InsertBuilder {
	i.returning = columns
	return i
}

// OnConflictDoNothing adds ON CONFLICT DO NOTHING clause.
func (i *InsertBuilder) OnConflictDoNothing(columns ...string) *InsertBuilder {
	i.onConflict = &conflictClause{
		columns:   columns,
		doNothing: true,
	}
	return i
}

// OnConflictDoUpdate adds ON CONFLICT DO UPDATE clause.
func (i *InsertBuilder) OnConflictDoUpdate(conflictColumns []string, updateData map[string]interface{}) *InsertBuilder {
	i.onConflict = &conflictClause{
		columns:  conflictColumns,
		doUpdate: updateData,
	}
	return i
}

// Values adds additional rows for batch insert.
func (i *InsertBuilder) Values(data map[string]interface{}) *InsertBuilder {
	if i.batchData == nil {
		i.batchData = []map[string]interface{}{i.data}
	}
	i.batchData = append(i.batchData, data)
	return i
}

// Build generates the SQL query and arguments.
func (i *InsertBuilder) Build() (string, []interface{}) {
	dialect := i.conn.Dialect
	var args []interface{}
	argIndex := 1

	// Get columns from first data row
	var columns []string
	for col := range i.data {
		columns = append(columns, col)
	}

	// Quote columns
	quotedCols := make([]string, len(columns))
	for idx, c := range columns {
		quotedCols[idx] = dialect.Quote(c)
	}

	// Build values
	var valueSets []string
	allData := i.batchData
	if allData == nil {
		allData = []map[string]interface{}{i.data}
	}

	for _, row := range allData {
		placeholders := make([]string, len(columns))
		for idx, col := range columns {
			placeholders[idx] = dialect.Placeholder(argIndex)
			args = append(args, row[col])
			argIndex++
		}
		valueSets = append(valueSets, "("+strings.Join(placeholders, ", ")+")")
	}

	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s",
		dialect.Quote(i.tableName),
		strings.Join(quotedCols, ", "),
		strings.Join(valueSets, ", "))

	// ON CONFLICT clause
	if i.onConflict != nil {
		conflictCols := make([]string, len(i.onConflict.columns))
		for idx, c := range i.onConflict.columns {
			conflictCols[idx] = dialect.Quote(c)
		}

		if i.onConflict.doNothing {
			sql += fmt.Sprintf(" ON CONFLICT (%s) DO NOTHING", strings.Join(conflictCols, ", "))
		} else if i.onConflict.doUpdate != nil {
			updates := make([]string, 0, len(i.onConflict.doUpdate))
			for col, val := range i.onConflict.doUpdate {
				updates = append(updates, fmt.Sprintf("%s = %s", dialect.Quote(col), dialect.Placeholder(argIndex)))
				args = append(args, val)
				argIndex++
			}
			sql += fmt.Sprintf(" ON CONFLICT (%s) DO UPDATE SET %s",
				strings.Join(conflictCols, ", "),
				strings.Join(updates, ", "))
		}
	}

	// RETURNING clause
	if len(i.returning) > 0 && dialect.SupportsReturning() {
		retCols := make([]string, len(i.returning))
		for idx, c := range i.returning {
			if c == "*" {
				retCols[idx] = "*"
			} else {
				retCols[idx] = dialect.Quote(c)
			}
		}
		sql += " RETURNING " + strings.Join(retCols, ", ")
	}

	return sql, args
}

// Exec executes the insert and returns the number of affected rows.
func (i *InsertBuilder) Exec(ctx context.Context) (int64, error) {
	query, args := i.Build()

	// Start profiling if enabled
	var profile *QueryProfile
	if i.profiler != nil && i.profiler.IsEnabled() {
		profile = i.profiler.StartQuery(query, args)
	}

	result, err := i.conn.Exec(ctx, query, args...)

	// Record profiling data
	if profile != nil {
		if err == nil {
			affected, _ := result.RowsAffected()
			profile.RowsAffected = affected
		}
		i.profiler.EndQuery(profile, err)
	}

	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// One executes the insert and returns the inserted row (requires RETURNING).
func (i *InsertBuilder) One(ctx context.Context) (Result, error) {
	if !i.conn.Dialect.SupportsReturning() {
		return nil, fmt.Errorf("dialect %s does not support RETURNING clause", i.conn.Dialect.Name())
	}

	if len(i.returning) == 0 {
		i.returning = []string{"*"}
	}

	query, args := i.Build()
	rows, err := i.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results, err := scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

// LastInsertId executes the insert and returns the last insert ID.
// For PostgreSQL, use One() with RETURNING instead.
func (i *InsertBuilder) LastInsertId(ctx context.Context) (int64, error) {
	query, args := i.Build()
	result, err := i.conn.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}
