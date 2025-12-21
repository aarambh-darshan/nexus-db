package test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/nexus-db/nexus/pkg/dialects"
	"github.com/nexus-db/nexus/pkg/dialects/sqlite"
	"github.com/nexus-db/nexus/pkg/query"
)

func setupV2TestDB(t *testing.T) *dialects.Connection {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	dialect := sqlite.New()
	conn := dialects.NewConnection(db, dialect)

	// Create test tables
	_, err = conn.Exec(context.Background(), `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			active INTEGER DEFAULT 1
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}

	_, err = conn.Exec(context.Background(), `
		CREATE TABLE orders (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			amount REAL NOT NULL
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create orders table: %v", err)
	}

	// Seed data
	conn.Exec(context.Background(), "INSERT INTO users (name, active) VALUES ('Alice', 1)")
	conn.Exec(context.Background(), "INSERT INTO users (name, active) VALUES ('Bob', 1)")
	conn.Exec(context.Background(), "INSERT INTO users (name, active) VALUES ('Charlie', 0)")

	conn.Exec(context.Background(), "INSERT INTO orders (user_id, amount) VALUES (1, 100.00)")
	conn.Exec(context.Background(), "INSERT INTO orders (user_id, amount) VALUES (1, 200.00)")
	conn.Exec(context.Background(), "INSERT INTO orders (user_id, amount) VALUES (2, 50.00)")

	return conn
}

// === Raw SQL Tests ===

func TestRawQuery(t *testing.T) {
	conn := setupV2TestDB(t)
	defer conn.Close()
	ctx := context.Background()

	results, err := query.NewRawQuery(conn, "SELECT * FROM users WHERE active = ?", 1).All(ctx)
	if err != nil {
		t.Fatalf("Raw query failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 active users, got %d", len(results))
	}
}

func TestRawExec(t *testing.T) {
	conn := setupV2TestDB(t)
	defer conn.Close()
	ctx := context.Background()

	result, err := query.RawExec(ctx, conn, "UPDATE users SET active = ? WHERE name = ?", 0, "Alice")
	if err != nil {
		t.Fatalf("Raw exec failed: %v", err)
	}

	affected, _ := result.RowsAffected()
	if affected != 1 {
		t.Errorf("Expected 1 affected, got %d", affected)
	}

	// Verify
	results, _ := query.RawQueryAll(ctx, conn, "SELECT * FROM users WHERE active = 0")
	if len(results) != 2 { // Charlie + Alice now
		t.Errorf("Expected 2 inactive users, got %d", len(results))
	}
}

func TestRawQueryScan(t *testing.T) {
	conn := setupV2TestDB(t)
	defer conn.Close()
	ctx := context.Background()

	var id int
	var name string
	err := query.NewRawQuery(conn, "SELECT id, name FROM users WHERE id = ?", 1).Scan(ctx, &id, &name)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	if id != 1 || name != "Alice" {
		t.Errorf("Expected 1, Alice, got %d, %s", id, name)
	}
}

// === Set Operations Tests ===

func TestUnion(t *testing.T) {
	conn := setupV2TestDB(t)
	defer conn.Close()
	ctx := context.Background()

	q1 := query.New(conn, "users").Select("id", "name").Where(query.Eq("name", "Alice"))
	q2 := query.New(conn, "users").Select("id", "name").Where(query.Eq("name", "Bob"))

	results, err := q1.Union(q2).All(ctx)
	if err != nil {
		t.Fatalf("Union failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results from union, got %d", len(results))
	}
}

func TestUnionAll(t *testing.T) {
	conn := setupV2TestDB(t)
	defer conn.Close()
	ctx := context.Background()

	// Get all active users twice
	q1 := query.New(conn, "users").Select("name").Where(query.Eq("active", 1))
	q2 := query.New(conn, "users").Select("name").Where(query.Eq("active", 1))

	results, err := q1.UnionAll(q2).All(ctx)
	if err != nil {
		t.Fatalf("UnionAll failed: %v", err)
	}

	// Should have duplicates (2 active users * 2)
	if len(results) != 4 {
		t.Errorf("Expected 4 results from union all, got %d", len(results))
	}
}

func TestUnionWithOrderAndLimit(t *testing.T) {
	conn := setupV2TestDB(t)
	defer conn.Close()
	ctx := context.Background()

	q1 := query.New(conn, "users").Select("id", "name")
	q2 := query.New(conn, "users").Select("id", "name")

	results, err := q1.Union(q2).OrderBy("name", query.Asc).Limit(2).All(ctx)
	if err != nil {
		t.Fatalf("Union with order failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// First should be Alice (alphabetically)
	if results[0]["name"] != "Alice" {
		t.Errorf("Expected Alice first, got %v", results[0]["name"])
	}
}

// === CTE Tests ===

func TestBasicCTE(t *testing.T) {
	conn := setupV2TestDB(t)
	defer conn.Close()
	ctx := context.Background()

	// WITH active_users AS (SELECT * FROM users WHERE active = 1)
	// SELECT * FROM active_users
	activeUsers := query.New(conn, "users").Select("id", "name").Where(query.Eq("active", 1))

	results, err := query.With(conn, "active_users", activeUsers).
		Select("*").From("active_users").All(ctx)
	if err != nil {
		t.Fatalf("CTE query failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 active users from CTE, got %d", len(results))
	}
}

func TestCTEWithWhere(t *testing.T) {
	conn := setupV2TestDB(t)
	defer conn.Close()
	ctx := context.Background()

	activeUsers := query.New(conn, "users").Select("id", "name").Where(query.Eq("active", 1))

	result, err := query.With(conn, "active_users", activeUsers).
		Select("*").From("active_users").
		Where(query.Eq("name", "Alice")).
		One(ctx)
	if err != nil {
		t.Fatalf("CTE with where failed: %v", err)
	}

	if result["name"] != "Alice" {
		t.Errorf("Expected Alice, got %v", result["name"])
	}
}

// === Subquery Tests ===

func TestWhereInSubquery(t *testing.T) {
	conn := setupV2TestDB(t)
	defer conn.Close()
	ctx := context.Background()

	// Get users who have orders
	usersWithOrders := query.New(conn, "orders").Select("user_id")

	results, err := query.New(conn, "users").Select("name").
		WhereIn("id", usersWithOrders).
		All(ctx)
	if err != nil {
		t.Fatalf("WhereIn subquery failed: %v", err)
	}

	// Alice and Bob have orders
	if len(results) != 2 {
		t.Errorf("Expected 2 users with orders, got %d", len(results))
	}
}

func TestWhereNotInSubquery(t *testing.T) {
	conn := setupV2TestDB(t)
	defer conn.Close()
	ctx := context.Background()

	// Get users who have NO orders
	usersWithOrders := query.New(conn, "orders").Select("user_id")

	results, err := query.New(conn, "users").Select("name").
		WhereNotIn("id", usersWithOrders).
		All(ctx)
	if err != nil {
		t.Fatalf("WhereNotIn subquery failed: %v", err)
	}

	// Only Charlie has no orders
	if len(results) != 1 {
		t.Errorf("Expected 1 user without orders, got %d", len(results))
	}

	if results[0]["name"] != "Charlie" {
		t.Errorf("Expected Charlie, got %v", results[0]["name"])
	}
}

// === Cache Tests ===

func TestStmtCache(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	cache := query.NewStmtCacheWithStats(db, 10)

	// First access - miss
	_, err = cache.Get("SELECT 1")
	if err != nil {
		t.Fatalf("Cache get failed: %v", err)
	}

	// Second access - hit
	_, err = cache.Get("SELECT 1")
	if err != nil {
		t.Fatalf("Cache get failed: %v", err)
	}

	stats := cache.Stats()
	if stats.Hits != 1 {
		t.Errorf("Expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}

	if cache.HitRate() != 0.5 {
		t.Errorf("Expected 50%% hit rate, got %f", cache.HitRate())
	}
}

func TestCacheEviction(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	cache := query.NewStmtCache(db, 2) // Only 2 slots

	cache.Get("SELECT 1")
	cache.Get("SELECT 2")
	cache.Get("SELECT 3") // Should evict SELECT 1

	if cache.Size() != 2 {
		t.Errorf("Expected cache size 2, got %d", cache.Size())
	}
}

// === Logging Tests ===

func TestQueryStats(t *testing.T) {
	collector := query.NewStatsCollector(100 * 1e6) // 100ms threshold

	collector.Record("SELECT 1", 10*1e6, nil)  // 10ms
	collector.Record("SELECT 2", 200*1e6, nil) // 200ms - slow
	collector.Record("SELECT 3", 5*1e6, nil)   // 5ms

	stats := collector.Stats()

	if stats.TotalQueries != 3 {
		t.Errorf("Expected 3 total queries, got %d", stats.TotalQueries)
	}

	if stats.SlowQueries != 1 {
		t.Errorf("Expected 1 slow query, got %d", stats.SlowQueries)
	}

	if stats.Errors != 0 {
		t.Errorf("Expected 0 errors, got %d", stats.Errors)
	}
}
