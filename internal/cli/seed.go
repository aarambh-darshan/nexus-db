package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/nexus-db/nexus/pkg/core/seed"
	"github.com/nexus-db/nexus/pkg/dialects"
	"github.com/nexus-db/nexus/pkg/dialects/mysql"
	"github.com/nexus-db/nexus/pkg/dialects/postgres"
	"github.com/nexus-db/nexus/pkg/dialects/sqlite"
)

const seedsDir = "seeds"

// SeedRun runs pending seeds for the specified environment.
func SeedRun(env string, reset bool) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	conn, err := connectForSeed(config)
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx := context.Background()
	engine := seed.NewEngine(conn)

	// Initialize seeds table
	if err := engine.Init(ctx); err != nil {
		return fmt.Errorf("initializing seeds table: %w", err)
	}

	// Load seeds
	if err := engine.LoadFromDir(seedsDir); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No seeds directory found. Create 'seeds/' with .sql files.")
			return nil
		}
		return fmt.Errorf("loading seeds: %w", err)
	}

	seeds := engine.GetSeeds()
	if len(seeds) == 0 {
		fmt.Println("No seed files found.")
		return nil
	}

	fmt.Printf("Found %d seed(s)\n", len(seeds))

	var applied int
	if reset {
		fmt.Println("Resetting seeds...")
		applied, err = engine.Reset(ctx, env)
	} else {
		applied, err = engine.Run(ctx, env)
	}

	if err != nil {
		return fmt.Errorf("running seeds: %w", err)
	}

	if applied == 0 {
		fmt.Println("No pending seeds.")
	} else {
		fmt.Printf("✓ Applied %d seed(s)\n", applied)
	}

	return nil
}

// SeedStatus shows the status of all seeds.
func SeedStatus() error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	conn, err := connectForSeed(config)
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx := context.Background()
	engine := seed.NewEngine(conn)

	// Initialize seeds table
	if err := engine.Init(ctx); err != nil {
		return fmt.Errorf("initializing seeds table: %w", err)
	}

	// Load seeds
	if err := engine.LoadFromDir(seedsDir); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("No seeds directory found.")
			return nil
		}
		return fmt.Errorf("loading seeds: %w", err)
	}

	// Get status
	status, err := engine.Status(ctx)
	if err != nil {
		return fmt.Errorf("getting status: %w", err)
	}

	if len(status) == 0 {
		fmt.Println("No seeds found.")
		return nil
	}

	fmt.Println("Seed Status:")
	fmt.Println(strings.Repeat("-", 60))
	for _, s := range status {
		indicator := "[ ]"
		appliedAt := ""
		envLabel := ""
		if s.Applied {
			indicator = "[✓]"
			appliedAt = s.AppliedAt.Format(time.RFC3339)
		}
		if s.Env != "" {
			envLabel = fmt.Sprintf(" [%s]", s.Env)
		}
		fmt.Printf("%s %s%s %s\n", indicator, s.Name, envLabel, appliedAt)
	}

	return nil
}

// SeedCreate creates a new seed file.
func SeedCreate(name, env string) error {
	// Ensure seeds directory exists
	targetDir := seedsDir
	if env != "" {
		targetDir = fmt.Sprintf("%s/%s", seedsDir, env)
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("creating seeds directory: %w", err)
	}

	// Find next order number
	files, err := os.ReadDir(targetDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	nextOrder := 1
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".sql") {
			var order int
			if _, err := fmt.Sscanf(f.Name(), "%d_", &order); err == nil {
				if order >= nextOrder {
					nextOrder = order + 1
				}
			}
		}
	}

	// Create seed file
	filename := fmt.Sprintf("%03d_%s.sql", nextOrder, name)
	filepath := fmt.Sprintf("%s/%s", targetDir, filename)

	content := fmt.Sprintf(`-- seed: %s
-- description: Add description here

-- Add your seed data here
-- Example:
-- INSERT INTO users (email, name) VALUES
--   ('admin@example.com', 'Admin'),
--   ('user@example.com', 'User');
`, name)

	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing seed file: %w", err)
	}

	fmt.Printf("✓ Created seed: %s\n", filepath)
	return nil
}

func connectForSeed(config *Config) (*dialects.Connection, error) {
	dialect, err := getDialectForSeed(config.Database.Dialect)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(dialect.DriverName(), config.Database.URL)
	if err != nil {
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return dialects.NewConnection(db, dialect), nil
}

func getDialectForSeed(name string) (dialects.Dialect, error) {
	switch strings.ToLower(name) {
	case "postgres", "postgresql":
		return postgres.New(), nil
	case "sqlite", "sqlite3":
		return sqlite.New(), nil
	case "mysql":
		return mysql.New(), nil
	default:
		return nil, fmt.Errorf("unknown dialect: %s", name)
	}
}
