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

func setupManyToManyDB(t *testing.T) (*dialects.Connection, *schema.Schema) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	dialect := sqlite.New()
	conn := dialects.NewConnection(db, dialect)
	ctx := context.Background()

	// Create tables for User <-> Tag many-to-many
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
		CREATE TABLE tags (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create tags table: %v", err)
	}

	// Junction table
	_, err = conn.Exec(ctx, `
		CREATE TABLE user_tags (
			user_id INTEGER,
			tag_id INTEGER,
			PRIMARY KEY (user_id, tag_id),
			FOREIGN KEY (user_id) REFERENCES users(id),
			FOREIGN KEY (tag_id) REFERENCES tags(id)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create user_tags table: %v", err)
	}

	// Create schema
	s := schema.NewSchema()
	s.Model("User", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("name")
		m.BelongsToMany("Tag", "user_tags", "user_id", "tag_id")
	})
	s.Model("Tag", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("name")
		m.BelongsToMany("User", "user_tags", "tag_id", "user_id")
	})

	return conn, s
}

func TestBelongsToManySetup(t *testing.T) {
	_, s := setupManyToManyDB(t)

	// Verify User has ManyToMany relation to Tag
	userModel := s.Models["User"]
	var foundTagRel bool
	for _, rel := range userModel.GetRelations() {
		if rel.TargetModel == "Tag" && rel.Type == schema.RelationManyToMany {
			foundTagRel = true
			if rel.Through != "user_tags" {
				t.Errorf("Expected Through='user_tags', got '%s'", rel.Through)
			}
			if rel.ThroughSourceKey != "user_id" {
				t.Errorf("Expected ThroughSourceKey='user_id', got '%s'", rel.ThroughSourceKey)
			}
			if rel.ThroughTargetKey != "tag_id" {
				t.Errorf("Expected ThroughTargetKey='tag_id', got '%s'", rel.ThroughTargetKey)
			}
		}
	}
	if !foundTagRel {
		t.Error("Expected User to have ManyToMany relation to Tag")
	}
}

func TestBelongsToManyEagerLoad(t *testing.T) {
	conn, s := setupManyToManyDB(t)
	defer conn.Close()
	ctx := context.Background()

	// Insert users
	users := query.New(conn, "users")
	users.Insert(map[string]interface{}{"name": "Alice"}).Exec(ctx)
	users.Insert(map[string]interface{}{"name": "Bob"}).Exec(ctx)

	// Insert tags
	tags := query.New(conn, "tags")
	tags.Insert(map[string]interface{}{"name": "Go"}).Exec(ctx)
	tags.Insert(map[string]interface{}{"name": "Python"}).Exec(ctx)
	tags.Insert(map[string]interface{}{"name": "Rust"}).Exec(ctx)

	// Create relationships: Alice -> Go, Python; Bob -> Go, Rust
	jt := query.New(conn, "user_tags")
	jt.Insert(map[string]interface{}{"user_id": 1, "tag_id": 1}).Exec(ctx) // Alice-Go
	jt.Insert(map[string]interface{}{"user_id": 1, "tag_id": 2}).Exec(ctx) // Alice-Python
	jt.Insert(map[string]interface{}{"user_id": 2, "tag_id": 1}).Exec(ctx) // Bob-Go
	jt.Insert(map[string]interface{}{"user_id": 2, "tag_id": 3}).Exec(ctx) // Bob-Rust

	// Query users with tags eager loaded
	usersQuery := query.NewWithSchema(conn, "users", s)
	results, err := usersQuery.Select().Include("Tag").All(ctx)
	if err != nil {
		t.Fatalf("Query with Include failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 users, got %d", len(results))
	}

	// Verify Alice has Go and Python tags
	for _, user := range results {
		tagsData, ok := user["Tag"]
		if !ok {
			t.Errorf("Expected Tag to be loaded for user %v", user["name"])
			continue
		}

		tagResults, ok := tagsData.(query.Results)
		if !ok {
			t.Errorf("Expected Tag to be query.Results, got %T", tagsData)
			continue
		}

		name := user["name"]
		switch name {
		case "Alice":
			if len(tagResults) != 2 {
				t.Errorf("Expected Alice to have 2 tags, got %d", len(tagResults))
			}
		case "Bob":
			if len(tagResults) != 2 {
				t.Errorf("Expected Bob to have 2 tags, got %d", len(tagResults))
			}
		}
	}
}

func TestBelongsToManyLazyLoad(t *testing.T) {
	conn, s := setupManyToManyDB(t)
	defer conn.Close()
	ctx := context.Background()

	// Insert data
	users := query.New(conn, "users")
	users.Insert(map[string]interface{}{"name": "Charlie"}).Exec(ctx)

	tags := query.New(conn, "tags")
	tags.Insert(map[string]interface{}{"name": "JavaScript"}).Exec(ctx)
	tags.Insert(map[string]interface{}{"name": "TypeScript"}).Exec(ctx)

	jt := query.New(conn, "user_tags")
	jt.Insert(map[string]interface{}{"user_id": 1, "tag_id": 1}).Exec(ctx)
	jt.Insert(map[string]interface{}{"user_id": 1, "tag_id": 2}).Exec(ctx)

	// Query with lazy loading
	usersQuery := query.NewWithSchema(conn, "users", s)
	results, err := usersQuery.Select().AllLazy(ctx)
	if err != nil {
		t.Fatalf("AllLazy failed: %v", err)
	}

	user := results[0]

	// Tags should NOT be loaded yet
	if user.IsLoaded("Tag") {
		t.Error("Tags should not be loaded before GetRelation")
	}

	// Lazy load tags
	tagsData, err := user.GetRelation(ctx, "Tag")
	if err != nil {
		t.Fatalf("GetRelation failed: %v", err)
	}

	if tagsData == nil {
		t.Fatal("Expected tags to be loaded")
	}

	// Tags should now be loaded
	if !user.IsLoaded("Tag") {
		t.Error("Tags should be loaded after GetRelation")
	}

	lazyTags := tagsData.(query.LazyResults)
	if len(lazyTags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(lazyTags))
	}
}

func TestBelongsToManyNoRelations(t *testing.T) {
	conn, s := setupManyToManyDB(t)
	defer conn.Close()
	ctx := context.Background()

	// Insert user with no tags
	users := query.New(conn, "users")
	users.Insert(map[string]interface{}{"name": "NoTags"}).Exec(ctx)

	// Query with eager loading
	usersQuery := query.NewWithSchema(conn, "users", s)
	results, err := usersQuery.Select().Include("Tag").All(ctx)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Should have empty Tags array
	tagsData := results[0]["Tag"]
	if tagsData == nil {
		t.Error("Expected Tag key to exist")
	}

	tagResults := tagsData.(query.Results)
	if len(tagResults) != 0 {
		t.Errorf("Expected 0 tags, got %d", len(tagResults))
	}
}
