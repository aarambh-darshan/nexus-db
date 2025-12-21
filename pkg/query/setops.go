package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-db/nexus/pkg/dialects"
)

// SetOperation represents a set operation type.
type SetOperation int

const (
	SetUnion SetOperation = iota
	SetUnionAll
	SetIntersect
	SetExcept
)

// String returns the SQL keyword for the set operation.
func (s SetOperation) String() string {
	switch s {
	case SetUnion:
		return "UNION"
	case SetUnionAll:
		return "UNION ALL"
	case SetIntersect:
		return "INTERSECT"
	case SetExcept:
		return "EXCEPT"
	default:
		return "UNION"
	}
}

// SetOpQuery represents a query with set operations.
type SetOpQuery struct {
	conn       *dialects.Connection
	queries    []*SelectBuilder
	operations []SetOperation
	orders     []OrderBy
	limit      int
	offset     int
}

// Union creates a UNION of two queries.
func (s *SelectBuilder) Union(other *SelectBuilder) *SetOpQuery {
	return &SetOpQuery{
		conn:       s.conn,
		queries:    []*SelectBuilder{s, other},
		operations: []SetOperation{SetUnion},
	}
}

// UnionAll creates a UNION ALL of two queries.
func (s *SelectBuilder) UnionAll(other *SelectBuilder) *SetOpQuery {
	return &SetOpQuery{
		conn:       s.conn,
		queries:    []*SelectBuilder{s, other},
		operations: []SetOperation{SetUnionAll},
	}
}

// Intersect creates an INTERSECT of two queries.
func (s *SelectBuilder) Intersect(other *SelectBuilder) *SetOpQuery {
	return &SetOpQuery{
		conn:       s.conn,
		queries:    []*SelectBuilder{s, other},
		operations: []SetOperation{SetIntersect},
	}
}

// Except creates an EXCEPT of two queries.
func (s *SelectBuilder) Except(other *SelectBuilder) *SetOpQuery {
	return &SetOpQuery{
		conn:       s.conn,
		queries:    []*SelectBuilder{s, other},
		operations: []SetOperation{SetExcept},
	}
}

// Union adds another UNION to the set operation.
func (q *SetOpQuery) Union(other *SelectBuilder) *SetOpQuery {
	q.queries = append(q.queries, other)
	q.operations = append(q.operations, SetUnion)
	return q
}

// UnionAll adds another UNION ALL to the set operation.
func (q *SetOpQuery) UnionAll(other *SelectBuilder) *SetOpQuery {
	q.queries = append(q.queries, other)
	q.operations = append(q.operations, SetUnionAll)
	return q
}

// OrderBy adds an ORDER BY clause to the final result.
func (q *SetOpQuery) OrderBy(column string, direction OrderDirection) *SetOpQuery {
	q.orders = append(q.orders, OrderBy{Column: column, Direction: direction})
	return q
}

// Limit sets the LIMIT clause for the final result.
func (q *SetOpQuery) Limit(n int) *SetOpQuery {
	q.limit = n
	return q
}

// Offset sets the OFFSET clause for the final result.
func (q *SetOpQuery) Offset(n int) *SetOpQuery {
	q.offset = n
	return q
}

// Build generates the SQL query and arguments.
func (q *SetOpQuery) Build() (string, []interface{}) {
	dialect := q.conn.Dialect
	var allArgs []interface{}
	var parts []string

	for i, selectQuery := range q.queries {
		sql, args := selectQuery.Build()
		// Don't wrap in parentheses for SQLite compatibility
		parts = append(parts, sql)
		allArgs = append(allArgs, args...)

		if i < len(q.operations) {
			parts = append(parts, q.operations[i].String())
		}
	}

	sql := strings.Join(parts, " ")

	// ORDER BY
	if len(q.orders) > 0 {
		orderParts := make([]string, len(q.orders))
		for i, o := range q.orders {
			orderParts[i] = fmt.Sprintf("%s %s", dialect.Quote(o.Column), o.Direction.String())
		}
		sql += " ORDER BY " + strings.Join(orderParts, ", ")
	}

	// LIMIT
	if q.limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", q.limit)
	}

	// OFFSET
	if q.offset > 0 {
		sql += fmt.Sprintf(" OFFSET %d", q.offset)
	}

	return sql, allArgs
}

// All executes the query and returns all results.
func (q *SetOpQuery) All(ctx context.Context) (Results, error) {
	sql, args := q.Build()
	rows, err := q.conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

// One executes the query and returns the first result.
func (q *SetOpQuery) One(ctx context.Context) (Result, error) {
	q.limit = 1
	results, err := q.All(ctx)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

// Count returns the count of results (wraps in subquery).
func (q *SetOpQuery) Count(ctx context.Context) (int64, error) {
	sql, args := q.Build()
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM (%s) AS _count", sql)

	var count int64
	row := q.conn.QueryRow(ctx, countSQL, args...)
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
