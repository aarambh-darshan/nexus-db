package test

import (
	"testing"

	"github.com/nexus-db/nexus/pkg/core/migration"
)

func TestSquash_BasicMerge(t *testing.T) {
	migrations := []*migration.Migration{
		{
			ID:      "20231201_100000",
			Name:    "create_users",
			UpSQL:   "CREATE TABLE users (id INTEGER PRIMARY KEY)",
			DownSQL: "DROP TABLE users",
		},
		{
			ID:      "20231202_100000",
			Name:    "create_posts",
			UpSQL:   "CREATE TABLE posts (id INTEGER PRIMARY KEY)",
			DownSQL: "DROP TABLE posts",
		},
	}

	result, err := migration.SquashMigrations(migrations, migration.SquashOptions{
		OutputName: "initial_schema",
	})

	if err != nil {
		t.Fatalf("Squash failed: %v", err)
	}

	if result.OriginalCount != 2 {
		t.Errorf("Expected 2 original migrations, got %d", result.OriginalCount)
	}

	if result.Migration.Name != "initial_schema" {
		t.Errorf("Expected name 'initial_schema', got '%s'", result.Migration.Name)
	}

	// Verify both CREATE statements are present
	if !containsSubstr(result.Migration.UpSQL, "CREATE TABLE users") {
		t.Error("UpSQL should contain CREATE TABLE users")
	}
	if !containsSubstr(result.Migration.UpSQL, "CREATE TABLE posts") {
		t.Error("UpSQL should contain CREATE TABLE posts")
	}
}

func TestSquash_OptimizeCreateDrop(t *testing.T) {
	migrations := []*migration.Migration{
		{
			ID:      "20231201_100000",
			Name:    "create_temp",
			UpSQL:   "CREATE TABLE temp_table (id INTEGER PRIMARY KEY)",
			DownSQL: "DROP TABLE temp_table",
		},
		{
			ID:      "20231202_100000",
			Name:    "create_users",
			UpSQL:   "CREATE TABLE users (id INTEGER PRIMARY KEY)",
			DownSQL: "DROP TABLE users",
		},
		{
			ID:      "20231203_100000",
			Name:    "drop_temp",
			UpSQL:   "DROP TABLE temp_table",
			DownSQL: "CREATE TABLE temp_table (id INTEGER PRIMARY KEY)",
		},
	}

	result, err := migration.SquashMigrations(migrations, migration.SquashOptions{
		OutputName: "optimized",
	})

	if err != nil {
		t.Fatalf("Squash failed: %v", err)
	}

	// temp_table CREATE and DROP should be removed
	if containsSubstr(result.Migration.UpSQL, "temp_table") {
		t.Error("temp_table should be optimized out")
	}

	// users should remain
	if !containsSubstr(result.Migration.UpSQL, "CREATE TABLE users") {
		t.Error("users table should remain")
	}

	if result.RemovedCount != 2 {
		t.Errorf("Expected 2 removed statements, got %d", result.RemovedCount)
	}
}

func TestSquash_OptimizeAddDropColumn(t *testing.T) {
	migrations := []*migration.Migration{
		{
			ID:      "20231201_100000",
			Name:    "add_column",
			UpSQL:   "ALTER TABLE users ADD COLUMN temp_col TEXT",
			DownSQL: "ALTER TABLE users DROP COLUMN temp_col",
		},
		{
			ID:      "20231202_100000",
			Name:    "add_email",
			UpSQL:   "ALTER TABLE users ADD COLUMN email TEXT",
			DownSQL: "ALTER TABLE users DROP COLUMN email",
		},
		{
			ID:      "20231203_100000",
			Name:    "drop_temp",
			UpSQL:   "ALTER TABLE users DROP COLUMN temp_col",
			DownSQL: "ALTER TABLE users ADD COLUMN temp_col TEXT",
		},
	}

	result, err := migration.SquashMigrations(migrations, migration.SquashOptions{
		OutputName: "columns",
	})

	if err != nil {
		t.Fatalf("Squash failed: %v", err)
	}

	// temp_col ADD and DROP should be removed
	if containsSubstr(result.Migration.UpSQL, "temp_col") {
		t.Error("temp_col should be optimized out")
	}

	// email should remain
	if !containsSubstr(result.Migration.UpSQL, "email") {
		t.Error("email column should remain")
	}
}

func TestSquash_OptimizeIndex(t *testing.T) {
	migrations := []*migration.Migration{
		{
			ID:      "20231201_100000",
			Name:    "create_index",
			UpSQL:   "CREATE INDEX idx_temp ON users(name)",
			DownSQL: "DROP INDEX idx_temp",
		},
		{
			ID:      "20231202_100000",
			Name:    "drop_index",
			UpSQL:   "DROP INDEX idx_temp",
			DownSQL: "CREATE INDEX idx_temp ON users(name)",
		},
	}

	result, err := migration.SquashMigrations(migrations, migration.SquashOptions{
		OutputName: "index_test",
	})

	// Should fail because everything cancels out
	if err == nil {
		t.Error("Expected error for empty result, got none")
	}

	if result != nil {
		t.Error("Result should be nil when all statements cancel")
	}
}

func TestSquash_RangeFilter(t *testing.T) {
	migrations := []*migration.Migration{
		{
			ID:      "20231201_100000",
			Name:    "first",
			UpSQL:   "CREATE TABLE first (id INTEGER)",
			DownSQL: "DROP TABLE first",
		},
		{
			ID:      "20231202_100000",
			Name:    "second",
			UpSQL:   "CREATE TABLE second (id INTEGER)",
			DownSQL: "DROP TABLE second",
		},
		{
			ID:      "20231203_100000",
			Name:    "third",
			UpSQL:   "CREATE TABLE third (id INTEGER)",
			DownSQL: "DROP TABLE third",
		},
	}

	result, err := migration.SquashMigrations(migrations, migration.SquashOptions{
		FromID:     "20231201",
		ToID:       "20231202",
		OutputName: "partial",
	})

	if err != nil {
		t.Fatalf("Squash failed: %v", err)
	}

	// Only first and second should be included
	if result.OriginalCount != 2 {
		t.Errorf("Expected 2 migrations in range, got %d", result.OriginalCount)
	}

	if containsSubstr(result.Migration.UpSQL, "third") {
		t.Error("third table should not be in squashed result")
	}
}

func TestSquash_SingleMigrationError(t *testing.T) {
	migrations := []*migration.Migration{
		{
			ID:      "20231201_100000",
			Name:    "only_one",
			UpSQL:   "CREATE TABLE users (id INTEGER)",
			DownSQL: "DROP TABLE users",
		},
	}

	_, err := migration.SquashMigrations(migrations, migration.SquashOptions{
		OutputName: "test",
	})

	if err == nil {
		t.Error("Expected error for single migration")
	}
}

func TestSquash_EmptyMigrationsError(t *testing.T) {
	migrations := []*migration.Migration{}

	_, err := migration.SquashMigrations(migrations, migration.SquashOptions{
		OutputName: "test",
	})

	if err == nil {
		t.Error("Expected error for empty migrations")
	}
}

func TestSquash_PreservesOrder(t *testing.T) {
	migrations := []*migration.Migration{
		{
			ID:      "20231201_100000",
			Name:    "first",
			UpSQL:   "CREATE TABLE users (id INTEGER PRIMARY KEY)",
			DownSQL: "DROP TABLE users",
		},
		{
			ID:      "20231202_100000",
			Name:    "second",
			UpSQL:   "ALTER TABLE users ADD COLUMN name TEXT",
			DownSQL: "ALTER TABLE users DROP COLUMN name",
		},
	}

	result, err := migration.SquashMigrations(migrations, migration.SquashOptions{
		OutputName: "ordered",
	})

	if err != nil {
		t.Fatalf("Squash failed: %v", err)
	}

	// CREATE TABLE should come before ALTER TABLE
	createIdx := indexOfSubstr(result.Migration.UpSQL, "CREATE TABLE")
	alterIdx := indexOfSubstr(result.Migration.UpSQL, "ALTER TABLE")

	if createIdx == -1 || alterIdx == -1 {
		t.Fatal("Both statements should be present")
	}

	if createIdx > alterIdx {
		t.Error("CREATE TABLE should come before ALTER TABLE")
	}
}

func containsSubstr(s, substr string) bool {
	return indexOfSubstr(s, substr) != -1
}

func indexOfSubstr(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
