// Package migration provides database migration functionality.
package migration

import (
	"context"
	"fmt"
	"os"
	"time"
)

// LockOptions configures migration locking behavior.
type LockOptions struct {
	// Timeout is how long to wait for lock acquisition (0 = immediate fail)
	Timeout time.Duration
	// LockTTL is how long before a lock is considered stale (default: 10 min)
	LockTTL time.Duration
	// Identifier is the process identifier for the lock (default: hostname)
	Identifier string
}

// DefaultLockOptions returns default lock configuration.
func DefaultLockOptions() LockOptions {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	return LockOptions{
		Timeout:    0,
		LockTTL:    10 * time.Minute,
		Identifier: hostname,
	}
}

// LockInfo contains information about the current lock.
type LockInfo struct {
	LockedAt  time.Time
	LockedBy  string
	ExpiresAt time.Time
	IsExpired bool
}

// initLockTable creates the lock table if it doesn't exist.
func (e *Engine) initLockTable(ctx context.Context) error {
	dialect := e.conn.Dialect
	sql := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id INTEGER PRIMARY KEY,
		locked_at TIMESTAMP NOT NULL,
		locked_by TEXT NOT NULL,
		expires_at TIMESTAMP NOT NULL
	)`, dialect.Quote(e.lockTableName))

	_, err := e.conn.Exec(ctx, sql)
	return err
}

// AcquireLock attempts to acquire the migration lock.
// Returns error if lock is held by another process and not expired.
func (e *Engine) AcquireLock(ctx context.Context, opts LockOptions) error {
	if err := e.initLockTable(ctx); err != nil {
		return fmt.Errorf("initializing lock table: %w", err)
	}

	if opts.Identifier == "" {
		opts.Identifier = DefaultLockOptions().Identifier
	}
	if opts.LockTTL == 0 {
		opts.LockTTL = DefaultLockOptions().LockTTL
	}

	dialect := e.conn.Dialect
	now := time.Now()
	expiresAt := now.Add(opts.LockTTL)

	// Check for existing lock
	lockInfo, err := e.GetLockInfo(ctx)
	if err != nil {
		return err
	}

	if lockInfo != nil {
		// Lock exists
		if !lockInfo.IsExpired {
			return fmt.Errorf("migrations locked by %s since %s (expires %s)",
				lockInfo.LockedBy,
				lockInfo.LockedAt.Format(time.RFC3339),
				lockInfo.ExpiresAt.Format(time.RFC3339))
		}
		// Lock is expired, remove it
		if err := e.ReleaseLock(ctx); err != nil {
			return fmt.Errorf("clearing expired lock: %w", err)
		}
	}

	// Insert new lock
	insertSQL := fmt.Sprintf(
		"INSERT INTO %s (id, locked_at, locked_by, expires_at) VALUES (1, %s, %s, %s)",
		dialect.Quote(e.lockTableName),
		dialect.Placeholder(1),
		dialect.Placeholder(2),
		dialect.Placeholder(3),
	)

	_, err = e.conn.Exec(ctx, insertSQL, now, opts.Identifier, expiresAt)
	if err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}

	return nil
}

// ReleaseLock releases the migration lock.
func (e *Engine) ReleaseLock(ctx context.Context) error {
	dialect := e.conn.Dialect
	deleteSQL := fmt.Sprintf("DELETE FROM %s WHERE id = 1", dialect.Quote(e.lockTableName))
	_, err := e.conn.Exec(ctx, deleteSQL)
	return err
}

// GetLockInfo returns information about the current lock, or nil if not locked.
func (e *Engine) GetLockInfo(ctx context.Context) (*LockInfo, error) {
	if err := e.initLockTable(ctx); err != nil {
		return nil, err
	}

	dialect := e.conn.Dialect
	query := fmt.Sprintf(
		"SELECT locked_at, locked_by, expires_at FROM %s WHERE id = 1",
		dialect.Quote(e.lockTableName),
	)

	row := e.conn.QueryRow(ctx, query)

	var info LockInfo
	err := row.Scan(&info.LockedAt, &info.LockedBy, &info.ExpiresAt)
	if err != nil {
		// No lock exists
		return nil, nil
	}

	info.IsExpired = time.Now().After(info.ExpiresAt)
	return &info, nil
}

// IsLocked checks if migrations are currently locked (and lock is not expired).
func (e *Engine) IsLocked(ctx context.Context) (bool, error) {
	info, err := e.GetLockInfo(ctx)
	if err != nil {
		return false, err
	}
	return info != nil && !info.IsExpired, nil
}

// WithLock executes a function with the migration lock held.
// Automatically acquires and releases the lock.
func (e *Engine) WithLock(ctx context.Context, opts LockOptions, fn func() error) error {
	if err := e.AcquireLock(ctx, opts); err != nil {
		return err
	}
	defer e.ReleaseLock(ctx)

	return fn()
}

// ForceUnlock removes the lock regardless of who holds it.
// Use with caution - only for breaking stale locks.
func (e *Engine) ForceUnlock(ctx context.Context) error {
	return e.ReleaseLock(ctx)
}
