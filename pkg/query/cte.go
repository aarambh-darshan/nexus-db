package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-db/nexus/pkg/dialects"
)

// CTE represents a Common Table Expression.
type CTE struct {
	Name      string
	Query     *SelectBuilder
	Recursive bool
	Columns   []string // Optional column aliases
}

// CTEBuilder builds queries with CTEs.
type CTEBuilder struct {
	conn      *dialects.Connection
	ctes      []*CTE
	recursive bool
}

// With creates a new CTE builder with a named CTE.
func With(conn *dialects.Connection, name string, query *SelectBuilder) *CTEBuilder {
	return &CTEBuilder{
		conn: conn,
		ctes: []*CTE{{Name: name, Query: query}},
	}
}

// WithColumns creates a CTE with explicit column aliases.
func WithColumns(conn *dialects.Connection, name string, columns []string, query *SelectBuilder) *CTEBuilder {
	return &CTEBuilder{
		conn: conn,
		ctes: []*CTE{{Name: name, Query: query, Columns: columns}},
	}
}

// WithRecursive creates a recursive CTE.
// The query should be a UNION of base case and recursive case.
func WithRecursive(conn *dialects.Connection, name string, columns []string, baseQuery, recursiveQuery *SelectBuilder) *CTEBuilder {
	// Create a combined query using UNION ALL
	_ = baseQuery.UnionAll(recursiveQuery) // Used in Build phase

	return &CTEBuilder{
		conn:      conn,
		recursive: true,
		ctes: []*CTE{{
			Name:      name,
			Columns:   columns,
			Recursive: true,
			Query:     baseQuery, // Store the base query for now
		}},
	}
}

// And adds another CTE to the builder.
func (c *CTEBuilder) And(name string, query *SelectBuilder) *CTEBuilder {
	c.ctes = append(c.ctes, &CTE{Name: name, Query: query})
	return c
}

// AndColumns adds another CTE with explicit column aliases.
func (c *CTEBuilder) AndColumns(name string, columns []string, query *SelectBuilder) *CTEBuilder {
	c.ctes = append(c.ctes, &CTE{Name: name, Query: query, Columns: columns})
	return c
}

// Select creates a SELECT query that uses the CTEs.
func (c *CTEBuilder) Select(columns ...string) *CTESelectBuilder {
	return &CTESelectBuilder{
		cteBuilder: c,
		columns:    columns,
	}
}

// CTESelectBuilder builds the main SELECT after WITH clause.
type CTESelectBuilder struct {
	cteBuilder *CTEBuilder
	tableName  string
	columns    []string
	conditions []Condition
	orders     []OrderBy
	limit      int
	offset     int
	joins      []joinClause
}

// From specifies the table to select from.
func (s *CTESelectBuilder) From(tableName string) *CTESelectBuilder {
	s.tableName = tableName
	return s
}

// Where adds a WHERE condition.
func (s *CTESelectBuilder) Where(conditions ...Condition) *CTESelectBuilder {
	s.conditions = append(s.conditions, conditions...)
	return s
}

// OrderBy adds an ORDER BY clause.
func (s *CTESelectBuilder) OrderBy(column string, direction OrderDirection) *CTESelectBuilder {
	s.orders = append(s.orders, OrderBy{Column: column, Direction: direction})
	return s
}

// Limit sets the LIMIT clause.
func (s *CTESelectBuilder) Limit(n int) *CTESelectBuilder {
	s.limit = n
	return s
}

// Offset sets the OFFSET clause.
func (s *CTESelectBuilder) Offset(n int) *CTESelectBuilder {
	s.offset = n
	return s
}

// Join adds a JOIN to a CTE.
func (s *CTESelectBuilder) Join(table, condition string) *CTESelectBuilder {
	s.joins = append(s.joins, joinClause{joinType: "INNER", table: table, condition: condition})
	return s
}

// Build generates the SQL query and arguments.
func (s *CTESelectBuilder) Build() (string, []interface{}) {
	dialect := s.cteBuilder.conn.Dialect
	var allArgs []interface{}
	argOffset := 0

	// Build WITH clause
	withKeyword := "WITH"
	if s.cteBuilder.recursive {
		withKeyword = "WITH RECURSIVE"
	}

	cteParts := make([]string, len(s.cteBuilder.ctes))
	for i, cte := range s.cteBuilder.ctes {
		cteSQL, cteArgs := cte.Query.Build()
		allArgs = append(allArgs, cteArgs...)
		argOffset += len(cteArgs)

		cteName := dialect.Quote(cte.Name)
		if len(cte.Columns) > 0 {
			quotedCols := make([]string, len(cte.Columns))
			for j, col := range cte.Columns {
				quotedCols[j] = dialect.Quote(col)
			}
			cteName += " (" + strings.Join(quotedCols, ", ") + ")"
		}
		cteParts[i] = fmt.Sprintf("%s AS (%s)", cteName, cteSQL)
	}

	withClause := withKeyword + " " + strings.Join(cteParts, ", ")

	// Build main SELECT
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

	sql := fmt.Sprintf("%s SELECT %s FROM %s", withClause, cols, dialect.Quote(s.tableName))

	// JOINs
	for _, join := range s.joins {
		sql += fmt.Sprintf(" %s JOIN %s ON %s", join.joinType, dialect.Quote(join.table), join.condition)
	}

	// WHERE
	if len(s.conditions) > 0 {
		whereSQL, whereArgs := buildWhere(dialect, s.conditions, argOffset+1)
		sql += " " + whereSQL
		allArgs = append(allArgs, whereArgs...)
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

	return sql, allArgs
}

// All executes the query and returns all results.
func (s *CTESelectBuilder) All(ctx context.Context) (Results, error) {
	sql, args := s.Build()
	rows, err := s.cteBuilder.conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

// One executes the query and returns the first result.
func (s *CTESelectBuilder) One(ctx context.Context) (Result, error) {
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
