package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-db/nexus/pkg/core/schema"
	"github.com/nexus-db/nexus/pkg/dialects"
)

// SelectBuilder builds SELECT queries.
type SelectBuilder struct {
	conn       *dialects.Connection
	tableName  string
	columns    []string
	conditions []Condition
	orders     []OrderBy
	limit      int
	offset     int
	joins      []joinClause
	groupBy    []string
	having     []Condition
	schema     *schema.Schema // Optional schema for relation-aware queries
	includes   []string       // Relations to eager load
	profiler   *Profiler      // Optional profiler for performance tracking
}

type joinClause struct {
	joinType  string // INNER, LEFT, RIGHT
	table     string
	condition string
}

// Where adds a WHERE condition.
func (s *SelectBuilder) Where(conditions ...Condition) *SelectBuilder {
	s.conditions = append(s.conditions, conditions...)
	return s
}

// OrderBy adds an ORDER BY clause.
func (s *SelectBuilder) OrderBy(column string, direction OrderDirection) *SelectBuilder {
	s.orders = append(s.orders, OrderBy{Column: column, Direction: direction})
	return s
}

// Limit sets the LIMIT clause.
func (s *SelectBuilder) Limit(n int) *SelectBuilder {
	s.limit = n
	return s
}

// Offset sets the OFFSET clause.
func (s *SelectBuilder) Offset(n int) *SelectBuilder {
	s.offset = n
	return s
}

// Join adds an INNER JOIN clause.
func (s *SelectBuilder) Join(table, condition string) *SelectBuilder {
	s.joins = append(s.joins, joinClause{joinType: "INNER", table: table, condition: condition})
	return s
}

// LeftJoin adds a LEFT JOIN clause.
func (s *SelectBuilder) LeftJoin(table, condition string) *SelectBuilder {
	s.joins = append(s.joins, joinClause{joinType: "LEFT", table: table, condition: condition})
	return s
}

// RightJoin adds a RIGHT JOIN clause.
func (s *SelectBuilder) RightJoin(table, condition string) *SelectBuilder {
	s.joins = append(s.joins, joinClause{joinType: "RIGHT", table: table, condition: condition})
	return s
}

// GroupBy adds a GROUP BY clause.
func (s *SelectBuilder) GroupBy(columns ...string) *SelectBuilder {
	s.groupBy = append(s.groupBy, columns...)
	return s
}

// Having adds a HAVING condition.
func (s *SelectBuilder) Having(conditions ...Condition) *SelectBuilder {
	s.having = append(s.having, conditions...)
	return s
}

// WithSchema attaches a schema for relation-aware operations like eager loading.
func (s *SelectBuilder) WithSchema(sch *schema.Schema) *SelectBuilder {
	s.schema = sch
	return s
}

// Include specifies relations to eager load.
// Example: users.Select().WithSchema(s).Include("Posts", "Profile").All(ctx)
func (s *SelectBuilder) Include(relations ...string) *SelectBuilder {
	s.includes = append(s.includes, relations...)
	return s
}

// Build generates the SQL query and arguments.
func (s *SelectBuilder) Build() (string, []interface{}) {
	dialect := s.conn.Dialect
	var args []interface{}
	argIndex := 1

	// SELECT columns
	cols := "*"
	if len(s.columns) > 0 {
		quotedCols := make([]string, len(s.columns))
		for i, c := range s.columns {
			if c == "*" || strings.Contains(c, "(") || strings.Contains(c, ".") {
				quotedCols[i] = c
			} else {
				quotedCols[i] = dialect.Quote(c)
			}
		}
		cols = strings.Join(quotedCols, ", ")
	}

	sql := fmt.Sprintf("SELECT %s FROM %s", cols, dialect.Quote(s.tableName))

	// JOINs
	for _, join := range s.joins {
		sql += fmt.Sprintf(" %s JOIN %s ON %s", join.joinType, dialect.Quote(join.table), join.condition)
	}

	// WHERE
	if len(s.conditions) > 0 {
		whereSQL, whereArgs := buildWhere(dialect, s.conditions, argIndex)
		sql += " " + whereSQL
		args = append(args, whereArgs...)
		argIndex += len(whereArgs)
	}

	// GROUP BY
	if len(s.groupBy) > 0 {
		quotedCols := make([]string, len(s.groupBy))
		for i, c := range s.groupBy {
			quotedCols[i] = dialect.Quote(c)
		}
		sql += " GROUP BY " + strings.Join(quotedCols, ", ")
	}

	// HAVING
	if len(s.having) > 0 {
		havingSQL, havingArgs := buildWhere(dialect, s.having, argIndex)
		sql += " " + strings.Replace(havingSQL, "WHERE", "HAVING", 1)
		args = append(args, havingArgs...)
		argIndex += len(havingArgs)
	}

	// ORDER BY
	if len(s.orders) > 0 {
		orderParts := make([]string, len(s.orders))
		for i, o := range s.orders {
			orderParts[i] = fmt.Sprintf("%s %s", dialect.Quote(o.Column), o.Direction.String())
		}
		sql += " ORDER BY " + strings.Join(orderParts, ", ")
	}

	// LIMIT
	if s.limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", s.limit)
	}

	// OFFSET
	if s.offset > 0 {
		sql += fmt.Sprintf(" OFFSET %d", s.offset)
	}

	return sql, args
}

// All executes the query and returns all matching rows.
func (s *SelectBuilder) All(ctx context.Context) (Results, error) {
	query, args := s.Build()

	// Start profiling if enabled
	var profile *QueryProfile
	if s.profiler != nil && s.profiler.IsEnabled() {
		profile = s.profiler.StartQuery(query, args)
	}

	rows, err := s.conn.Query(ctx, query, args...)
	if err != nil {
		if profile != nil {
			s.profiler.EndQuery(profile, err)
		}
		return nil, err
	}
	defer rows.Close()

	results, err := scanRows(rows)
	if err != nil {
		if profile != nil {
			s.profiler.EndQuery(profile, err)
		}
		return nil, err
	}

	// Record profiling data
	if profile != nil {
		profile.RowsReturned = len(results)
		s.profiler.EndQuery(profile, nil)
	}

	// Eager load related data if includes are specified
	if err := s.preloadRelations(ctx, results); err != nil {
		return nil, err
	}

	return results, nil
}

// One executes the query and returns the first matching row.
func (s *SelectBuilder) One(ctx context.Context) (Result, error) {
	s.limit = 1
	results, err := s.All(ctx)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

// AllLazy executes the query and returns LazyResults with deferred relation loading.
// Unlike Include() which eagerly loads relations, lazy loading defers queries
// until GetRelation() is called on each result.
func (s *SelectBuilder) AllLazy(ctx context.Context) (LazyResults, error) {
	query, args := s.Build()
	rows, err := s.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results, err := scanRows(rows)
	if err != nil {
		return nil, err
	}

	// Wrap each result in LazyResult
	lazyResults := make(LazyResults, len(results))
	for i, r := range results {
		lazyResults[i] = NewLazyResult(r, s.conn, s.schema, s.tableName)
	}

	return lazyResults, nil
}

// OneLazy executes the query and returns a single LazyResult.
func (s *SelectBuilder) OneLazy(ctx context.Context) (*LazyResult, error) {
	s.limit = 1
	results, err := s.AllLazy(ctx)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

// Count returns the count of matching rows.
func (s *SelectBuilder) Count(ctx context.Context) (int64, error) {
	// Build count query
	dialect := s.conn.Dialect
	var args []interface{}
	argIndex := 1

	sql := fmt.Sprintf("SELECT COUNT(*) FROM %s", dialect.Quote(s.tableName))

	// JOINs
	for _, join := range s.joins {
		sql += fmt.Sprintf(" %s JOIN %s ON %s", join.joinType, dialect.Quote(join.table), join.condition)
	}

	// WHERE
	if len(s.conditions) > 0 {
		whereSQL, whereArgs := buildWhere(dialect, s.conditions, argIndex)
		sql += " " + whereSQL
		args = append(args, whereArgs...)
	}

	// Start profiling if enabled
	var profile *QueryProfile
	if s.profiler != nil && s.profiler.IsEnabled() {
		profile = s.profiler.StartQuery(sql, args)
	}

	var count int64
	row := s.conn.QueryRow(ctx, sql, args...)
	err := row.Scan(&count)

	// Record profiling data
	if profile != nil {
		profile.RowsReturned = 1
		s.profiler.EndQuery(profile, err)
	}

	if err != nil {
		return 0, err
	}
	return count, nil
}

// Exists returns true if any matching rows exist.
func (s *SelectBuilder) Exists(ctx context.Context) (bool, error) {
	count, err := s.Count(ctx)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
