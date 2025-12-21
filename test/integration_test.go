package test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/nexus-db/nexus/pkg/core/schema"
	"github.com/nexus-db/nexus/pkg/dialects"
	"github.com/nexus-db/nexus/pkg/dialects/sqlite"
	"github.com/nexus-db/nexus/pkg/query"
)

func setupTestDB(t *testing.T) *dialects.Connection {
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
			email TEXT NOT NULL UNIQUE,
			name TEXT,
			active INTEGER DEFAULT 1,
			created_at TEXT DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	return conn
}

func TestSchemaDefinition(t *testing.T) {
	s := schema.NewSchema()

	s.Model("User", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("email").Unique()
		m.String("name").Null()
		m.Bool("active").Default(true)
		m.DateTime("created_at").DefaultNow()
	})

	// Validate
	if err := s.Validate(); err != nil {
		t.Errorf("Schema validation failed: %v", err)
	}

	// Check model exists
	user, exists := s.Models["User"]
	if !exists {
		t.Fatal("User model not found")
	}

	// Check fields
	if len(user.Fields) != 5 {
		t.Errorf("Expected 5 fields, got %d", len(user.Fields))
	}

	// Check primary key
	id := user.Fields["id"]
	if !id.IsPrimaryKey {
		t.Error("id should be primary key")
	}
	if !id.AutoIncrement {
		t.Error("id should be auto increment")
	}

	// Check unique
	email := user.Fields["email"]
	if !email.IsUnique {
		t.Error("email should be unique")
	}

	// Check nullable
	name := user.Fields["name"]
	if !name.Nullable {
		t.Error("name should be nullable")
	}
}

func TestSchemaValidation_NoPrimaryKey(t *testing.T) {
	s := schema.NewSchema()

	s.Model("Invalid", func(m *schema.Model) {
		m.String("name")
	})

	err := s.Validate()
	if err == nil {
		t.Error("Expected validation error for missing primary key")
	}
}

func TestInsertAndSelect(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()

	ctx := context.Background()
	users := query.New(conn, "users")

	// Insert
	_, err := users.Insert(map[string]interface{}{
		"email": "alice@example.com",
		"name":  "Alice",
	}).Exec(ctx)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Select all
	results, err := users.Select("id", "email", "name").All(ctx)
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0]["email"] != "alice@example.com" {
		t.Errorf("Expected alice@example.com, got %v", results[0]["email"])
	}
}

func TestSelectWithWhere(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()

	ctx := context.Background()
	users := query.New(conn, "users")

	// Insert multiple users
	users.Insert(map[string]interface{}{"email": "alice@example.com", "name": "Alice"}).Exec(ctx)
	users.Insert(map[string]interface{}{"email": "bob@example.com", "name": "Bob"}).Exec(ctx)
	users.Insert(map[string]interface{}{"email": "charlie@example.com", "name": "Charlie"}).Exec(ctx)

	// Select with condition
	result, err := users.Select().Where(query.Eq("email", "bob@example.com")).One(ctx)
	if err != nil {
		t.Fatalf("Select with where failed: %v", err)
	}

	if result["name"] != "Bob" {
		t.Errorf("Expected Bob, got %v", result["name"])
	}

	// Select with LIKE
	results, err := users.Select().Where(query.Like("email", "%@example.com")).All(ctx)
	if err != nil {
		t.Fatalf("Select with like failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}
}

func TestUpdate(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()

	ctx := context.Background()
	users := query.New(conn, "users")

	// Insert
	users.Insert(map[string]interface{}{"email": "alice@example.com", "name": "Alice"}).Exec(ctx)

	// Update
	affected, err := users.Update(map[string]interface{}{"name": "Alice Smith"}).
		Where(query.Eq("email", "alice@example.com")).
		Exec(ctx)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if affected != 1 {
		t.Errorf("Expected 1 affected row, got %d", affected)
	}

	// Verify update
	result, _ := users.Select("name").Where(query.Eq("email", "alice@example.com")).One(ctx)
	if result["name"] != "Alice Smith" {
		t.Errorf("Expected Alice Smith, got %v", result["name"])
	}
}

func TestDelete(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()

	ctx := context.Background()
	users := query.New(conn, "users")

	// Insert
	users.Insert(map[string]interface{}{"email": "alice@example.com", "name": "Alice"}).Exec(ctx)
	users.Insert(map[string]interface{}{"email": "bob@example.com", "name": "Bob"}).Exec(ctx)

	// Delete one
	affected, err := users.Delete().Where(query.Eq("email", "alice@example.com")).Exec(ctx)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if affected != 1 {
		t.Errorf("Expected 1 affected row, got %d", affected)
	}

	// Verify count
	count, _ := users.Select().Count(ctx)
	if count != 1 {
		t.Errorf("Expected 1 remaining, got %d", count)
	}
}

func TestCount(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()

	ctx := context.Background()
	users := query.New(conn, "users")

	// Insert multiple
	for i := 0; i < 5; i++ {
		users.Insert(map[string]interface{}{
			"email": fmt.Sprintf("user%d@example.com", i),
			"name":  fmt.Sprintf("User %d", i),
		}).Exec(ctx)
	}

	count, err := users.Select().Count(ctx)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}

	if count != 5 {
		t.Errorf("Expected 5, got %d", count)
	}
}

func TestExists(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()

	ctx := context.Background()
	users := query.New(conn, "users")

	// Check empty
	exists, err := users.Select().Where(query.Eq("email", "alice@example.com")).Exists(ctx)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("Expected not to exist")
	}

	// Insert and check
	users.Insert(map[string]interface{}{"email": "alice@example.com", "name": "Alice"}).Exec(ctx)

	exists, err = users.Select().Where(query.Eq("email", "alice@example.com")).Exists(ctx)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Expected to exist")
	}
}

func TestOrderByAndLimit(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()

	ctx := context.Background()
	users := query.New(conn, "users")

	// Insert in random order
	users.Insert(map[string]interface{}{"email": "charlie@example.com", "name": "Charlie"}).Exec(ctx)
	users.Insert(map[string]interface{}{"email": "alice@example.com", "name": "Alice"}).Exec(ctx)
	users.Insert(map[string]interface{}{"email": "bob@example.com", "name": "Bob"}).Exec(ctx)

	// Order by name ascending, limit 2
	results, err := users.Select("name").
		OrderBy("name", query.Asc).
		Limit(2).
		All(ctx)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	if results[0]["name"] != "Alice" {
		t.Errorf("Expected Alice first, got %v", results[0]["name"])
	}
	if results[1]["name"] != "Bob" {
		t.Errorf("Expected Bob second, got %v", results[1]["name"])
	}
}

func TestTransaction(t *testing.T) {
	conn := setupTestDB(t)
	defer conn.Close()

	ctx := context.Background()

	// Successful transaction
	err := query.Transaction(ctx, conn, func(tx *dialects.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO users (email, name) VALUES (?, ?)", "alice@example.com", "Alice")
		return err
	})
	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// Verify insert
	count, _ := query.New(conn, "users").Select().Count(ctx)
	if count != 1 {
		t.Errorf("Expected 1, got %d", count)
	}

	// Failed transaction (should rollback)
	err = query.Transaction(ctx, conn, func(tx *dialects.Tx) error {
		tx.Exec(ctx, "INSERT INTO users (email, name) VALUES (?, ?)", "bob@example.com", "Bob")
		return fmt.Errorf("intentional error")
	})
	if err == nil {
		t.Error("Expected error")
	}

	// Verify rollback (still only 1 user)
	count, _ = query.New(conn, "users").Select().Count(ctx)
	if count != 1 {
		t.Errorf("Expected 1 (rollback), got %d", count)
	}
}
