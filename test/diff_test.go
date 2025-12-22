package test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/nexus-db/nexus/pkg/core/migration"
	"github.com/nexus-db/nexus/pkg/core/schema"
	"github.com/nexus-db/nexus/pkg/dialects/sqlite"
)

func setupDiffTestDB(t *testing.T) (*sql.DB, *sqlite.Dialect) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	return db, sqlite.New()
}

func TestDiff_NewTable(t *testing.T) {
	db, dialect := setupDiffTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Empty database snapshot
	snapshot, err := migration.IntrospectDatabase(ctx, db, dialect)
	if err != nil {
		t.Fatalf("Failed to introspect: %v", err)
	}

	// Schema with one table
	s := schema.NewSchema()
	s.Model("User", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("email").Unique()
		m.String("name").Null()
	})

	// Compute diff
	diff := migration.Diff(s, snapshot)

	if !diff.HasChanges() {
		t.Fatal("Expected changes but got none")
	}

	if len(diff.Changes) != 1 {
		t.Fatalf("Expected 1 change, got %d", len(diff.Changes))
	}

	if diff.Changes[0].Type != migration.ChangeCreateTable {
		t.Errorf("Expected CREATE TABLE, got %s", diff.Changes[0].Type)
	}

	if diff.Changes[0].TableName != "User" {
		t.Errorf("Expected table name 'User', got '%s'", diff.Changes[0].TableName)
	}
}

func TestDiff_DropTable(t *testing.T) {
	db, dialect := setupDiffTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Create a table in DB
	_, err := db.ExecContext(ctx, `CREATE TABLE OldTable (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Snapshot with the table
	snapshot, err := migration.IntrospectDatabase(ctx, db, dialect)
	if err != nil {
		t.Fatalf("Failed to introspect: %v", err)
	}

	// Empty schema (no tables)
	s := schema.NewSchema()

	// Compute diff
	diff := migration.Diff(s, snapshot)

	if !diff.HasChanges() {
		t.Fatal("Expected changes but got none")
	}

	if len(diff.Changes) != 1 {
		t.Fatalf("Expected 1 change, got %d", len(diff.Changes))
	}

	if diff.Changes[0].Type != migration.ChangeDropTable {
		t.Errorf("Expected DROP TABLE, got %s", diff.Changes[0].Type)
	}

	if diff.Changes[0].TableName != "OldTable" {
		t.Errorf("Expected table name 'OldTable', got '%s'", diff.Changes[0].TableName)
	}
}

func TestDiff_AddColumn(t *testing.T) {
	db, dialect := setupDiffTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Create a table with one column
	_, err := db.ExecContext(ctx, `CREATE TABLE User (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Snapshot
	snapshot, err := migration.IntrospectDatabase(ctx, db, dialect)
	if err != nil {
		t.Fatalf("Failed to introspect: %v", err)
	}

	// Schema with additional column
	s := schema.NewSchema()
	s.Model("User", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("email").Unique() // New column
	})

	// Compute diff
	diff := migration.Diff(s, snapshot)

	if !diff.HasChanges() {
		t.Fatal("Expected changes but got none")
	}

	// Should have ADD COLUMN for email
	found := false
	for _, change := range diff.Changes {
		if change.Type == migration.ChangeAddColumn && change.ColumnName == "email" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected ADD COLUMN for 'email'")
	}
}

func TestDiff_DropColumn(t *testing.T) {
	db, dialect := setupDiffTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Create a table with two columns
	_, err := db.ExecContext(ctx, `CREATE TABLE User (id INTEGER PRIMARY KEY, old_column TEXT)`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Snapshot
	snapshot, err := migration.IntrospectDatabase(ctx, db, dialect)
	if err != nil {
		t.Fatalf("Failed to introspect: %v", err)
	}

	// Schema with only id (old_column removed)
	s := schema.NewSchema()
	s.Model("User", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
	})

	// Compute diff
	diff := migration.Diff(s, snapshot)

	if !diff.HasChanges() {
		t.Fatal("Expected changes but got none")
	}

	// Should have DROP COLUMN for old_column
	found := false
	for _, change := range diff.Changes {
		if change.Type == migration.ChangeDropColumn && change.ColumnName == "old_column" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected DROP COLUMN for 'old_column'")
	}
}

func TestDiff_NoChanges(t *testing.T) {
	db, dialect := setupDiffTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Create a table matching schema
	_, err := db.ExecContext(ctx, `CREATE TABLE User (id INTEGER PRIMARY KEY, email TEXT NOT NULL UNIQUE)`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Snapshot
	snapshot, err := migration.IntrospectDatabase(ctx, db, dialect)
	if err != nil {
		t.Fatalf("Failed to introspect: %v", err)
	}

	// Matching schema
	s := schema.NewSchema()
	s.Model("User", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("email").Unique()
	})

	// Compute diff
	diff := migration.Diff(s, snapshot)

	if diff.HasChanges() {
		t.Errorf("Expected no changes, but got %d: %v", len(diff.Changes), migration.DescribeChanges(diff.Changes))
	}
}

func TestDiff_GenerateMigration(t *testing.T) {
	db, dialect := setupDiffTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Empty database
	snapshot, err := migration.IntrospectDatabase(ctx, db, dialect)
	if err != nil {
		t.Fatalf("Failed to introspect: %v", err)
	}

	// Schema with table
	s := schema.NewSchema()
	s.Model("User", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("email").Unique()
	})

	// Compute diff
	diff := migration.Diff(s, snapshot)

	// Generate migration
	m, err := migration.GenerateMigrationFromDiff(dialect, diff.Changes, "create_users")
	if err != nil {
		t.Fatalf("Failed to generate migration: %v", err)
	}

	if m.Name != "create_users" {
		t.Errorf("Expected name 'create_users', got '%s'", m.Name)
	}

	if m.UpSQL == "" {
		t.Error("Expected non-empty UpSQL")
	}

	if m.DownSQL == "" {
		t.Error("Expected non-empty DownSQL")
	}

	// Verify the migration contains CREATE TABLE
	if !contains(m.UpSQL, "CREATE TABLE") {
		t.Error("UpSQL should contain CREATE TABLE")
	}

	// Verify the migration contains DROP TABLE for rollback
	if !contains(m.DownSQL, "DROP TABLE") {
		t.Error("DownSQL should contain DROP TABLE")
	}
}

func TestDiff_DescribeChanges(t *testing.T) {
	changes := []migration.SchemaChange{
		{Type: migration.ChangeCreateTable, TableName: "User"},
		{Type: migration.ChangeAddColumn, TableName: "User", ColumnName: "email"},
		{Type: migration.ChangeDropColumn, TableName: "Post", ColumnName: "old"},
	}

	descriptions := migration.DescribeChanges(changes)

	if len(descriptions) != 3 {
		t.Fatalf("Expected 3 descriptions, got %d", len(descriptions))
	}

	if !contains(descriptions[0], "CREATE TABLE") {
		t.Errorf("Expected CREATE TABLE in first description, got: %s", descriptions[0])
	}

	if !contains(descriptions[1], "ADD COLUMN") {
		t.Errorf("Expected ADD COLUMN in second description, got: %s", descriptions[1])
	}

	if !contains(descriptions[2], "DROP COLUMN") {
		t.Errorf("Expected DROP COLUMN in third description, got: %s", descriptions[2])
	}
}

func TestIntrospect_SQLite(t *testing.T) {
	db, dialect := setupDiffTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Create a complex table
	_, err := db.ExecContext(ctx, `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			name TEXT,
			active INTEGER DEFAULT 1
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Create an index
	_, err = db.ExecContext(ctx, `CREATE INDEX idx_users_name ON users(name)`)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Introspect
	snapshot, err := migration.IntrospectDatabase(ctx, db, dialect)
	if err != nil {
		t.Fatalf("Failed to introspect: %v", err)
	}

	// Verify table exists
	table, ok := snapshot.Tables["users"]
	if !ok {
		t.Fatal("Table 'users' not found in snapshot")
	}

	// Verify columns
	if len(table.Columns) != 4 {
		t.Errorf("Expected 4 columns, got %d", len(table.Columns))
	}

	// Verify id column
	idCol := table.Columns["id"]
	if idCol == nil {
		t.Fatal("Column 'id' not found")
	}
	if !idCol.IsPrimaryKey {
		t.Error("id should be primary key")
	}

	// Verify email column
	emailCol := table.Columns["email"]
	if emailCol == nil {
		t.Fatal("Column 'email' not found")
	}
	if emailCol.Nullable {
		t.Error("email should not be nullable")
	}

	// Verify index
	if len(table.Indexes) < 1 {
		t.Error("Expected at least 1 index")
	}

	idx := table.Indexes["idx_users_name"]
	if idx == nil {
		t.Error("Index 'idx_users_name' not found")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
