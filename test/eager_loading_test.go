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

func setupEagerLoadingDB(t *testing.T) (*dialects.Connection, *schema.Schema) {
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

func TestPreloadBelongsTo(t *testing.T) {
	conn, s := setupEagerLoadingDB(t)
	defer conn.Close()
	ctx := context.Background()

	// Insert test data
	users := query.New(conn, "users")
	users.Insert(map[string]interface{}{"name": "Alice", "email": "alice@example.com"}).Exec(ctx)
	users.Insert(map[string]interface{}{"name": "Bob", "email": "bob@example.com"}).Exec(ctx)

	posts := query.New(conn, "posts")
	posts.Insert(map[string]interface{}{"title": "Alice Post 1", "user_id": 1}).Exec(ctx)
	posts.Insert(map[string]interface{}{"title": "Bob Post 1", "user_id": 2}).Exec(ctx)
	posts.Insert(map[string]interface{}{"title": "Alice Post 2", "user_id": 1}).Exec(ctx)

	// Query posts with User eager loaded
	postsWithUser := query.NewWithSchema(conn, "posts", s)
	results, err := postsWithUser.Select().Include("User").All(ctx)
	if err != nil {
		t.Fatalf("Query with Include failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 posts, got %d", len(results))
	}

	// Verify User is loaded for each post
	for _, post := range results {
		user, ok := post["User"]
		if !ok {
			t.Errorf("Expected User to be loaded for post %v", post["title"])
			continue
		}

		userMap, ok := user.(query.Result)
		if !ok {
			t.Errorf("Expected User to be query.Result, got %T", user)
			continue
		}

		userId := post["user_id"]
		expectedUserId := userMap["id"]
		if userId != expectedUserId {
			t.Errorf("User ID mismatch: post.user_id=%v, user.id=%v", userId, expectedUserId)
		}
	}
}

func TestPreloadHasMany(t *testing.T) {
	conn, s := setupEagerLoadingDB(t)
	defer conn.Close()
	ctx := context.Background()

	// Insert test data
	users := query.New(conn, "users")
	users.Insert(map[string]interface{}{"name": "Alice", "email": "alice@example.com"}).Exec(ctx)
	users.Insert(map[string]interface{}{"name": "Bob", "email": "bob@example.com"}).Exec(ctx)

	posts := query.New(conn, "posts")
	posts.Insert(map[string]interface{}{"title": "Alice Post 1", "user_id": 1}).Exec(ctx)
	posts.Insert(map[string]interface{}{"title": "Alice Post 2", "user_id": 1}).Exec(ctx)
	posts.Insert(map[string]interface{}{"title": "Bob Post 1", "user_id": 2}).Exec(ctx)

	// Query users with Posts eager loaded
	usersWithPosts := query.NewWithSchema(conn, "users", s)
	results, err := usersWithPosts.Select().Include("Post").All(ctx)
	if err != nil {
		t.Fatalf("Query with Include failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 users, got %d", len(results))
	}

	// Verify Posts are loaded
	for _, user := range results {
		postsData, ok := user["Post"]
		if !ok {
			t.Errorf("Expected Post to be loaded for user %v", user["name"])
			continue
		}

		postsSlice, ok := postsData.(query.Results)
		if !ok {
			t.Errorf("Expected Post to be query.Results, got %T", postsData)
			continue
		}

		name := user["name"]
		if name == "Alice" && len(postsSlice) != 2 {
			t.Errorf("Expected Alice to have 2 posts, got %d", len(postsSlice))
		}
		if name == "Bob" && len(postsSlice) != 1 {
			t.Errorf("Expected Bob to have 1 post, got %d", len(postsSlice))
		}
	}
}

func TestPreloadWithoutSchema(t *testing.T) {
	conn, _ := setupEagerLoadingDB(t)
	defer conn.Close()
	ctx := context.Background()

	// Insert test data
	users := query.New(conn, "users")
	users.Insert(map[string]interface{}{"name": "Alice", "email": "alice@example.com"}).Exec(ctx)

	posts := query.New(conn, "posts")
	posts.Insert(map[string]interface{}{"title": "Post 1", "user_id": 1}).Exec(ctx)

	// Query without schema - Include should be ignored gracefully
	postsQuery := query.New(conn, "posts")
	results, err := postsQuery.Select().Include("User").All(ctx)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Should still return posts, just without User loaded
	if len(results) != 1 {
		t.Errorf("Expected 1 post, got %d", len(results))
	}

	// User should NOT be loaded (no schema)
	if _, ok := results[0]["User"]; ok {
		t.Error("User should not be loaded without schema")
	}
}

func TestPreloadNonExistentRelation(t *testing.T) {
	conn, s := setupEagerLoadingDB(t)
	defer conn.Close()
	ctx := context.Background()

	users := query.New(conn, "users")
	users.Insert(map[string]interface{}{"name": "Alice", "email": "alice@example.com"}).Exec(ctx)

	// Include non-existent relation - should be ignored gracefully
	usersQuery := query.NewWithSchema(conn, "users", s)
	results, err := usersQuery.Select().Include("NonExistent").All(ctx)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 user, got %d", len(results))
	}
}
