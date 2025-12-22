package main

import (
	"fmt"
	"os"

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
	cmd.AddCommand(&cobra.Command{
		Use:   "up",
		Short: "Apply pending migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.MigrateUp()
		},
	})

	// migrate down
	cmd.AddCommand(&cobra.Command{
		Use:   "down",
		Short: "Rollback the last migration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.MigrateDown()
		},
	})

	// migrate status
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show migration status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.MigrateStatus()
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
	return &cobra.Command{
		Use:   "dev",
		Short: "Run in development mode (watch + auto-generate)",
		Long:  "Watches schema files and auto-generates code on changes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Development mode is not yet implemented.")
			fmt.Println("For now, run 'nexus gen' manually after schema changes.")
			return nil
		},
	}
}
