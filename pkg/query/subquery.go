package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-db/nexus/pkg/dialects"
)

// Subquery wraps a SelectBuilder for use in WHERE clauses.
type Subquery struct {
	builder *SelectBuilder
}

// Sub creates a subquery from a SelectBuilder.
func Sub(builder *SelectBuilder) *Subquery {
	return &Subquery{builder: builder}
}

// WhereIn adds a WHERE column IN (subquery) condition.
func (s *SelectBuilder) WhereIn(column string, subquery *SelectBuilder) *SelectBuilder {
	s.conditions = append(s.conditions, Condition{
		Column:   column,
		Operator: "IN_SUBQUERY",
		Value:    subquery,
	})
	return s
}

// WhereNotIn adds a WHERE column NOT IN (subquery) condition.
func (s *SelectBuilder) WhereNotIn(column string, subquery *SelectBuilder) *SelectBuilder {
	s.conditions = append(s.conditions, Condition{
		Column:   column,
		Operator: "NOT_IN_SUBQUERY",
		Value:    subquery,
	})
	return s
}

// WhereExists adds a WHERE EXISTS (subquery) condition.
func (s *SelectBuilder) WhereExists(subquery *SelectBuilder) *SelectBuilder {
	s.conditions = append(s.conditions, Condition{
		Operator: "EXISTS",
		Value:    subquery,
	})
	return s
}

// WhereNotExists adds a WHERE NOT EXISTS (subquery) condition.
func (s *SelectBuilder) WhereNotExists(subquery *SelectBuilder) *SelectBuilder {
	s.conditions = append(s.conditions, Condition{
		Operator: "NOT_EXISTS",
		Value:    subquery,
	})
	return s
}

// buildWhereWithSubqueries builds WHERE clause handling subqueries.
func buildWhereWithSubqueries(dialect dialects.Dialect, conditions []Condition, startIndex int) (string, []interface{}) {
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

		switch cond.Operator {
		case "IN_SUBQUERY":
			subquery := cond.Value.(*SelectBuilder)
			subSQL, subArgs := subquery.Build()
			parts = append(parts, fmt.Sprintf("%s IN (%s)", dialect.Quote(cond.Column), subSQL))
			args = append(args, subArgs...)
			argIndex += len(subArgs)

		case "NOT_IN_SUBQUERY":
			subquery := cond.Value.(*SelectBuilder)
			subSQL, subArgs := subquery.Build()
			parts = append(parts, fmt.Sprintf("%s NOT IN (%s)", dialect.Quote(cond.Column), subSQL))
			args = append(args, subArgs...)
			argIndex += len(subArgs)

		case "EXISTS":
			subquery := cond.Value.(*SelectBuilder)
			subSQL, subArgs := subquery.Build()
			parts = append(parts, fmt.Sprintf("EXISTS (%s)", subSQL))
			args = append(args, subArgs...)
			argIndex += len(subArgs)

		case "NOT_EXISTS":
			subquery := cond.Value.(*SelectBuilder)
			subSQL, subArgs := subquery.Build()
			parts = append(parts, fmt.Sprintf("NOT EXISTS (%s)", subSQL))
			args = append(args, subArgs...)
			argIndex += len(subArgs)

		case "IS NULL", "IS NOT NULL":
			quotedCol := dialect.Quote(cond.Column)
			parts = append(parts, fmt.Sprintf("%s %s", quotedCol, cond.Operator))

		case "IN":
			values := cond.Value.([]interface{})
			placeholders := make([]string, len(values))
			for i, v := range values {
				placeholders[i] = dialect.Placeholder(argIndex)
				args = append(args, v)
				argIndex++
			}
			parts = append(parts, fmt.Sprintf("%s IN (%s)", dialect.Quote(cond.Column), strings.Join(placeholders, ", ")))

		default:
			quotedCol := dialect.Quote(cond.Column)
			parts = append(parts, fmt.Sprintf("%s %s %s", quotedCol, cond.Operator, dialect.Placeholder(argIndex)))
			args = append(args, cond.Value)
			argIndex++
		}
	}

	return "WHERE " + strings.Join(parts, " AND "), args
}

// ScalarSubquery represents a subquery that returns a single value.
type ScalarSubquery struct {
	builder *SelectBuilder
	alias   string
}

// AsScalar creates a scalar subquery that can be used in SELECT.
func (s *SelectBuilder) AsScalar(alias string) *ScalarSubquery {
	return &ScalarSubquery{
		builder: s,
		alias:   alias,
	}
}

// FromSubquery creates a query from a subquery (derived table).
func FromSubquery(conn *dialects.Connection, subquery *SelectBuilder, alias string) *DerivedTableBuilder {
	return &DerivedTableBuilder{
		conn:     conn,
		subquery: subquery,
		alias:    alias,
	}
}

// DerivedTableBuilder builds queries from subqueries.
type DerivedTableBuilder struct {
	conn       *dialects.Connection
	subquery   *SelectBuilder
	alias      string
	columns    []string
	conditions []Condition
	orders     []OrderBy
	limit      int
	offset     int
}

// Select specifies columns to select.
func (d *DerivedTableBuilder) Select(columns ...string) *DerivedTableBuilder {
	d.columns = columns
	return d
}

// Where adds a WHERE condition.
func (d *DerivedTableBuilder) Where(conditions ...Condition) *DerivedTableBuilder {
	d.conditions = append(d.conditions, conditions...)
	return d
}

// OrderBy adds an ORDER BY clause.
func (d *DerivedTableBuilder) OrderBy(column string, direction OrderDirection) *DerivedTableBuilder {
	d.orders = append(d.orders, OrderBy{Column: column, Direction: direction})
	return d
}

// Limit sets the LIMIT clause.
func (d *DerivedTableBuilder) Limit(n int) *DerivedTableBuilder {
	d.limit = n
	return d
}

// Build generates the SQL query and arguments.
func (d *DerivedTableBuilder) Build() (string, []interface{}) {
	dialect := d.conn.Dialect

	subSQL, subArgs := d.subquery.Build()

	// SELECT columns
	cols := "*"
	if len(d.columns) > 0 {
		quotedCols := make([]string, len(d.columns))
		for i, c := range d.columns {
			if c == "*" {
				quotedCols[i] = c
			} else {
				quotedCols[i] = dialect.Quote(c)
			}
		}
		cols = strings.Join(quotedCols, ", ")
	}

	sql := fmt.Sprintf("SELECT %s FROM (%s) AS %s", cols, subSQL, dialect.Quote(d.alias))
	args := subArgs
	argIndex := len(subArgs) + 1

	// WHERE
	if len(d.conditions) > 0 {
		whereSQL, whereArgs := buildWhere(dialect, d.conditions, argIndex)
		sql += " " + whereSQL
		args = append(args, whereArgs...)
	}

	// ORDER BY
	if len(d.orders) > 0 {
		orderParts := make([]string, len(d.orders))
		for i, o := range d.orders {
			orderParts[i] = fmt.Sprintf("%s %s", dialect.Quote(o.Column), o.Direction.String())
		}
		sql += " ORDER BY " + strings.Join(orderParts, ", ")
	}

	// LIMIT
	if d.limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", d.limit)
	}

	// OFFSET
	if d.offset > 0 {
		sql += fmt.Sprintf(" OFFSET %d", d.offset)
	}

	return sql, args
}

// All executes the query and returns all results.
func (d *DerivedTableBuilder) All(ctx context.Context) (Results, error) {
	sql, args := d.Build()
	rows, err := d.conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

// One executes the query and returns the first result.
func (d *DerivedTableBuilder) One(ctx context.Context) (Result, error) {
	d.limit = 1
	results, err := d.All(ctx)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}
