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
func MigrateUp() error {
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

// MigrateDown rolls back the last migration.
func MigrateDown() error {
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
		return fmt.Errorf("loading migrations: %w", err)
	}

	// Rollback
	if err := engine.Down(ctx); err != nil {
		return fmt.Errorf("rolling back: %w", err)
	}

	fmt.Println("✓ Rolled back last migration")
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
