package test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/nexus-db/nexus/pkg/core/seed"
	"github.com/nexus-db/nexus/pkg/dialects"
	"github.com/nexus-db/nexus/pkg/dialects/sqlite"
)

func setupSeedTestDB(t *testing.T) *dialects.Connection {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	dialect := sqlite.New()
	conn := dialects.NewConnection(db, dialect)

	// Create test table
	_, err = conn.Exec(context.Background(), `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL,
			name TEXT
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	return conn
}

func createTestSeedsDir(t *testing.T) string {
	dir := t.TempDir()

	// Create main seed
	seed1 := `-- seed: users
-- description: Initial users

INSERT INTO users (email, name) VALUES ('admin@example.com', 'Admin');
`
	if err := os.WriteFile(filepath.Join(dir, "001_users.sql"), []byte(seed1), 0644); err != nil {
		t.Fatal(err)
	}

	// Create second seed
	seed2 := `INSERT INTO users (email, name) VALUES ('user@example.com', 'User');`
	if err := os.WriteFile(filepath.Join(dir, "002_more_users.sql"), []byte(seed2), 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestSeed_BasicRun(t *testing.T) {
	conn := setupSeedTestDB(t)
	defer conn.Close()

	dir := createTestSeedsDir(t)
	ctx := context.Background()

	engine := seed.NewEngine(conn)
	if err := engine.Init(ctx); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if err := engine.LoadFromDir(dir); err != nil {
		t.Fatalf("LoadFromDir failed: %v", err)
	}

	applied, err := engine.Run(ctx, "")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if applied != 2 {
		t.Errorf("Expected 2 applied seeds, got %d", applied)
	}

	// Verify data
	var count int
	row := conn.QueryRow(ctx, "SELECT COUNT(*) FROM users")
	if err := row.Scan(&count); err != nil {
		t.Fatal(err)
	}

	if count != 2 {
		t.Errorf("Expected 2 users, got %d", count)
	}
}

func TestSeed_Idempotency(t *testing.T) {
	conn := setupSeedTestDB(t)
	defer conn.Close()

	dir := createTestSeedsDir(t)
	ctx := context.Background()

	engine := seed.NewEngine(conn)
	engine.Init(ctx)
	engine.LoadFromDir(dir)

	// First run
	applied1, _ := engine.Run(ctx, "")
	if applied1 != 2 {
		t.Errorf("First run: expected 2, got %d", applied1)
	}

	// Second run - should apply nothing
	applied2, _ := engine.Run(ctx, "")
	if applied2 != 0 {
		t.Errorf("Second run: expected 0, got %d", applied2)
	}

	// Verify data unchanged
	var count int
	conn.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 users, got %d", count)
	}
}

func TestSeed_Reset(t *testing.T) {
	conn := setupSeedTestDB(t)
	defer conn.Close()

	dir := createTestSeedsDir(t)
	ctx := context.Background()

	engine := seed.NewEngine(conn)
	engine.Init(ctx)
	engine.LoadFromDir(dir)

	// First run
	engine.Run(ctx, "")

	// Delete original data
	conn.Exec(ctx, "DELETE FROM users")

	// Reset - should re-run all seeds
	applied, err := engine.Reset(ctx, "")
	if err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	if applied != 2 {
		t.Errorf("Reset: expected 2, got %d", applied)
	}

	var count int
	conn.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 users after reset, got %d", count)
	}
}

func TestSeed_Status(t *testing.T) {
	conn := setupSeedTestDB(t)
	defer conn.Close()

	dir := createTestSeedsDir(t)
	ctx := context.Background()

	engine := seed.NewEngine(conn)
	engine.Init(ctx)
	engine.LoadFromDir(dir)

	// Before run - nothing applied
	status1, _ := engine.Status(ctx)
	appliedCount := 0
	for _, s := range status1 {
		if s.Applied {
			appliedCount++
		}
	}
	if appliedCount != 0 {
		t.Errorf("Before run: expected 0 applied, got %d", appliedCount)
	}

	// After run
	engine.Run(ctx, "")
	status2, _ := engine.Status(ctx)
	appliedCount = 0
	for _, s := range status2 {
		if s.Applied {
			appliedCount++
		}
	}
	if appliedCount != 2 {
		t.Errorf("After run: expected 2 applied, got %d", appliedCount)
	}
}

func TestSeed_EnvironmentFilter(t *testing.T) {
	conn := setupSeedTestDB(t)
	defer conn.Close()

	dir := t.TempDir()
	ctx := context.Background()

	// Create main seed (no env)
	mainSeed := `INSERT INTO users (email, name) VALUES ('main@example.com', 'Main');`
	os.WriteFile(filepath.Join(dir, "001_main.sql"), []byte(mainSeed), 0644)

	// Create dev env directory and seed
	devDir := filepath.Join(dir, "dev")
	os.MkdirAll(devDir, 0755)
	devSeed := `INSERT INTO users (email, name) VALUES ('dev@example.com', 'Dev');`
	os.WriteFile(filepath.Join(devDir, "001_dev.sql"), []byte(devSeed), 0644)

	engine := seed.NewEngine(conn)
	engine.Init(ctx)
	engine.LoadFromDir(dir)

	// Run with dev env - should run both main and dev seeds
	applied, err := engine.Run(ctx, "dev")
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if applied != 2 {
		t.Errorf("Expected 2 seeds (main + dev), got %d", applied)
	}

	var count int
	conn.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if count != 2 {
		t.Errorf("Expected 2 users, got %d", count)
	}
}

func TestSeed_OrderPreservation(t *testing.T) {
	conn := setupSeedTestDB(t)
	defer conn.Close()

	dir := t.TempDir()
	ctx := context.Background()

	// Create seeds in non-alphabetical order
	seed3 := `INSERT INTO users (email, name) VALUES ('third@example.com', 'Third');`
	os.WriteFile(filepath.Join(dir, "003_third.sql"), []byte(seed3), 0644)

	seed1 := `INSERT INTO users (email, name) VALUES ('first@example.com', 'First');`
	os.WriteFile(filepath.Join(dir, "001_first.sql"), []byte(seed1), 0644)

	seed2 := `INSERT INTO users (email, name) VALUES ('second@example.com', 'Second');`
	os.WriteFile(filepath.Join(dir, "002_second.sql"), []byte(seed2), 0644)

	engine := seed.NewEngine(conn)
	engine.Init(ctx)
	engine.LoadFromDir(dir)

	// Verify order
	seeds := engine.GetSeeds()
	if len(seeds) != 3 {
		t.Fatalf("Expected 3 seeds, got %d", len(seeds))
	}

	expectedOrder := []string{"first", "second", "third"}
	for i, s := range seeds {
		if s.Name != expectedOrder[i] {
			t.Errorf("Position %d: expected %s, got %s", i, expectedOrder[i], s.Name)
		}
	}
}
