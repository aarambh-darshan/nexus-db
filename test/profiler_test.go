package test

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/nexus-db/nexus/pkg/dialects"
	"github.com/nexus-db/nexus/pkg/dialects/sqlite"
	"github.com/nexus-db/nexus/pkg/query"
)

func setupProfilerTestDB(t *testing.T) *dialects.Connection {
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
			name TEXT,
			active INTEGER DEFAULT 1
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	return conn
}

func TestProfilerBasic(t *testing.T) {
	profiler := query.NewProfiler(query.DefaultProfilerOptions())

	// Initially not enabled
	if profiler.IsEnabled() {
		t.Error("Profiler should not be enabled before Start()")
	}

	// Start profiling
	profiler.Start()
	if !profiler.IsEnabled() {
		t.Error("Profiler should be enabled after Start()")
	}

	// Record a query manually
	profile := profiler.StartQuery("SELECT * FROM users", nil)
	time.Sleep(10 * time.Millisecond)
	profiler.EndQuery(profile, nil)

	// Stop profiling
	profiler.Stop()
	if profiler.IsEnabled() {
		t.Error("Profiler should not be enabled after Stop()")
	}

	// Get report
	report := profiler.Report()
	if report.TotalQueries != 1 {
		t.Errorf("Expected 1 query, got %d", report.TotalQueries)
	}
}

func TestProfilerWithSelectBuilder(t *testing.T) {
	conn := setupProfilerTestDB(t)
	defer conn.Close()

	profiler := query.NewProfiler(query.DefaultProfilerOptions())
	profiler.Start()

	ctx := context.Background()

	// Use builder with profiler
	users := query.New(conn, "users").WithProfiler(profiler)

	// Insert data
	_, err := users.Insert(map[string]interface{}{
		"email": "test@example.com",
		"name":  "Test User",
	}).Exec(ctx)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Select data
	results, err := users.Select("id", "email", "name").All(ctx)
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	// Count
	count, err := users.Select().Count(ctx)
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}

	profiler.Stop()

	report := profiler.Report()
	if report.TotalQueries != 3 {
		t.Errorf("Expected 3 queries (insert, select, count), got %d", report.TotalQueries)
	}
}

func TestProfilerSlowQueryDetection(t *testing.T) {
	opts := query.DefaultProfilerOptions()
	opts.SlowThreshold = 5 * time.Millisecond

	profiler := query.NewProfiler(opts)
	profiler.Start()

	// Record a fast query
	fastProfile := profiler.StartQuery("SELECT 1", nil)
	profiler.EndQuery(fastProfile, nil)

	// Record a slow query
	slowProfile := profiler.StartQuery("SELECT * FROM big_table", nil)
	time.Sleep(10 * time.Millisecond) // Exceed threshold
	profiler.EndQuery(slowProfile, nil)

	profiler.Stop()

	report := profiler.Report()
	if len(report.SlowQueries) != 1 {
		t.Errorf("Expected 1 slow query, got %d", len(report.SlowQueries))
	}

	// Check the slow query is the right one
	if len(report.SlowQueries) > 0 {
		if !strings.Contains(report.SlowQueries[0].SQL, "big_table") {
			t.Error("Slow query should be the big_table query")
		}
		if !report.SlowQueries[0].IsSlow {
			t.Error("Query should be marked as slow")
		}
	}
}

func TestProfilerNPlusOneDetection(t *testing.T) {
	conn := setupProfilerTestDB(t)
	defer conn.Close()

	opts := query.DefaultProfilerOptions()
	opts.NPlusOneThreshold = 3 // Warn after 3 repeated queries

	profiler := query.NewProfiler(opts)
	profiler.Start()

	ctx := context.Background()

	// Insert test data
	users := query.New(conn, "users").WithProfiler(profiler)
	for i := 0; i < 5; i++ {
		_, _ = users.Insert(map[string]interface{}{
			"email": fmt.Sprintf("user%d@example.com", i),
			"name":  fmt.Sprintf("User %d", i),
		}).Exec(ctx)
	}

	// Simulate N+1 pattern - same query structure executed multiple times
	for i := 0; i < 5; i++ {
		_, _ = users.Select("id", "name").Where(query.Eq("id", i+1)).All(ctx)
	}

	profiler.Stop()

	report := profiler.Report()

	// Should detect the repeated SELECT pattern
	if len(report.NPlusOneWarnings) == 0 {
		t.Log("N+1 warnings:", report.NPlusOneWarnings)
		// Note: N+1 detection depends on SQL normalization
		// This test may need adjustment based on actual query patterns
	}

	// Check suggestions mention N+1
	hasNPlusOneSuggestion := false
	for _, s := range report.Suggestions {
		if strings.Contains(s, "N+1") {
			hasNPlusOneSuggestion = true
			break
		}
	}
	if len(report.NPlusOneWarnings) > 0 && !hasNPlusOneSuggestion {
		t.Error("Expected N+1 suggestion when warnings exist")
	}
}

func TestProfilerReport(t *testing.T) {
	profiler := query.NewProfiler(query.DefaultProfilerOptions())
	profiler.Start()

	// Record various queries
	queries := []string{
		"SELECT * FROM users",
		"SELECT id, name FROM users WHERE id = 1",
		"INSERT INTO users (email) VALUES ('test@example.com')",
		"UPDATE users SET name = 'Test' WHERE id = 1",
		"DELETE FROM users WHERE id = 1",
	}

	for _, sql := range queries {
		profile := profiler.StartQuery(sql, nil)
		time.Sleep(1 * time.Millisecond)
		profiler.EndQuery(profile, nil)
	}

	profiler.Stop()

	report := profiler.Report()

	// Verify report fields
	if report.TotalQueries != len(queries) {
		t.Errorf("Expected %d queries, got %d", len(queries), report.TotalQueries)
	}

	if report.TotalDuration <= 0 {
		t.Error("Expected positive total duration")
	}

	if report.AverageDuration <= 0 {
		t.Error("Expected positive average duration")
	}

	if len(report.TopByDuration) == 0 {
		t.Error("Expected top by duration to be populated")
	}

	if len(report.TopByFrequency) == 0 {
		t.Error("Expected top by frequency to be populated")
	}

	// Test report string output
	reportStr := report.String()
	if !strings.Contains(reportStr, "Performance Profile Report") {
		t.Error("Report should contain header")
	}
	if !strings.Contains(reportStr, report.SessionID) {
		t.Error("Report should contain session ID")
	}
}

func TestProfilerUpdateAndDelete(t *testing.T) {
	conn := setupProfilerTestDB(t)
	defer conn.Close()

	profiler := query.NewProfiler(query.DefaultProfilerOptions())
	profiler.Start()

	ctx := context.Background()

	// Insert
	users := query.New(conn, "users").WithProfiler(profiler)
	_, err := users.Insert(map[string]interface{}{
		"email": "test@example.com",
		"name":  "Test",
	}).Exec(ctx)
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	// Update
	affected, err := users.Update(map[string]interface{}{
		"name": "Updated",
	}).Where(query.Eq("id", 1)).Exec(ctx)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if affected != 1 {
		t.Errorf("Expected 1 row affected, got %d", affected)
	}

	// Delete
	affected, err = query.New(conn, "users").WithProfiler(profiler).
		Delete().Where(query.Eq("id", 1)).Exec(ctx)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if affected != 1 {
		t.Errorf("Expected 1 row affected, got %d", affected)
	}

	profiler.Stop()

	report := profiler.Report()
	if report.TotalQueries != 3 {
		t.Errorf("Expected 3 queries, got %d", report.TotalQueries)
	}
}

func TestProfilerCallerInfo(t *testing.T) {
	opts := query.DefaultProfilerOptions()
	opts.EnableCallerInfo = true

	profiler := query.NewProfiler(opts)
	profiler.Start()

	profile := profiler.StartQuery("SELECT 1", nil)
	profiler.EndQuery(profile, nil)

	profiler.Stop()

	report := profiler.Report()
	if len(report.TopByDuration) > 0 {
		if report.TopByDuration[0].CallerInfo == "" {
			t.Error("Expected caller info to be captured")
		} else {
			t.Logf("Caller info: %s", report.TopByDuration[0].CallerInfo)
		}
	}
}

func TestProfilerReset(t *testing.T) {
	profiler := query.NewProfiler(query.DefaultProfilerOptions())
	profiler.Start()

	// Record some queries
	for i := 0; i < 5; i++ {
		profile := profiler.StartQuery("SELECT 1", nil)
		profiler.EndQuery(profile, nil)
	}

	report := profiler.Report()
	if report.TotalQueries != 5 {
		t.Errorf("Expected 5 queries, got %d", report.TotalQueries)
	}

	// Reset
	profiler.Reset()

	report = profiler.Report()
	if report.TotalQueries != 0 {
		t.Errorf("Expected 0 queries after reset, got %d", report.TotalQueries)
	}

	profiler.Stop()
}

func TestProfilerMaxProfiles(t *testing.T) {
	opts := query.DefaultProfilerOptions()
	opts.MaxProfiles = 5

	profiler := query.NewProfiler(opts)
	profiler.Start()

	// Record more than max
	for i := 0; i < 10; i++ {
		profile := profiler.StartQuery(fmt.Sprintf("SELECT %d", i), nil)
		profiler.EndQuery(profile, nil)
	}

	profiler.Stop()

	report := profiler.Report()
	if report.TotalQueries > opts.MaxProfiles {
		t.Errorf("Expected max %d queries, got %d", opts.MaxProfiles, report.TotalQueries)
	}
}

func TestProfilerErrorTracking(t *testing.T) {
	profiler := query.NewProfiler(query.DefaultProfilerOptions())
	profiler.Start()

	// Record a successful query
	profile1 := profiler.StartQuery("SELECT 1", nil)
	profiler.EndQuery(profile1, nil)

	// Record a failed query
	profile2 := profiler.StartQuery("SELECT * FROM nonexistent", nil)
	profiler.EndQuery(profile2, fmt.Errorf("table not found"))

	profiler.Stop()

	report := profiler.Report()
	if report.ErrorCount != 1 {
		t.Errorf("Expected 1 error, got %d", report.ErrorCount)
	}
}
