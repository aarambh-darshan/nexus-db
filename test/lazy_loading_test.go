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

func setupLazyLoadingDB(t *testing.T) (*dialects.Connection, *schema.Schema) {
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
			name TEXT NOT NULL,
			email TEXT UNIQUE
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}

	_, err = conn.Exec(ctx, `
		CREATE TABLE posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			content TEXT,
			user_id INTEGER,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create posts table: %v", err)
	}

	// Create schema
	s := schema.NewSchema()
	s.Model("User", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("name")
		m.String("email").Unique()
	})
	s.Model("Post", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("title")
		m.Text("content").Null()
		m.Int("user_id")
	})
	s.DetectRelations()

	return conn, s
}

func TestLazyLoadBelongsTo(t *testing.T) {
	conn, s := setupLazyLoadingDB(t)
	defer conn.Close()
	ctx := context.Background()

	// Insert test data
	users := query.New(conn, "users")
	users.Insert(map[string]interface{}{"name": "Alice", "email": "alice@example.com"}).Exec(ctx)

	posts := query.New(conn, "posts")
	posts.Insert(map[string]interface{}{"title": "Post 1", "user_id": 1}).Exec(ctx)

	// Query posts with lazy loading
	postsQuery := query.NewWithSchema(conn, "posts", s)
	results, err := postsQuery.Select().AllLazy(ctx)
	if err != nil {
		t.Fatalf("AllLazy failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 post, got %d", len(results))
	}

	post := results[0]

	// User should NOT be loaded yet
	if post.IsLoaded("User") {
		t.Error("User should not be loaded before GetRelation")
	}

	// Lazy load User
	user, err := post.GetRelation(ctx, "User")
	if err != nil {
		t.Fatalf("GetRelation failed: %v", err)
	}

	if user == nil {
		t.Fatal("Expected user to be loaded")
	}

	// User should now be loaded
	if !post.IsLoaded("User") {
		t.Error("User should be loaded after GetRelation")
	}

	// Verify user data
	lazyUser := user.(*query.LazyResult)
	if lazyUser.Get("name") != "Alice" {
		t.Errorf("Expected Alice, got %v", lazyUser.Get("name"))
	}
}

func TestLazyCaching(t *testing.T) {
	conn, s := setupLazyLoadingDB(t)
	defer conn.Close()
	ctx := context.Background()

	// Insert test data
	users := query.New(conn, "users")
	users.Insert(map[string]interface{}{"name": "Bob", "email": "bob@example.com"}).Exec(ctx)

	posts := query.New(conn, "posts")
	posts.Insert(map[string]interface{}{"title": "Bob's Post", "user_id": 1}).Exec(ctx)

	// Query with lazy loading
	postsQuery := query.NewWithSchema(conn, "posts", s)
	results, err := postsQuery.Select().AllLazy(ctx)
	if err != nil {
		t.Fatalf("AllLazy failed: %v", err)
	}

	post := results[0]

	// Load user twice
	user1, _ := post.GetRelation(ctx, "User")
	user2, _ := post.GetRelation(ctx, "User")

	// Should be the same cached instance
	if user1 != user2 {
		t.Error("Expected cached result, got different instances")
	}
}

func TestLazyLoadHasMany(t *testing.T) {
	conn, s := setupLazyLoadingDB(t)
	defer conn.Close()
	ctx := context.Background()

	// Insert test data
	users := query.New(conn, "users")
	users.Insert(map[string]interface{}{"name": "Charlie", "email": "charlie@example.com"}).Exec(ctx)

	posts := query.New(conn, "posts")
	posts.Insert(map[string]interface{}{"title": "Post 1", "user_id": 1}).Exec(ctx)
	posts.Insert(map[string]interface{}{"title": "Post 2", "user_id": 1}).Exec(ctx)
	posts.Insert(map[string]interface{}{"title": "Post 3", "user_id": 1}).Exec(ctx)

	// Query users with lazy loading
	usersQuery := query.NewWithSchema(conn, "users", s)
	results, err := usersQuery.Select().AllLazy(ctx)
	if err != nil {
		t.Fatalf("AllLazy failed: %v", err)
	}

	user := results[0]

	// Lazy load Posts
	postsData, err := user.GetRelation(ctx, "Post")
	if err != nil {
		t.Fatalf("GetRelation failed: %v", err)
	}

	lazyPosts := postsData.(query.LazyResults)
	if len(lazyPosts) != 3 {
		t.Errorf("Expected 3 posts, got %d", len(lazyPosts))
	}
}

func TestLazyNonExistentRelation(t *testing.T) {
	conn, s := setupLazyLoadingDB(t)
	defer conn.Close()
	ctx := context.Background()

	users := query.New(conn, "users")
	users.Insert(map[string]interface{}{"name": "Dave", "email": "dave@example.com"}).Exec(ctx)

	usersQuery := query.NewWithSchema(conn, "users", s)
	results, _ := usersQuery.Select().AllLazy(ctx)
	user := results[0]

	// Get non-existent relation
	result, err := user.GetRelation(ctx, "NonExistent")
	if err != nil {
		t.Errorf("Expected no error for non-existent relation, got %v", err)
	}
	if result != nil {
		t.Error("Expected nil for non-existent relation")
	}
}

func TestOneLazy(t *testing.T) {
	conn, s := setupLazyLoadingDB(t)
	defer conn.Close()
	ctx := context.Background()

	users := query.New(conn, "users")
	users.Insert(map[string]interface{}{"name": "Eve", "email": "eve@example.com"}).Exec(ctx)

	usersQuery := query.NewWithSchema(conn, "users", s)
	user, err := usersQuery.Select().OneLazy(ctx)
	if err != nil {
		t.Fatalf("OneLazy failed: %v", err)
	}

	if user == nil {
		t.Fatal("Expected user")
	}

	if user.Get("name") != "Eve" {
		t.Errorf("Expected Eve, got %v", user.Get("name"))
	}
}

func TestLazyResultToResults(t *testing.T) {
	conn, s := setupLazyLoadingDB(t)
	defer conn.Close()
	ctx := context.Background()

	users := query.New(conn, "users")
	users.Insert(map[string]interface{}{"name": "Frank", "email": "frank@example.com"}).Exec(ctx)

	usersQuery := query.NewWithSchema(conn, "users", s)
	lazyResults, _ := usersQuery.Select().AllLazy(ctx)

	// Convert to regular Results
	results := lazyResults.ToResults()
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0]["name"] != "Frank" {
		t.Errorf("Expected Frank, got %v", results[0]["name"])
	}
}
