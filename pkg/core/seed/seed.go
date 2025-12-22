// Package seed provides database seeding functionality.
package seed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/nexus-db/nexus/pkg/dialects"
)

// Seed represents a single seed file.
type Seed struct {
	Name        string    // Seed name (from filename or header)
	Description string    // Optional description
	SQL         string    // SQL statements to execute
	Order       int       // Execution order (from filename prefix)
	Env         string    // Environment (dev, test, prod, or empty for all)
	Checksum    string    // SHA256 hash of SQL
	AppliedAt   time.Time // When seed was applied (zero if pending)
}

// SeedHistory represents applied seeds stored in the database.
type SeedHistory struct {
	ID        int
	Name      string
	Env       string
	Checksum  string
	AppliedAt time.Time
}

// SeedStatus represents the status of a seed.
type SeedStatus struct {
	Name      string
	Env       string
	Applied   bool
	AppliedAt time.Time
}

// Engine manages seed operations.
type Engine struct {
	conn      *dialects.Connection
	seeds     []*Seed
	tableName string
}

// NewEngine creates a new seed engine.
func NewEngine(conn *dialects.Connection) *Engine {
	return &Engine{
		conn:      conn,
		tableName: "_nexus_seeds",
	}
}

// Init creates the seeds tracking table if it doesn't exist.
func (e *Engine) Init(ctx context.Context) error {
	dialect := e.conn.Dialect
	sql := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		env TEXT NOT NULL DEFAULT '',
		checksum TEXT NOT NULL,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(name, env)
	)`, dialect.Quote(e.tableName))

	_, err := e.conn.Exec(ctx, sql)
	return err
}

// LoadFromDir loads seeds from a directory.
// Directory structure:
//
//	seeds/
//	├── 001_users.sql       (runs for all environments)
//	├── 002_categories.sql
//	├── dev/
//	│   └── 001_test_data.sql
//	└── test/
//	    └── 001_fixtures.sql
func (e *Engine) LoadFromDir(dir string) error {
	// Load root-level seeds (all environments)
	if err := e.loadSeedsFromPath(dir, ""); err != nil {
		return err
	}

	// Load environment-specific seeds
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			envName := entry.Name()
			// Skip hidden directories
			if strings.HasPrefix(envName, ".") {
				continue
			}
			envPath := filepath.Join(dir, envName)
			if err := e.loadSeedsFromPath(envPath, envName); err != nil {
				return err
			}
		}
	}

	// Sort by order
	sort.Slice(e.seeds, func(i, j int) bool {
		if e.seeds[i].Order != e.seeds[j].Order {
			return e.seeds[i].Order < e.seeds[j].Order
		}
		return e.seeds[i].Name < e.seeds[j].Name
	})

	return nil
}

func (e *Engine) loadSeedsFromPath(dir, env string) error {
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
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

		seed, err := parseSeedFile(f.Name(), string(content), env)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", f.Name(), err)
		}

		e.seeds = append(e.seeds, seed)
	}

	return nil
}

// parseSeedFile parses a seed file.
// Expected filename format: 001_seed_name.sql or seed_name.sql
func parseSeedFile(filename, content, env string) (*Seed, error) {
	name := strings.TrimSuffix(filename, ".sql")
	order := 0

	// Extract order from prefix if present (e.g., 001_users)
	re := regexp.MustCompile(`^(\d+)_(.+)$`)
	if matches := re.FindStringSubmatch(name); len(matches) == 3 {
		fmt.Sscanf(matches[1], "%d", &order)
		name = matches[2]
	}

	// Parse header comments for metadata
	description := ""
	headerRe := regexp.MustCompile(`(?m)^--\s*description:\s*(.+)$`)
	if matches := headerRe.FindStringSubmatch(content); len(matches) == 2 {
		description = strings.TrimSpace(matches[1])
	}

	// Compute checksum
	hash := sha256.Sum256([]byte(content))
	checksum := hex.EncodeToString(hash[:])

	return &Seed{
		Name:        name,
		Description: description,
		SQL:         content,
		Order:       order,
		Env:         env,
		Checksum:    checksum,
	}, nil
}

// Run executes pending seeds for the specified environment.
// If env is empty, only runs seeds with no environment specified.
// If env is "*", runs all seeds regardless of environment.
func (e *Engine) Run(ctx context.Context, env string) (int, error) {
	applied, err := e.getApplied(ctx)
	if err != nil {
		return 0, err
	}

	appliedMap := make(map[string]bool)
	for _, h := range applied {
		key := h.Name + ":" + h.Env
		appliedMap[key] = true
	}

	count := 0
	for _, seed := range e.seeds {
		// Check if this seed should run for this environment
		if !shouldRunSeed(seed, env) {
			continue
		}

		key := seed.Name + ":" + seed.Env
		if appliedMap[key] {
			continue // Already applied
		}

		if err := e.applySeed(ctx, seed); err != nil {
			return count, fmt.Errorf("applying seed %s: %w", seed.Name, err)
		}
		count++
	}

	return count, nil
}

// shouldRunSeed determines if a seed should run for the given environment.
func shouldRunSeed(seed *Seed, env string) bool {
	// Run all seeds
	if env == "*" {
		return true
	}

	// Seed has no environment restriction - always run
	if seed.Env == "" {
		return true
	}

	// Seed matches requested environment
	return seed.Env == env
}

// Reset clears seed history and re-runs all seeds.
func (e *Engine) Reset(ctx context.Context, env string) (int, error) {
	dialect := e.conn.Dialect

	// Clear seed history for this environment
	var deleteSQL string
	if env == "*" || env == "" {
		deleteSQL = fmt.Sprintf("DELETE FROM %s", dialect.Quote(e.tableName))
		_, err := e.conn.Exec(ctx, deleteSQL)
		if err != nil {
			return 0, err
		}
	} else {
		deleteSQL = fmt.Sprintf("DELETE FROM %s WHERE env = %s OR env = ''",
			dialect.Quote(e.tableName), dialect.Placeholder(1))
		_, err := e.conn.Exec(ctx, deleteSQL, env)
		if err != nil {
			return 0, err
		}
	}

	// Re-run seeds
	return e.Run(ctx, env)
}

// Status returns the status of all seeds.
func (e *Engine) Status(ctx context.Context) ([]SeedStatus, error) {
	applied, err := e.getApplied(ctx)
	if err != nil {
		return nil, err
	}

	appliedMap := make(map[string]*SeedHistory)
	for i := range applied {
		key := applied[i].Name + ":" + applied[i].Env
		appliedMap[key] = &applied[i]
	}

	var status []SeedStatus
	for _, seed := range e.seeds {
		key := seed.Name + ":" + seed.Env
		s := SeedStatus{
			Name: seed.Name,
			Env:  seed.Env,
		}
		if h, ok := appliedMap[key]; ok {
			s.Applied = true
			s.AppliedAt = h.AppliedAt
		}
		status = append(status, s)
	}

	return status, nil
}

func (e *Engine) getApplied(ctx context.Context) ([]SeedHistory, error) {
	dialect := e.conn.Dialect
	query := fmt.Sprintf(
		"SELECT id, name, env, checksum, applied_at FROM %s ORDER BY id",
		dialect.Quote(e.tableName),
	)

	rows, err := e.conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []SeedHistory
	for rows.Next() {
		var h SeedHistory
		if err := rows.Scan(&h.ID, &h.Name, &h.Env, &h.Checksum, &h.AppliedAt); err != nil {
			return nil, err
		}
		history = append(history, h)
	}

	return history, rows.Err()
}

func (e *Engine) applySeed(ctx context.Context, seed *Seed) error {
	dialect := e.conn.Dialect

	// Execute seed SQL
	_, err := e.conn.Exec(ctx, seed.SQL)
	if err != nil {
		return err
	}

	// Record in history
	insertSQL := fmt.Sprintf(
		"INSERT INTO %s (name, env, checksum) VALUES (%s, %s, %s)",
		dialect.Quote(e.tableName),
		dialect.Placeholder(1),
		dialect.Placeholder(2),
		dialect.Placeholder(3),
	)

	_, err = e.conn.Exec(ctx, insertSQL, seed.Name, seed.Env, seed.Checksum)
	return err
}

// GetSeeds returns all loaded seeds.
func (e *Engine) GetSeeds() []*Seed {
	return e.seeds
}
