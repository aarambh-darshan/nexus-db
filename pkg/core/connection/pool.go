// Package connection provides database connection pooling and management.
package connection

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

// Pool represents a database connection pool.
type Pool struct {
	db       *sql.DB
	mu       sync.RWMutex
	maxConns int
	maxIdle  int
	maxLife  time.Duration
}

// PoolConfig configures the connection pool.
type PoolConfig struct {
	MaxOpenConns    int           // Maximum number of open connections
	MaxIdleConns    int           // Maximum number of idle connections
	ConnMaxLifetime time.Duration // Maximum lifetime of a connection
	ConnMaxIdleTime time.Duration // Maximum idle time before closing
}

// DefaultPoolConfig returns sensible defaults for a connection pool.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	}
}

// NewPool creates a new connection pool from an existing *sql.DB.
func NewPool(db *sql.DB, config PoolConfig) *Pool {
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	return &Pool{
		db:       db,
		maxConns: config.MaxOpenConns,
		maxIdle:  config.MaxIdleConns,
		maxLife:  config.ConnMaxLifetime,
	}
}

// DB returns the underlying *sql.DB.
func (p *Pool) DB() *sql.DB {
	return p.db
}

// Ping verifies the connection is alive.
func (p *Pool) Ping(ctx context.Context) error {
	return p.db.PingContext(ctx)
}

// Stats returns database connection pool statistics.
func (p *Pool) Stats() sql.DBStats {
	return p.db.Stats()
}

// Close closes the connection pool.
func (p *Pool) Close() error {
	return p.db.Close()
}

// Conn acquires a single connection from the pool.
func (p *Pool) Conn(ctx context.Context) (*sql.Conn, error) {
	return p.db.Conn(ctx)
}

// BeginTx starts a transaction with the given options.
func (p *Pool) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return p.db.BeginTx(ctx, opts)
}

// ExecContext executes a query without returning rows.
func (p *Pool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return p.db.ExecContext(ctx, query, args...)
}

// QueryContext executes a query that returns rows.
func (p *Pool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return p.db.QueryContext(ctx, query, args...)
}

// QueryRowContext executes a query expected to return at most one row.
func (p *Pool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return p.db.QueryRowContext(ctx, query, args...)
}

// HealthCheck performs a health check on the pool.
func (p *Pool) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := p.Ping(ctx); err != nil {
		return err
	}

	stats := p.Stats()
	if stats.OpenConnections >= p.maxConns {
		// Pool is at capacity - not an error but worth noting
	}

	return nil
}
