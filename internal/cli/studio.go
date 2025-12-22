// Package cli implements the CLI command handlers.
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	_ "github.com/mattn/go-sqlite3"

	"github.com/nexus-db/nexus/internal/studio"
	"github.com/nexus-db/nexus/pkg/core/migration"
	"github.com/nexus-db/nexus/pkg/core/schema"
	"github.com/nexus-db/nexus/pkg/dialects"
	"github.com/nexus-db/nexus/pkg/dialects/sqlite"
)

// StudioOptions configures the studio server.
type StudioOptions struct {
	Port   int
	Host   string
	NoOpen bool
}

// DefaultStudioOptions returns the default studio options.
func DefaultStudioOptions() StudioOptions {
	return StudioOptions{
		Port:   4000,
		Host:   "localhost",
		NoOpen: false,
	}
}

// Studio starts the database browser UI server.
func Studio(opts StudioOptions) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	// Connect to database
	conn, err := connectToDatabase(config)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}

	// Parse schema if available
	var sch *schema.Schema
	if config.Schema.Path != "" {
		sch, err = schema.ParseFile(config.Schema.Path)
		if err != nil {
			// Non-fatal, continue without schema
			fmt.Printf("⚠ Could not parse schema: %v\n", err)
		}
	}

	// Set up migration engine
	migrationEngine := migration.NewEngine(conn)
	if err := migrationEngine.Init(context.Background()); err != nil {
		fmt.Printf("⚠ Could not initialize migrations: %v\n", err)
	}

	// Load migrations from directory
	if err := migrationEngine.LoadFromDir("./migrations"); err != nil {
		// Non-fatal, migrations might not exist yet
	}

	// Create server
	server := studio.NewServer(studio.Config{
		Port:       opts.Port,
		Host:       opts.Host,
		Connection: conn,
		Schema:     sch,
		Migrations: migrationEngine,
	})

	// Print startup banner
	printStudioBanner(opts)

	// Open browser
	if !opts.NoOpen {
		go openBrowser(fmt.Sprintf("http://%s:%d", opts.Host, opts.Port))
	}

	// Handle signals for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\n👋 Stopping Nexus Studio...")
		cancel()
	}()

	// Start server
	if err := server.StartWithContext(ctx); err != nil && err.Error() != "http: Server closed" {
		return err
	}

	return nil
}

// connectToDatabase establishes a database connection based on config.
func connectToDatabase(config *Config) (*dialects.Connection, error) {
	var db *sql.DB
	var dialect dialects.Dialect
	var err error

	switch config.Database.Dialect {
	case "sqlite", "sqlite3":
		// Parse SQLite URL (file:./path or just ./path)
		dsn := config.Database.URL
		if len(dsn) > 5 && dsn[:5] == "file:" {
			dsn = dsn[5:]
		}

		db, err = sql.Open("sqlite3", dsn)
		if err != nil {
			return nil, err
		}
		dialect = sqlite.New()

	case "postgres", "postgresql":
		return nil, fmt.Errorf("PostgreSQL support requires additional driver. Install github.com/lib/pq")

	case "mysql":
		return nil, fmt.Errorf("MySQL support requires additional driver. Install github.com/go-sql-driver/mysql")

	default:
		return nil, fmt.Errorf("unsupported dialect: %s", config.Database.Dialect)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	return dialects.NewConnection(db, dialect), nil
}

// printStudioBanner prints the startup banner.
func printStudioBanner(opts StudioOptions) {
	fmt.Println()
	fmt.Println("🔷 Nexus Studio")
	fmt.Println()
	fmt.Printf("   Local:   http://%s:%d\n", opts.Host, opts.Port)
	fmt.Println()
	fmt.Println("   Press Ctrl+C to stop")
	fmt.Println()
}

// openBrowser opens the default browser to the given URL.
func openBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		return
	}

	cmd.Start()
}
