// Package cli implements the CLI command handlers.
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/nexus-db/nexus/pkg/dialects"
	"github.com/nexus-db/nexus/pkg/dialects/sqlite"
	"github.com/nexus-db/nexus/pkg/query"
)

// ProfileOptions configures profiling behavior.
type ProfileOptions struct {
	// Duration auto-stops profiling after this duration (0 = manual stop).
	Duration time.Duration
	// SlowThreshold marks queries slower than this as slow.
	SlowThreshold time.Duration
	// OutputFormat is "text" or "json".
	OutputFormat string
}

// DefaultProfileOptions returns sensible defaults.
func DefaultProfileOptions() ProfileOptions {
	return ProfileOptions{
		Duration:      0,
		SlowThreshold: 100 * time.Millisecond,
		OutputFormat:  "text",
	}
}

// Profile runs an interactive profiling session.
func Profile(opts ProfileOptions) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	// Connect to database
	conn, err := connectToDatabase(config)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer conn.Close()

	// Create profiler
	profilerOpts := query.DefaultProfilerOptions()
	profilerOpts.SlowThreshold = opts.SlowThreshold
	profiler := query.NewProfiler(profilerOpts)

	printProfileBanner(opts)

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start profiling
	profiler.Start()
	startTime := time.Now()

	fmt.Printf("[%s] ▶ Profiling started\n", timestamp())
	fmt.Println("   Press Ctrl+C to stop and view report")
	fmt.Println()

	// Wait for signal or duration
	if opts.Duration > 0 {
		select {
		case <-sigChan:
		case <-time.After(opts.Duration):
			fmt.Printf("\n[%s] ⏱ Duration reached (%s)\n", timestamp(), opts.Duration)
		}
	} else {
		<-sigChan
	}

	// Stop profiling
	profiler.Stop()
	elapsed := time.Since(startTime)

	fmt.Printf("\n[%s] ⏹ Profiling stopped after %s\n", timestamp(), elapsed.Round(time.Millisecond))

	// Generate and display report
	report := profiler.Report()

	if opts.OutputFormat == "json" {
		fmt.Println(reportToJSON(report))
	} else {
		fmt.Println(report.String())
	}

	return nil
}

// ProfileDemo runs a demo profiling session with sample queries.
func ProfileDemo() error {
	fmt.Println("\n🔬 Performance Profiler Demo")
	fmt.Println("   This demo shows how the profiler captures query metrics.")
	fmt.Println()

	// Create in-memory database for demo
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return err
	}
	defer db.Close()

	dialect := sqlite.New()
	conn := dialects.NewConnection(db, dialect)

	// Create profiler
	profiler := query.NewProfiler(query.DefaultProfilerOptions())
	profiler.Start()

	// Create test table
	ctx := context.TODO()
	_, _ = conn.Exec(ctx, `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL,
			name TEXT,
			active INTEGER DEFAULT 1
		)
	`)

	// Create a builder with profiler attached
	users := query.New(conn, "users").WithProfiler(profiler)

	fmt.Println("📝 Running sample queries...")

	// Insert some data
	for i := 0; i < 10; i++ {
		_, _ = users.Insert(map[string]interface{}{
			"email": fmt.Sprintf("user%d@example.com", i),
			"name":  fmt.Sprintf("User %d", i),
		}).Exec(ctx)
	}

	// Run some selects (simulate N+1 pattern)
	for i := 0; i < 8; i++ {
		_, _ = users.Select("id", "name").
			Where(query.Eq("id", i+1)).
			All(ctx)
	}

	// Run a slower query (full table scan)
	_, _ = users.Select().All(ctx)

	// Update
	_, _ = users.Update(map[string]interface{}{
		"active": 0,
	}).Where(query.Eq("id", 1)).Exec(ctx)

	// Delete
	_, _ = query.New(conn, "users").WithProfiler(profiler).
		Delete().Where(query.Eq("id", 10)).Exec(ctx)

	// Stop and report
	profiler.Stop()

	fmt.Println("\n✅ Sample queries completed")
	fmt.Println()

	report := profiler.Report()
	fmt.Println(report.String())

	return nil
}

// printProfileBanner prints the startup banner.
func printProfileBanner(opts ProfileOptions) {
	fmt.Println()
	fmt.Println("🔬 Nexus Performance Profiler")
	fmt.Printf("   Slow threshold: %s\n", opts.SlowThreshold)
	if opts.Duration > 0 {
		fmt.Printf("   Duration: %s\n", opts.Duration)
	}
	fmt.Println()
}

// reportToJSON converts a report to JSON format.
func reportToJSON(report *query.ProfileReport) string {
	// Simple JSON output
	return fmt.Sprintf(`{
  "session_id": "%s",
  "total_queries": %d,
  "total_duration_ms": %.2f,
  "avg_duration_ms": %.2f,
  "slow_queries": %d,
  "errors": %d,
  "n_plus_one_warnings": %d,
  "suggestions": %d
}`,
		report.SessionID,
		report.TotalQueries,
		float64(report.TotalDuration.Microseconds())/1000,
		float64(report.AverageDuration.Microseconds())/1000,
		len(report.SlowQueries),
		report.ErrorCount,
		len(report.NPlusOneWarnings),
		len(report.Suggestions),
	)
}
