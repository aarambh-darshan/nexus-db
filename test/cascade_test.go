package test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/nexus-db/nexus/pkg/core/schema"
	"github.com/nexus-db/nexus/pkg/dialects"
	"github.com/nexus-db/nexus/pkg/dialects/sqlite"
	"github.com/nexus-db/nexus/pkg/query"
)

func setupCascadeDB(t *testing.T) (*dialects.Connection, *schema.Schema) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	dialect := sqlite.New()
	conn := dialects.NewConnection(db, dialect)
	ctx := context.Background()

	// Create tables
	_, err = conn.Exec(ctx, `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}

	_, err = conn.Exec(ctx, `
		CREATE TABLE posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			user_id INTEGER
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create posts table: %v", err)
	}

	// Create schema with cascade relations
	s := schema.NewSchema()
	s.Model("User", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("name")
		// HasMany with cascade delete
		m.HasMany("Post", "user_id")
	})
	s.Model("Post", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("title")
		m.Int("user_id")
	})
	s.DetectRelations()

	// Set cascade on User's HasMany Post relation
	userModel := s.Models["User"]
	for _, rel := range userModel.GetRelations() {
		if rel.TargetModel == "Post" {
			rel.OnDelete(schema.Cascade)
		}
	}

	return conn, s
}

func TestCascadeDeleteHasMany(t *testing.T) {
	conn, s := setupCascadeDB(t)
	defer conn.Close()
	ctx := context.Background()

	// Insert user and posts
	users := query.New(conn, "users")
	users.Insert(map[string]interface{}{"name": "Alice"}).Exec(ctx)

	posts := query.New(conn, "posts")
	posts.Insert(map[string]interface{}{"title": "Post 1", "user_id": 1}).Exec(ctx)
	posts.Insert(map[string]interface{}{"title": "Post 2", "user_id": 1}).Exec(ctx)
	posts.Insert(map[string]interface{}{"title": "Post 3", "user_id": 1}).Exec(ctx)

	// Verify posts exist
	count, _ := posts.Select().Count(ctx)
	if count != 3 {
		t.Fatalf("Expected 3 posts, got %d", count)
	}

	// Delete user with cascade
	usersWithSchema := query.NewWithSchema(conn, "users", s)
	affected, err := usersWithSchema.Delete().
		Where(query.Eq("id", 1)).
		Cascade().
		Exec(ctx)

	if err != nil {
		t.Fatalf("Cascade delete failed: %v", err)
	}

	if affected != 1 {
		t.Errorf("Expected 1 user deleted, got %d", affected)
	}

	// Verify posts are also deleted
	postCount, _ := posts.Select().Count(ctx)
	if postCount != 0 {
		t.Errorf("Expected 0 posts after cascade delete, got %d", postCount)
	}
}

func TestDeleteWithoutCascade(t *testing.T) {
	conn, _ := setupCascadeDB(t)
	defer conn.Close()
	ctx := context.Background()

	// Insert user and posts
	users := query.New(conn, "users")
	users.Insert(map[string]interface{}{"name": "Bob"}).Exec(ctx)

	posts := query.New(conn, "posts")
	posts.Insert(map[string]interface{}{"title": "Bob's Post", "user_id": 1}).Exec(ctx)

	// Delete user WITHOUT cascade (posts should remain - SQLite doesn't enforce FK by default)
	affected, err := users.Delete().Where(query.Eq("id", 1)).Exec(ctx)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if affected != 1 {
		t.Errorf("Expected 1 user deleted, got %d", affected)
	}

	// Posts should still exist (no cascade)
	postCount, _ := posts.Select().Count(ctx)
	if postCount != 1 {
		t.Errorf("Expected 1 post still existing, got %d", postCount)
	}
}

func TestCascadeSetNull(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	dialect := sqlite.New()
	conn := dialects.NewConnection(db, dialect)
	defer conn.Close()
	ctx := context.Background()

	// Create tables
	conn.Exec(ctx, `CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`)
	conn.Exec(ctx, `CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT, user_id INTEGER)`)

	// Create schema with SetNull action
	s := schema.NewSchema()
	s.Model("User", func(m *schema.Model) {
		m.Int("id").PrimaryKey()
		m.String("name")
		m.HasMany("Post", "user_id")
	})
	s.Model("Post", func(m *schema.Model) {
		m.Int("id").PrimaryKey()
		m.String("title")
		m.Int("user_id").Null()
	})
	s.DetectRelations()

	// Set SetNull on User's HasMany Post relation
	userModel := s.Models["User"]
	for _, rel := range userModel.GetRelations() {
		if rel.TargetModel == "Post" {
			rel.OnDelete(schema.SetNull)
		}
	}

	// Insert data
	users := query.New(conn, "users")
	users.Insert(map[string]interface{}{"id": 1, "name": "Charlie"}).Exec(ctx)

	posts := query.New(conn, "posts")
	posts.Insert(map[string]interface{}{"id": 1, "title": "Post", "user_id": 1}).Exec(ctx)

	// Delete user with cascade (should set user_id to NULL)
	usersWithSchema := query.NewWithSchema(conn, "users", s)
	usersWithSchema.Delete().Where(query.Eq("id", 1)).Cascade().Exec(ctx)

	// Verify post still exists but user_id is NULL
	result, _ := posts.Select("user_id").Where(query.Eq("id", 1)).One(ctx)
	if result["user_id"] != nil {
		t.Errorf("Expected user_id to be NULL, got %v", result["user_id"])
	}
}

func TestCascadeRestrict(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	dialect := sqlite.New()
	conn := dialects.NewConnection(db, dialect)
	defer conn.Close()
	ctx := context.Background()

	// Create tables
	conn.Exec(ctx, `CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)`)
	conn.Exec(ctx, `CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT, user_id INTEGER)`)

	// Create schema with Restrict action
	s := schema.NewSchema()
	s.Model("User", func(m *schema.Model) {
		m.Int("id").PrimaryKey()
		m.String("name")
		m.HasMany("Post", "user_id")
	})
	s.Model("Post", func(m *schema.Model) {
		m.Int("id").PrimaryKey()
		m.String("title")
		m.Int("user_id")
	})
	s.DetectRelations()

	// Set Restrict on User's HasMany Post relation
	userModel := s.Models["User"]
	for _, rel := range userModel.GetRelations() {
		if rel.TargetModel == "Post" {
			rel.OnDelete(schema.Restrict)
		}
	}

	// Insert data
	users := query.New(conn, "users")
	users.Insert(map[string]interface{}{"id": 1, "name": "Dave"}).Exec(ctx)

	posts := query.New(conn, "posts")
	posts.Insert(map[string]interface{}{"id": 1, "title": "Post", "user_id": 1}).Exec(ctx)

	// Try to delete user with cascade - should fail with restrict
	usersWithSchema := query.NewWithSchema(conn, "users", s)
	_, err = usersWithSchema.Delete().Where(query.Eq("id", 1)).Cascade().Exec(ctx)

	if err == nil {
		t.Error("Expected error due to restrict, got nil")
	}
}
