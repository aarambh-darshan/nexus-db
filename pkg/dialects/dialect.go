// Package dialects provides database dialect interfaces and implementations.
package dialects

import (
	"context"
	"database/sql"

	"github.com/nexus-db/nexus/pkg/core/schema"
)

// Dialect defines the interface that all database dialects must implement.
type Dialect interface {
	// Name returns the dialect name (e.g., "postgres", "sqlite", "mysql").
	Name() string

	// DriverName returns the Go sql driver name.
	DriverName() string

	// Quote quotes an identifier (table/column name).
	Quote(identifier string) string

	// Placeholder returns the parameter placeholder for the given index (1-based).
	// PostgreSQL uses $1, $2; MySQL/SQLite use ?.
	Placeholder(index int) string

	// TypeMapping maps schema field types to SQL types.
	TypeMapping(field *schema.Field) string

	// CreateTableSQL generates CREATE TABLE statement.
	CreateTableSQL(model *schema.Model) string

	// DropTableSQL generates DROP TABLE statement.
	DropTableSQL(tableName string) string

	// CreateIndexSQL generates CREATE INDEX statement.
	CreateIndexSQL(tableName string, index *schema.Index) string

	// DropIndexSQL generates DROP INDEX statement.
	DropIndexSQL(tableName, indexName string) string

	// AddColumnSQL generates ALTER TABLE ADD COLUMN statement.
	AddColumnSQL(tableName string, field *schema.Field) string

	// DropColumnSQL generates ALTER TABLE DROP COLUMN statement.
	DropColumnSQL(tableName, columnName string) string

	// RenameColumnSQL generates ALTER TABLE RENAME COLUMN statement.
	RenameColumnSQL(tableName, oldName, newName string) string

	// SupportsReturning returns true if RETURNING clause is supported.
	SupportsReturning() bool

	// SupportsUpsert returns true if upsert (ON CONFLICT) is supported.
	SupportsUpsert() bool

	// ExplainSQL wraps a query with EXPLAIN syntax.
	// format: output format (json, text, etc.)
	// analyze: if true, actually execute the query
	ExplainSQL(query string, format string, analyze bool) string

	// SupportsExplainFormat returns true if the given format is supported.
	SupportsExplainFormat(format string) bool
}

// Connection represents a database connection with dialect awareness.
type Connection struct {
	DB      *sql.DB
	Dialect Dialect
}

// NewConnection creates a new connection with the specified dialect.
func NewConnection(db *sql.DB, dialect Dialect) *Connection {
	return &Connection{
		DB:      db,
		Dialect: dialect,
	}
}

// Exec executes a query without returning rows.
func (c *Connection) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return c.DB.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows.
func (c *Connection) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return c.DB.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row.
func (c *Connection) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return c.DB.QueryRowContext(ctx, query, args...)
}

// Begin starts a transaction.
func (c *Connection) Begin(ctx context.Context) (*Tx, error) {
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &Tx{Tx: tx, Dialect: c.Dialect}, nil
}

// Close closes the database connection.
func (c *Connection) Close() error {
	return c.DB.Close()
}

// Tx represents a database transaction with dialect awareness.
type Tx struct {
	Tx      *sql.Tx
	Dialect Dialect
}

// Exec executes a query within the transaction.
func (t *Tx) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return t.Tx.ExecContext(ctx, query, args...)
}

// Query executes a query that returns rows within the transaction.
func (t *Tx) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return t.Tx.QueryContext(ctx, query, args...)
}

// QueryRow executes a query that returns at most one row within the transaction.
func (t *Tx) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return t.Tx.QueryRowContext(ctx, query, args...)
}

// Commit commits the transaction.
func (t *Tx) Commit() error {
	return t.Tx.Commit()
}

// Rollback aborts the transaction.
func (t *Tx) Rollback() error {
	return t.Tx.Rollback()
}
