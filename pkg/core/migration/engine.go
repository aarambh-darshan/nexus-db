// Package migration provides database migration functionality.
package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nexus-db/nexus/pkg/core/schema"
	"github.com/nexus-db/nexus/pkg/dialects"
)

// Migration represents a single database migration.
type Migration struct {
	ID        string    // Unique identifier (timestamp-based)
	Name      string    // Human-readable name
	UpSQL     string    // SQL to apply migration
	DownSQL   string    // SQL to rollback migration
	Checksum  string    // SHA256 hash of UpSQL
	AppliedAt time.Time // When migration was applied (zero if pending)
}

// MigrationHistory represents applied migrations stored in the database.
type MigrationHistory struct {
	ID          int
	MigrationID string
	Name        string
	Checksum    string
	AppliedAt   time.Time
}

// Engine manages database migrations.
type Engine struct {
	conn       *dialects.Connection
	migrations []*Migration
	tableName  string
}

// NewEngine creates a new migration engine.
func NewEngine(conn *dialects.Connection) *Engine {
	return &Engine{
		conn:      conn,
		tableName: "_nexus_migrations",
	}
}

// Init creates the migrations table if it doesn't exist.
func (e *Engine) Init(ctx context.Context) error {
	dialect := e.conn.Dialect
	sql := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id INTEGER PRIMARY KEY,
		migration_id TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		checksum TEXT NOT NULL,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`, dialect.Quote(e.tableName))

	_, err := e.conn.Exec(ctx, sql)
	return err
}

// LoadFromDir loads migrations from a directory.
func (e *Engine) LoadFromDir(dir string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			return err
		}

		migration, err := parseMigrationFile(f.Name(), string(content))
		if err != nil {
			return fmt.Errorf("parsing %s: %w", f.Name(), err)
		}

		e.migrations = append(e.migrations, migration)
	}

	// Sort by ID (timestamp)
	sort.Slice(e.migrations, func(i, j int) bool {
		return e.migrations[i].ID < e.migrations[j].ID
	})

	return nil
}

// parseMigrationFile parses a migration file with UP/DOWN sections.
func parseMigrationFile(filename, content string) (*Migration, error) {
	// Expected format: 20231221_123000_create_users.sql
	parts := strings.SplitN(strings.TrimSuffix(filename, ".sql"), "_", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid migration filename format")
	}

	id := parts[0] + "_" + parts[1]
	name := parts[2]

	// Parse UP and DOWN sections
	upSQL, downSQL := "", ""
	sections := strings.Split(content, "-- DOWN")
	if len(sections) >= 1 {
		upPart := strings.TrimPrefix(sections[0], "-- UP")
		upSQL = strings.TrimSpace(upPart)
	}
	if len(sections) >= 2 {
		downSQL = strings.TrimSpace(sections[1])
	}

	hash := sha256.Sum256([]byte(upSQL))
	checksum := hex.EncodeToString(hash[:])

	return &Migration{
		ID:       id,
		Name:     name,
		UpSQL:    upSQL,
		DownSQL:  downSQL,
		Checksum: checksum,
	}, nil
}

// Pending returns migrations that haven't been applied yet.
func (e *Engine) Pending(ctx context.Context) ([]*Migration, error) {
	applied, err := e.getApplied(ctx)
	if err != nil {
		return nil, err
	}

	appliedMap := make(map[string]bool)
	for _, m := range applied {
		appliedMap[m.MigrationID] = true
	}

	var pending []*Migration
	for _, m := range e.migrations {
		if !appliedMap[m.ID] {
			pending = append(pending, m)
		}
	}

	return pending, nil
}

// Up applies all pending migrations.
func (e *Engine) Up(ctx context.Context) (int, error) {
	pending, err := e.Pending(ctx)
	if err != nil {
		return 0, err
	}

	for _, m := range pending {
		if err := e.applyMigration(ctx, m); err != nil {
			return 0, fmt.Errorf("applying migration %s: %w", m.ID, err)
		}
	}

	return len(pending), nil
}

// Down rolls back the last applied migration.
func (e *Engine) Down(ctx context.Context) error {
	applied, err := e.getApplied(ctx)
	if err != nil {
		return err
	}

	if len(applied) == 0 {
		return fmt.Errorf("no migrations to rollback")
	}

	// Get the last applied migration
	last := applied[len(applied)-1]

	// Find corresponding migration
	var migration *Migration
	for _, m := range e.migrations {
		if m.ID == last.MigrationID {
			migration = m
			break
		}
	}

	if migration == nil {
		return fmt.Errorf("migration %s not found in loaded migrations", last.MigrationID)
	}

	return e.rollbackMigration(ctx, migration)
}

// Status returns the status of all migrations.
func (e *Engine) Status(ctx context.Context) ([]MigrationStatus, error) {
	applied, err := e.getApplied(ctx)
	if err != nil {
		return nil, err
	}

	appliedMap := make(map[string]*MigrationHistory)
	for i := range applied {
		appliedMap[applied[i].MigrationID] = &applied[i]
	}

	var status []MigrationStatus
	for _, m := range e.migrations {
		s := MigrationStatus{
			ID:   m.ID,
			Name: m.Name,
		}
		if h, ok := appliedMap[m.ID]; ok {
			s.Applied = true
			s.AppliedAt = h.AppliedAt
		}
		status = append(status, s)
	}

	return status, nil
}

// MigrationStatus represents the status of a migration.
type MigrationStatus struct {
	ID        string
	Name      string
	Applied   bool
	AppliedAt time.Time
}

func (e *Engine) getApplied(ctx context.Context) ([]MigrationHistory, error) {
	dialect := e.conn.Dialect
	query := fmt.Sprintf(
		"SELECT id, migration_id, name, checksum, applied_at FROM %s ORDER BY id",
		dialect.Quote(e.tableName),
	)

	rows, err := e.conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []MigrationHistory
	for rows.Next() {
		var h MigrationHistory
		if err := rows.Scan(&h.ID, &h.MigrationID, &h.Name, &h.Checksum, &h.AppliedAt); err != nil {
			return nil, err
		}
		history = append(history, h)
	}

	return history, rows.Err()
}

func (e *Engine) applyMigration(ctx context.Context, m *Migration) error {
	dialect := e.conn.Dialect

	// Execute migration SQL
	_, err := e.conn.Exec(ctx, m.UpSQL)
	if err != nil {
		return err
	}

	// Record in history
	insertSQL := fmt.Sprintf(
		"INSERT INTO %s (migration_id, name, checksum) VALUES (%s, %s, %s)",
		dialect.Quote(e.tableName),
		dialect.Placeholder(1),
		dialect.Placeholder(2),
		dialect.Placeholder(3),
	)

	_, err = e.conn.Exec(ctx, insertSQL, m.ID, m.Name, m.Checksum)
	return err
}

func (e *Engine) rollbackMigration(ctx context.Context, m *Migration) error {
	dialect := e.conn.Dialect

	if m.DownSQL == "" {
		return fmt.Errorf("migration %s has no DOWN section", m.ID)
	}

	// Execute rollback SQL
	_, err := e.conn.Exec(ctx, m.DownSQL)
	if err != nil {
		return err
	}

	// Remove from history
	deleteSQL := fmt.Sprintf(
		"DELETE FROM %s WHERE migration_id = %s",
		dialect.Quote(e.tableName),
		dialect.Placeholder(1),
	)

	_, err = e.conn.Exec(ctx, deleteSQL, m.ID)
	return err
}

// GenerateFromSchema generates migrations from schema changes.
func (e *Engine) GenerateFromSchema(s *schema.Schema, name string) (*Migration, error) {
	dialect := e.conn.Dialect

	var upStatements []string
	var downStatements []string

	for _, model := range s.GetModels() {
		upStatements = append(upStatements, dialect.CreateTableSQL(model))
		downStatements = append(downStatements, dialect.DropTableSQL(model.Name))

		// Create indexes
		for _, idx := range model.Indexes {
			if len(idx.Fields) > 1 || !idx.Unique {
				upStatements = append(upStatements, dialect.CreateIndexSQL(model.Name, idx))
				downStatements = append(downStatements, dialect.DropIndexSQL(model.Name, idx.Name))
			}
		}
	}

	now := time.Now()
	id := now.Format("20060102_150405")

	upSQL := strings.Join(upStatements, ";\n\n") + ";"
	downSQL := strings.Join(downStatements, ";\n\n") + ";"

	hash := sha256.Sum256([]byte(upSQL))
	checksum := hex.EncodeToString(hash[:])

	return &Migration{
		ID:       id,
		Name:     name,
		UpSQL:    upSQL,
		DownSQL:  downSQL,
		Checksum: checksum,
	}, nil
}

// SaveMigration saves a migration to a file.
func SaveMigration(dir string, m *Migration) error {
	filename := fmt.Sprintf("%s_%s.sql", m.ID, m.Name)
	content := fmt.Sprintf("-- UP\n%s\n\n-- DOWN\n%s\n", m.UpSQL, m.DownSQL)
	return os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644)
}
