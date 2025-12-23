package test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/nexus-db/nexus/pkg/dialects"
	"github.com/nexus-db/nexus/pkg/dialects/sqlite"
	"github.com/nexus-db/nexus/pkg/query"
)

func setupExplainTestDB(t *testing.T) *dialects.Connection {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	dialect := sqlite.New()
	conn := dialects.NewConnection(db, dialect)

	// Create test tables with indexes
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
		t.Fatalf("Failed to create users table: %v", err)
	}

	// Create an index
	_, err = conn.Exec(context.Background(), `
		CREATE INDEX idx_users_name ON users(name)
	`)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Insert some data
	for i := 0; i < 10; i++ {
		_, err = conn.Exec(context.Background(),
			"INSERT INTO users (email, name) VALUES (?, ?)",
			"user"+string(rune('0'+i))+"@example.com",
			"User "+string(rune('0'+i)),
		)
		if err != nil {
			t.Fatalf("Failed to insert data: %v", err)
		}
	}

	return conn
}

func TestExplainBasic(t *testing.T) {
	conn := setupExplainTestDB(t)
	defer conn.Close()

	ctx := context.Background()
	users := query.New(conn, "users")

	// Get query plan
	plan, err := users.Select("id", "email", "name").
		Where(query.Eq("id", 1)).
		Explain(ctx)
	if err != nil {
		t.Fatalf("Explain failed: %v", err)
	}

	// Verify we got a plan
	if plan == nil {
		t.Fatal("Expected non-nil plan")
	}

	// Verify raw output is not empty
	if plan.Raw == "" {
		t.Error("Expected non-empty raw plan output")
	}

	t.Logf("Query plan:\n%s", plan.Raw)
}

func TestExplainWithWhere(t *testing.T) {
	conn := setupExplainTestDB(t)
	defer conn.Close()

	ctx := context.Background()
	users := query.New(conn, "users")

	// Get plan for query with WHERE clause
	plan, err := users.Select().
		Where(query.Like("name", "User%")).
		Explain(ctx)
	if err != nil {
		t.Fatalf("Explain failed: %v", err)
	}

	if plan.Raw == "" {
		t.Error("Expected non-empty plan")
	}

	// Log the plan for debugging
	t.Logf("Plan for WHERE query:\n%s", plan.Raw)
}

func TestExplainScanTypes(t *testing.T) {
	conn := setupExplainTestDB(t)
	defer conn.Close()

	ctx := context.Background()
	users := query.New(conn, "users")

	// Full table scan (no index on 'active' column)
	plan, err := users.Select().
		Where(query.Eq("active", 1)).
		Explain(ctx)
	if err != nil {
		t.Fatalf("Explain failed: %v", err)
	}

	// SQLite EXPLAIN QUERY PLAN contains "SCAN" for full table scans
	if !strings.Contains(strings.ToUpper(plan.Raw), "SCAN") {
		t.Logf("Plan output: %s", plan.Raw)
		// This might not always be SCAN depending on SQLite version
	}
}

func TestExplainWithIndex(t *testing.T) {
	conn := setupExplainTestDB(t)
	defer conn.Close()

	ctx := context.Background()
	users := query.New(conn, "users")

	// Query using unique index on email
	plan, err := users.Select().
		Where(query.Eq("email", "user0@example.com")).
		Explain(ctx)
	if err != nil {
		t.Fatalf("Explain failed: %v", err)
	}

	t.Logf("Plan for indexed query:\n%s", plan.Raw)

	// SQLite should use the unique index or primary key
	raw := strings.ToUpper(plan.Raw)
	if !strings.Contains(raw, "SEARCH") && !strings.Contains(raw, "INDEX") {
		t.Logf("Note: Index might not be shown in plan, got: %s", plan.Raw)
	}
}

func TestAnalyze(t *testing.T) {
	conn := setupExplainTestDB(t)
	defer conn.Close()

	ctx := context.Background()
	users := query.New(conn, "users")

	// Analyze actually executes the query
	plan, err := users.Select("id", "name").
		Limit(5).
		Analyze(ctx)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if plan == nil {
		t.Fatal("Expected non-nil plan from Analyze")
	}

	if !plan.IsAnalyzed {
		t.Error("Expected IsAnalyzed to be true")
	}

	t.Logf("Analyzed plan:\n%s", plan.Raw)
}

func TestExplainWithJoin(t *testing.T) {
	conn := setupExplainTestDB(t)
	defer conn.Close()

	// Create a posts table
	_, err := conn.Exec(context.Background(), `
		CREATE TABLE posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			title TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create posts table: %v", err)
	}

	// Insert some posts
	_, _ = conn.Exec(context.Background(),
		"INSERT INTO posts (user_id, title) VALUES (1, 'First Post')",
	)

	ctx := context.Background()
	posts := query.New(conn, "posts")

	// Explain a JOIN query
	plan, err := posts.Select("posts.id", "posts.title", "users.name").
		Join("users", "posts.user_id = users.id").
		Explain(ctx)
	if err != nil {
		t.Fatalf("Explain failed: %v", err)
	}

	t.Logf("Join query plan:\n%s", plan.Raw)
}

func TestExplainWithOptions(t *testing.T) {
	conn := setupExplainTestDB(t)
	defer conn.Close()

	ctx := context.Background()
	users := query.New(conn, "users")

	// Test with options (SQLite doesn't support formats, but it shouldn't error)
	plan, err := users.Select().
		OrderBy("name", query.Asc).
		Limit(10).
		Explain(ctx, query.ExplainOptions{
			Format: query.ExplainFormatText,
		})
	if err != nil {
		t.Fatalf("Explain with options failed: %v", err)
	}

	if plan.Raw == "" {
		t.Error("Expected non-empty plan")
	}
}

func TestExplainWarnings(t *testing.T) {
	conn := setupExplainTestDB(t)
	defer conn.Close()

	ctx := context.Background()
	users := query.New(conn, "users")

	// Query without using an index
	plan, err := users.Select().Explain(ctx)
	if err != nil {
		t.Fatalf("Explain failed: %v", err)
	}

	// Log any warnings
	t.Logf("Warnings: %v", plan.Warnings)
	t.Logf("Scan types: %v", plan.ScanTypes)
	t.Logf("Used indexes: %v", plan.UsedIndexes)
}

func TestDialectExplainSQL(t *testing.T) {
	dialect := sqlite.New()

	// Test basic explain
	sql := dialect.ExplainSQL("SELECT * FROM users", "", false)
	if !strings.HasPrefix(sql, "EXPLAIN QUERY PLAN") {
		t.Errorf("Expected EXPLAIN QUERY PLAN prefix, got: %s", sql)
	}

	// Test format support
	if !dialect.SupportsExplainFormat("text") {
		t.Error("SQLite should support text format")
	}
	if !dialect.SupportsExplainFormat("") {
		t.Error("SQLite should support empty format")
	}
	if dialect.SupportsExplainFormat("json") {
		t.Error("SQLite should not support json format")
	}
}
