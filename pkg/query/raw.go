package query

import (
	"context"
	"database/sql"
	"strings"

	"github.com/nexus-db/nexus/pkg/dialects"
)

// RawQuery represents a raw SQL query.
type RawQuery struct {
	conn *dialects.Connection
	sql  string
	args []interface{}
}

// Raw creates a new raw SQL query.
// Use ? as placeholder for all dialects; it will be converted automatically.
//
// Example:
//
//	conn.Raw("SELECT * FROM users WHERE id = ?", 1).Query(ctx)
//	conn.Raw("UPDATE users SET name = ? WHERE id = ?", "Alice", 1).Exec(ctx)
func NewRawQuery(conn *dialects.Connection, sql string, args ...interface{}) *RawQuery {
	return &RawQuery{
		conn: conn,
		sql:  sql,
		args: args,
	}
}

// convertPlaceholders converts ? placeholders to dialect-specific format.
func (r *RawQuery) convertPlaceholders() string {
	dialect := r.conn.Dialect

	// Count placeholders
	count := strings.Count(r.sql, "?")
	if count == 0 {
		return r.sql
	}

	// If dialect uses ? (MySQL, SQLite), return as-is
	if dialect.Placeholder(1) == "?" {
		return r.sql
	}

	// Replace ? with $1, $2, etc. for PostgreSQL
	result := r.sql
	for i := 1; i <= count; i++ {
		result = strings.Replace(result, "?", dialect.Placeholder(i), 1)
	}
	return result
}

// Query executes the raw SQL and returns rows.
func (r *RawQuery) Query(ctx context.Context) (*sql.Rows, error) {
	sql := r.convertPlaceholders()
	return r.conn.Query(ctx, sql, r.args...)
}

// QueryRow executes the raw SQL and returns a single row.
func (r *RawQuery) QueryRow(ctx context.Context) *sql.Row {
	sql := r.convertPlaceholders()
	return r.conn.QueryRow(ctx, sql, r.args...)
}

// Exec executes the raw SQL without returning rows.
func (r *RawQuery) Exec(ctx context.Context) (sql.Result, error) {
	sql := r.convertPlaceholders()
	return r.conn.Exec(ctx, sql, r.args...)
}

// All executes the query and returns all results as Results.
func (r *RawQuery) All(ctx context.Context) (Results, error) {
	rows, err := r.Query(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

// One executes the query and returns the first result.
func (r *RawQuery) One(ctx context.Context) (Result, error) {
	results, err := r.All(ctx)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	return results[0], nil
}

// Scan executes the query and scans the result into dest.
// dest should be pointers to variables for each column.
func (r *RawQuery) Scan(ctx context.Context, dest ...interface{}) error {
	return r.QueryRow(ctx).Scan(dest...)
}

// ScanStruct executes the query and scans the result into a struct.
// The struct fields should have `db` tags matching column names.
func (r *RawQuery) ScanAll(ctx context.Context, dest interface{}) error {
	rows, err := r.Query(ctx)
	if err != nil {
		return err
	}
	defer rows.Close()

	// This is a simplified implementation
	// For production, use a library like sqlx for struct scanning
	return scanIntoSlice(rows, dest)
}

// scanIntoSlice is a placeholder for struct scanning.
// In production, this should handle reflection-based scanning.
func scanIntoSlice(rows *sql.Rows, dest interface{}) error {
	// For now, just exhaust the rows
	// TODO: Implement proper struct scanning
	for rows.Next() {
		// Skip
	}
	return rows.Err()
}

// RawBuilder adds raw SQL support to the Builder.
type RawBuilder struct {
	*Builder
}

// RawQuery creates a raw SQL query from the builder's connection.
func (b *Builder) RawQuery(sql string, args ...interface{}) *RawQuery {
	return NewRawQuery(b.conn, sql, args...)
}

// RawExec is a convenience function for executing raw SQL without results.
func RawExec(ctx context.Context, conn *dialects.Connection, sql string, args ...interface{}) (sql.Result, error) {
	return NewRawQuery(conn, sql, args...).Exec(ctx)
}

// RawQueryAll is a convenience function for querying with raw SQL.
func RawQueryAll(ctx context.Context, conn *dialects.Connection, sql string, args ...interface{}) (Results, error) {
	return NewRawQuery(conn, sql, args...).All(ctx)
}
