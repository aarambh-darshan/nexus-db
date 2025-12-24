package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/nexus-db/nexus/internal/cli"
)

var version = "0.2.0"

func main() {
	rootCmd := &cobra.Command{
		Use:   "nexus",
		Short: "Nexus - Schema-first database framework for Go",
		Long: `Nexus is a Prisma/Drizzle-inspired database toolkit providing:
  • Schema-first migrations with up/down support
  • Type-safe query builder
  • Multi-dialect support (PostgreSQL, SQLite, MySQL)
  • Code generation from schemas`,
		Version: version,
	}

	// Add subcommands
	rootCmd.AddCommand(initCmd())
	rootCmd.AddCommand(migrateCmd())
	rootCmd.AddCommand(seedCmd())
	rootCmd.AddCommand(genCmd())
	rootCmd.AddCommand(devCmd())
	rootCmd.AddCommand(studioCmd())
	rootCmd.AddCommand(profileCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// initCmd creates a new Nexus project
func initCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [directory]",
		Short: "Initialize a new Nexus project",
		Long:  "Creates a new Nexus project with default configuration and schema files.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			return cli.Init(dir)
		},
	}
	return cmd
}

// migrateCmd handles database migrations
func migrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Manage database migrations",
		Long:  "Create, apply, and manage database migrations.",
	}

	// migrate new
	cmd.AddCommand(&cobra.Command{
		Use:   "new <name>",
		Short: "Create a new migration from schema",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.MigrateNew(args[0])
		},
	})

	// migrate up
	upCmd := &cobra.Command{
		Use:   "up",
		Short: "Apply pending migrations",
		Long:  "Apply all pending migrations. Use --force to break stale locks.",
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")
			return cli.MigrateUp(force)
		},
	}
	upCmd.Flags().Bool("force", false, "Force break any stale migration locks")
	cmd.AddCommand(upCmd)

	// migrate down
	downCmd := &cobra.Command{
		Use:   "down",
		Short: "Rollback migrations",
		Long: `Rollback migrations. By default rolls back the last migration.
Use --to to rollback to a specific version (exclusive).
Use -n to rollback a specific number of migrations.
Use --force to break stale locks.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			to, _ := cmd.Flags().GetString("to")
			n, _ := cmd.Flags().GetInt("n")
			force, _ := cmd.Flags().GetBool("force")
			return cli.MigrateDown(to, n, force)
		},
	}
	downCmd.Flags().String("to", "", "Rollback to this migration ID (exclusive)")
	downCmd.Flags().IntP("n", "n", 0, "Number of migrations to rollback")
	downCmd.Flags().Bool("force", false, "Force break any stale migration locks")
	cmd.AddCommand(downCmd)

	// migrate status
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show migration status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.MigrateStatus()
		},
	})

	// migrate validate
	cmd.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate migration SQL files",
		Long:  "Checks all migration files for syntax errors and warns about dangerous operations.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.MigrateValidate()
		},
	})

	// migrate reset
	cmd.AddCommand(&cobra.Command{
		Use:   "reset",
		Short: "Reset database (rollback all, then apply all)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.MigrateReset()
		},
	})

	// migrate diff
	cmd.AddCommand(&cobra.Command{
		Use:   "diff <name>",
		Short: "Auto-generate migration from schema changes",
		Long:  "Compares your schema with the database and generates a migration with the detected changes.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.MigrateDiff(args[0])
		},
	})

	// migrate squash
	squashCmd := &cobra.Command{
		Use:   "squash <name>",
		Short: "Combine multiple migrations into one",
		Long: `Squashes multiple migration files into a single optimized migration.
Redundant operations (like CREATE TABLE followed by DROP TABLE) are removed.
Original migrations are backed up to migrations/.squashed_backup/`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			from, _ := cmd.Flags().GetString("from")
			to, _ := cmd.Flags().GetString("to")
			keepOriginals, _ := cmd.Flags().GetBool("keep-originals")
			return cli.MigrateSquash(args[0], from, to, keepOriginals)
		},
	}
	squashCmd.Flags().String("from", "", "Start from this migration ID (inclusive)")
	squashCmd.Flags().String("to", "", "End at this migration ID (inclusive)")
	squashCmd.Flags().Bool("keep-originals", false, "Keep original migration files (don't move to backup)")
	cmd.AddCommand(squashCmd)

	return cmd
}

// seedCmd handles database seeding
func seedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Manage database seed data",
		Long:  "Load initial or test data into the database from seed files.",
	}

	// seed (run)
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run pending seeds",
		RunE: func(cmd *cobra.Command, args []string) error {
			env, _ := cmd.Flags().GetString("env")
			reset, _ := cmd.Flags().GetBool("reset")
			return cli.SeedRun(env, reset)
		},
	}
	runCmd.Flags().String("env", "", "Environment to run seeds for (dev, test, prod)")
	runCmd.Flags().Bool("reset", false, "Clear seed history and re-run all seeds")
	cmd.AddCommand(runCmd)

	// Make "run" the default action when just "nexus seed" is called
	cmd.RunE = func(c *cobra.Command, args []string) error {
		env, _ := c.Flags().GetString("env")
		reset, _ := c.Flags().GetBool("reset")
		return cli.SeedRun(env, reset)
	}
	cmd.Flags().String("env", "", "Environment to run seeds for (dev, test, prod)")
	cmd.Flags().Bool("reset", false, "Clear seed history and re-run all seeds")

	// seed status
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show seed status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.SeedStatus()
		},
	})

	// seed new
	newCmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a new seed file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, _ := cmd.Flags().GetString("env")
			return cli.SeedCreate(args[0], env)
		},
	}
	newCmd.Flags().String("env", "", "Environment for the seed (dev, test, prod)")
	cmd.AddCommand(newCmd)

	return cmd
}

// genCmd generates code from schema
func genCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gen",
		Short: "Generate Go types from schema",
		Long:  "Parses the schema and generates type-safe Go code.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.Generate()
		},
	}
}

// devCmd runs in development mode
func devCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Run in development mode (watch + auto-generate)",
		Long: `Watches schema files and auto-generates code on changes.

The watcher monitors your schema.nexus file and automatically runs
code generation whenever changes are detected. Use Ctrl+C to stop.

Examples:
  nexus dev                    # Start watching with defaults
  nexus dev --no-gen           # Watch without auto-generation
  nexus dev --poll             # Use polling (for network drives)
  nexus dev --interval 1s      # Set debounce interval`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := cli.DefaultDevOptions()

			noGen, _ := cmd.Flags().GetBool("no-gen")
			poll, _ := cmd.Flags().GetBool("poll")
			interval, _ := cmd.Flags().GetDuration("interval")

			opts.NoGen = noGen
			opts.Poll = poll
			opts.Interval = interval

			return cli.Dev(opts)
		},
	}

	cmd.Flags().Bool("no-gen", false, "Disable automatic code generation")
	cmd.Flags().Bool("poll", false, "Use polling instead of OS events (for network drives)")
	cmd.Flags().Duration("interval", 500*time.Millisecond, "Debounce/poll interval")

	return cmd
}

// studioCmd runs the database browser UI
func studioCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "studio",
		Short: "Open the database browser UI",
		Long: `Starts a local web server with a database browser UI.

The studio provides:
  • Table browser with data viewing
  • SQL query editor
  • Schema visualization
  • Migration status

Examples:
  nexus studio                  # Start on default port 4000
  nexus studio --port 3000      # Use custom port
  nexus studio --no-open        # Don't open browser automatically`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := cli.DefaultStudioOptions()

			port, _ := cmd.Flags().GetInt("port")
			host, _ := cmd.Flags().GetString("host")
			noOpen, _ := cmd.Flags().GetBool("no-open")

			opts.Port = port
			opts.Host = host
			opts.NoOpen = noOpen

			return cli.Studio(opts)
		},
	}

	cmd.Flags().Int("port", 4000, "Port to run the studio server on")
	cmd.Flags().String("host", "localhost", "Host to bind the server to")
	cmd.Flags().Bool("no-open", false, "Don't automatically open browser")

	return cmd
}

// profileCmd runs the performance profiler
func profileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Run the performance profiler",
		Long: `Starts a query profiling session to analyze database performance.

The profiler captures query execution metrics, detects slow queries,
identifies N+1 patterns, and provides optimization suggestions.

Examples:
  nexus profile                    # Run in demo mode with sample queries
  nexus profile --duration 30s     # Profile for 30 seconds
  nexus profile --slow 50ms        # Set slow query threshold to 50ms
  nexus profile --json             # Output report as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			demo, _ := cmd.Flags().GetBool("demo")
			if demo {
				return cli.ProfileDemo()
			}

			opts := cli.DefaultProfileOptions()

			duration, _ := cmd.Flags().GetDuration("duration")
			slow, _ := cmd.Flags().GetDuration("slow")
			jsonOutput, _ := cmd.Flags().GetBool("json")

			opts.Duration = duration
			opts.SlowThreshold = slow
			if jsonOutput {
				opts.OutputFormat = "json"
			}

			return cli.Profile(opts)
		},
	}

	cmd.Flags().Bool("demo", true, "Run in demo mode with sample queries")
	cmd.Flags().Duration("duration", 0, "Auto-stop profiling after this duration")
	cmd.Flags().Duration("slow", 100*time.Millisecond, "Slow query threshold")
	cmd.Flags().Bool("json", false, "Output report as JSON")

	return cmd
}
