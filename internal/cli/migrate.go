package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-db/nexus/pkg/core/migration"
	"github.com/nexus-db/nexus/pkg/core/schema"
	"github.com/nexus-db/nexus/pkg/dialects"
	"github.com/nexus-db/nexus/pkg/dialects/mysql"
	"github.com/nexus-db/nexus/pkg/dialects/postgres"
	"github.com/nexus-db/nexus/pkg/dialects/sqlite"
)

const migrationsDir = "migrations"

// MigrateNew creates a new migration file.
func MigrateNew(name string) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	// Parse schema
	s, err := schema.ParseFile(config.Schema.Path)
	if err != nil {
		return fmt.Errorf("parsing schema: %w", err)
	}

	if err := s.Validate(); err != nil {
		return fmt.Errorf("validating schema: %w", err)
	}

	// Get dialect
	dialect, err := getDialect(config.Database.Dialect)
	if err != nil {
		return err
	}

	// Create migrations directory
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return err
	}

	// Generate migration
	conn := dialects.NewConnection(nil, dialect)
	engine := migration.NewEngine(conn)

	m, err := engine.GenerateFromSchema(s, name)
	if err != nil {
		return fmt.Errorf("generating migration: %w", err)
	}

	// Save to file
	if err := migration.SaveMigration(migrationsDir, m); err != nil {
		return fmt.Errorf("saving migration: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.sql", m.ID, m.Name)
	fmt.Printf("✓ Created migration: %s/%s\n", migrationsDir, filename)

	return nil
}

// MigrateUp applies all pending migrations.
// If force is true, breaks any stale locks before proceeding.
func MigrateUp(force bool) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	conn, err := connect(config)
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx := context.Background()
	engine := migration.NewEngine(conn)

	// Initialize migrations table
	if err := engine.Init(ctx); err != nil {
		return fmt.Errorf("initializing migrations table: %w", err)
	}

	// Handle force unlock
	if force {
		if err := engine.ForceUnlock(ctx); err != nil {
			return fmt.Errorf("force unlocking: %w", err)
		}
	}

	// Acquire lock
	lockOpts := migration.DefaultLockOptions()
	if err := engine.AcquireLock(ctx, lockOpts); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer engine.ReleaseLock(ctx)

	// Load migrations
	if err := engine.LoadFromDir(migrationsDir); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No migrations directory found. Run 'nexus migrate new <name>' first.")
			return nil
		}
		return fmt.Errorf("loading migrations: %w", err)
	}

	// Apply pending
	applied, err := engine.Up(ctx)
	if err != nil {
		return fmt.Errorf("applying migrations: %w", err)
	}

	if applied == 0 {
		fmt.Println("No pending migrations.")
	} else {
		fmt.Printf("✓ Applied %d migration(s)\n", applied)
	}

	return nil
}

// MigrateDown rolls back migrations.
// If targetID is specified, rolls back to that migration (exclusive).
// If n > 0, rolls back n migrations.
// Otherwise rolls back just the last migration.
// If force is true, breaks any stale locks before proceeding.
func MigrateDown(targetID string, n int, force bool) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	conn, err := connect(config)
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx := context.Background()
	engine := migration.NewEngine(conn)

	// Initialize migrations table
	if err := engine.Init(ctx); err != nil {
		return fmt.Errorf("initializing migrations table: %w", err)
	}

	// Handle force unlock
	if force {
		if err := engine.ForceUnlock(ctx); err != nil {
			return fmt.Errorf("force unlocking: %w", err)
		}
	}

	// Acquire lock
	lockOpts := migration.DefaultLockOptions()
	if err := engine.AcquireLock(ctx, lockOpts); err != nil {
		return fmt.Errorf("acquiring lock: %w", err)
	}
	defer engine.ReleaseLock(ctx)

	// Load migrations
	if err := engine.LoadFromDir(migrationsDir); err != nil {
		return fmt.Errorf("loading migrations: %w", err)
	}

	// Determine rollback mode
	if targetID != "" {
		// Rollback to specific version
		count, err := engine.DownTo(ctx, targetID)
		if err != nil {
			return fmt.Errorf("rolling back to %s: %w", targetID, err)
		}
		if count == 0 {
			fmt.Printf("Already at or before migration %s\n", targetID)
		} else {
			fmt.Printf("✓ Rolled back %d migration(s) to %s\n", count, targetID)
		}
	} else if n > 0 {
		// Rollback n migrations
		count, err := engine.DownN(ctx, n)
		if err != nil {
			return fmt.Errorf("rolling back: %w", err)
		}
		fmt.Printf("✓ Rolled back %d migration(s)\n", count)
	} else {
		// Rollback just the last one
		if err := engine.Down(ctx); err != nil {
			return fmt.Errorf("rolling back: %w", err)
		}
		fmt.Println("✓ Rolled back last migration")
	}

	return nil
}

// MigrateStatus shows the status of all migrations.
func MigrateStatus() error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	conn, err := connect(config)
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx := context.Background()
	engine := migration.NewEngine(conn)

	// Initialize migrations table
	if err := engine.Init(ctx); err != nil {
		return fmt.Errorf("initializing migrations table: %w", err)
	}

	// Load migrations
	if err := engine.LoadFromDir(migrationsDir); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No migrations found.")
			return nil
		}
		return fmt.Errorf("loading migrations: %w", err)
	}

	// Get status
	status, err := engine.Status(ctx)
	if err != nil {
		return fmt.Errorf("getting status: %w", err)
	}

	if len(status) == 0 {
		fmt.Println("No migrations found.")
		return nil
	}

	fmt.Println("Migration Status:")
	fmt.Println(strings.Repeat("-", 60))
	for _, s := range status {
		indicator := "[ ]"
		appliedAt := ""
		if s.Applied {
			indicator = "[✓]"
			appliedAt = s.AppliedAt.Format(time.RFC3339)
		}
		fmt.Printf("%s %s_%s %s\n", indicator, s.ID, s.Name, appliedAt)
	}

	return nil
}

// MigrateReset drops all tables and reruns all migrations.
func MigrateReset() error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	conn, err := connect(config)
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx := context.Background()
	engine := migration.NewEngine(conn)

	// Load migrations
	if err := engine.LoadFromDir(migrationsDir); err != nil {
		return fmt.Errorf("loading migrations: %w", err)
	}

	// Rollback all
	for {
		if err := engine.Down(ctx); err != nil {
			break // No more migrations to rollback
		}
	}

	// Initialize and apply all
	if err := engine.Init(ctx); err != nil {
		return err
	}

	applied, err := engine.Up(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("✓ Reset complete. Applied %d migration(s)\n", applied)
	return nil
}

func connect(config *Config) (*dialects.Connection, error) {
	dialect, err := getDialect(config.Database.Dialect)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(dialect.DriverName(), config.Database.URL)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return dialects.NewConnection(db, dialect), nil
}

func getDialect(name string) (dialects.Dialect, error) {
	switch strings.ToLower(name) {
	case "postgres", "postgresql":
		return postgres.New(), nil
	case "sqlite", "sqlite3":
		return sqlite.New(), nil
	case "mysql":
		return mysql.New(), nil
	default:
		return nil, fmt.Errorf("unknown dialect: %s (supported: postgres, sqlite, mysql)", name)
	}
}

// MigrateFresh creates a migration from current schema state.
func MigrateFresh(name string) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	// Parse schema
	s, err := schema.ParseFile(config.Schema.Path)
	if err != nil {
		return fmt.Errorf("parsing schema: %w", err)
	}

	if err := s.Validate(); err != nil {
		return fmt.Errorf("validating schema: %w", err)
	}

	// Generate SQL for each model
	dialect, err := getDialect(config.Database.Dialect)
	if err != nil {
		return err
	}

	var upStatements []string
	var downStatements []string

	for _, model := range s.GetModels() {
		upStatements = append(upStatements, dialect.CreateTableSQL(model))
		downStatements = append(downStatements, dialect.DropTableSQL(model.Name))

		for _, idx := range model.Indexes {
			if len(idx.Fields) > 1 || !idx.Unique {
				upStatements = append(upStatements, dialect.CreateIndexSQL(model.Name, idx))
				downStatements = append(downStatements, dialect.DropIndexSQL(model.Name, idx.Name))
			}
		}
	}

	// Create migration file
	now := time.Now()
	id := now.Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s.sql", id, name)

	upSQL := strings.Join(upStatements, ";\n\n") + ";"
	downSQL := strings.Join(downStatements, ";\n\n") + ";"

	content := fmt.Sprintf("-- UP\n%s\n\n-- DOWN\n%s\n", upSQL, downSQL)

	// Ensure migrations directory exists
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return err
	}

	migrationPath := filepath.Join(migrationsDir, filename)
	if err := os.WriteFile(migrationPath, []byte(content), 0644); err != nil {
		return err
	}

	fmt.Printf("✓ Created migration: %s\n", migrationPath)
	return nil
}

// MigrateDiff compares the schema with the current database and generates a migration.
func MigrateDiff(name string) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	// Parse schema
	s, err := schema.ParseFile(config.Schema.Path)
	if err != nil {
		return fmt.Errorf("parsing schema: %w", err)
	}

	if err := s.Validate(); err != nil {
		return fmt.Errorf("validating schema: %w", err)
	}

	// Connect to database
	conn, err := connect(config)
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx := context.Background()

	// Get the introspector from the dialect
	introspector, ok := conn.Dialect.(migration.Introspector)
	if !ok {
		return fmt.Errorf("dialect %s does not support introspection", conn.Dialect.Name())
	}

	// Introspect current database state
	fmt.Println("Introspecting database...")
	snapshot, err := migration.IntrospectDatabase(ctx, conn.DB, introspector)
	if err != nil {
		return fmt.Errorf("introspecting database: %w", err)
	}

	// Compute diff
	fmt.Println("Computing schema diff...")
	diff := migration.Diff(s, snapshot)

	if !diff.HasChanges() {
		fmt.Println("No schema changes detected. Database is up to date.")
		return nil
	}

	// Display changes
	fmt.Println("\nDetected changes:")
	for _, desc := range migration.DescribeChanges(diff.Changes) {
		fmt.Printf("  %s\n", desc)
	}
	fmt.Println()

	// Generate migration
	m, err := migration.GenerateMigrationFromDiff(conn.Dialect, diff.Changes, name)
	if err != nil {
		return fmt.Errorf("generating migration: %w", err)
	}

	// Ensure migrations directory exists
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return err
	}

	// Save migration file
	if err := migration.SaveMigration(migrationsDir, m); err != nil {
		return fmt.Errorf("saving migration: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.sql", m.ID, m.Name)
	fmt.Printf("✓ Created migration: %s/%s\n", migrationsDir, filename)

	return nil
}

// MigrateSquash combines multiple migrations into a single optimized migration.
func MigrateSquash(name, fromID, toID string, keepOriginals bool) error {
	// Load migrations from directory
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no migrations directory found")
		}
		return fmt.Errorf("reading migrations: %w", err)
	}

	var migrations []*migration.Migration
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(migrationsDir, f.Name()))
		if err != nil {
			return fmt.Errorf("reading %s: %w", f.Name(), err)
		}

		m, err := parseMigrationFile(f.Name(), string(content))
		if err != nil {
			return fmt.Errorf("parsing %s: %w", f.Name(), err)
		}

		migrations = append(migrations, m)
	}

	if len(migrations) < 2 {
		return fmt.Errorf("need at least 2 migrations to squash, found %d", len(migrations))
	}

	fmt.Printf("Found %d migrations\n", len(migrations))

	// Squash migrations
	opts := migration.SquashOptions{
		FromID:     fromID,
		ToID:       toID,
		OutputName: name,
	}

	result, err := migration.SquashMigrations(migrations, opts)
	if err != nil {
		return fmt.Errorf("squashing migrations: %w", err)
	}

	fmt.Printf("\nSquash summary:\n")
	fmt.Printf("  Original migrations: %d\n", result.OriginalCount)
	fmt.Printf("  Statements after optimization: %d\n", result.OptimizedCount)
	if result.RemovedCount > 0 {
		fmt.Printf("  Redundant statements removed: %d\n", result.RemovedCount)
	}

	// Backup and/or delete original migrations
	if !keepOriginals {
		backupDir := filepath.Join(migrationsDir, ".squashed_backup")
		if err := os.MkdirAll(backupDir, 0755); err != nil {
			return fmt.Errorf("creating backup directory: %w", err)
		}

		for _, id := range result.OriginalIDs {
			// Find and move the file
			for _, f := range files {
				if strings.HasPrefix(f.Name(), id) {
					oldPath := filepath.Join(migrationsDir, f.Name())
					newPath := filepath.Join(backupDir, f.Name())
					if err := os.Rename(oldPath, newPath); err != nil {
						return fmt.Errorf("backing up %s: %w", f.Name(), err)
					}
				}
			}
		}
		fmt.Printf("\nOriginal migrations backed up to: %s/.squashed_backup/\n", migrationsDir)
	}

	// Save squashed migration
	if err := migration.SaveMigration(migrationsDir, result.Migration); err != nil {
		return fmt.Errorf("saving squashed migration: %w", err)
	}

	filename := fmt.Sprintf("%s_%s.sql", result.Migration.ID, result.Migration.Name)
	fmt.Printf("✓ Created squashed migration: %s/%s\n", migrationsDir, filename)

	return nil
}

// parseMigrationFile parses a migration file (local copy for CLI).
func parseMigrationFile(filename, content string) (*migration.Migration, error) {
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

	return &migration.Migration{
		ID:      id,
		Name:    name,
		UpSQL:   upSQL,
		DownSQL: downSQL,
	}, nil
}
