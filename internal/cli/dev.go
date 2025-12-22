// Package cli implements the CLI command handlers.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/nexus-db/nexus/internal/codegen"
	"github.com/nexus-db/nexus/pkg/core/schema"
)

// DevOptions configures the dev mode behavior.
type DevOptions struct {
	NoGen    bool          // Disable automatic code generation
	Poll     bool          // Use polling instead of OS events
	Interval time.Duration // Debounce/poll interval
}

// DefaultDevOptions returns the default dev mode options.
func DefaultDevOptions() DevOptions {
	return DevOptions{
		NoGen:    false,
		Poll:     false,
		Interval: 500 * time.Millisecond,
	}
}

// Dev runs the development mode with file watching.
func Dev(opts DevOptions) error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	// Resolve schema path
	schemaPath := config.Schema.Path
	absSchemaPath, err := filepath.Abs(schemaPath)
	if err != nil {
		return fmt.Errorf("resolving schema path: %w", err)
	}

	// Check schema file exists
	if _, err := os.Stat(absSchemaPath); os.IsNotExist(err) {
		return fmt.Errorf("schema file not found: %s", schemaPath)
	}

	// Print startup banner
	printDevBanner(schemaPath, config.Output.Dir)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\n👋 Stopping dev mode...")
		cancel()
	}()

	// Run initial generation
	if !opts.NoGen {
		if err := runGeneration(config); err != nil {
			fmt.Printf("[%s] ❌ Error: %v\n", timestamp(), err)
		}
	}

	fmt.Printf("[%s] Watching for changes...\n", timestamp())

	// Start watching
	if opts.Poll {
		return watchWithPolling(ctx, absSchemaPath, config, opts)
	}
	return watchWithFsnotify(ctx, absSchemaPath, config, opts)
}

// watchWithFsnotify uses OS-level file system events.
func watchWithFsnotify(ctx context.Context, schemaPath string, config *Config, opts DevOptions) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("creating watcher: %w", err)
	}
	defer watcher.Close()

	// Watch the directory containing the schema file
	schemaDir := filepath.Dir(schemaPath)
	if err := watcher.Add(schemaDir); err != nil {
		return fmt.Errorf("watching directory: %w", err)
	}

	// Also watch the config file
	configPath, _ := filepath.Abs(configFileName)
	configDir := filepath.Dir(configPath)
	if configDir != schemaDir {
		if err := watcher.Add(configDir); err != nil {
			// Non-fatal, just skip config watching
			fmt.Printf("[%s] ⚠ Could not watch config file\n", timestamp())
		}
	}

	// Debouncer
	var debounceTimer *time.Timer
	var debounceMu sync.Mutex

	for {
		select {
		case <-ctx.Done():
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Only react to write/create events on relevant files
			if !isRelevantEvent(event, schemaPath, configPath) {
				continue
			}

			// Debounce rapid changes
			debounceMu.Lock()
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(opts.Interval, func() {
				handleChange(event.Name, config, opts)
			})
			debounceMu.Unlock()

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Printf("[%s] ⚠ Watcher error: %v\n", timestamp(), err)
		}
	}
}

// watchWithPolling uses file modification time polling.
func watchWithPolling(ctx context.Context, schemaPath string, config *Config, opts DevOptions) error {
	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()

	// Track last modification times
	lastMod := make(map[string]time.Time)
	files := []string{schemaPath}

	// Add config file to watch list
	configPath, _ := filepath.Abs(configFileName)
	if configPath != "" {
		files = append(files, configPath)
	}

	// Initialize last modification times
	for _, file := range files {
		if info, err := os.Stat(file); err == nil {
			lastMod[file] = info.ModTime()
		}
	}

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-ticker.C:
			for _, file := range files {
				info, err := os.Stat(file)
				if err != nil {
					continue
				}

				if !info.ModTime().Equal(lastMod[file]) {
					lastMod[file] = info.ModTime()
					handleChange(file, config, opts)
				}
			}
		}
	}
}

// isRelevantEvent checks if the file system event is relevant.
func isRelevantEvent(event fsnotify.Event, schemaPath, configPath string) bool {
	// Only care about write and create operations
	if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
		return false
	}

	// Check if it's a relevant file
	absPath, _ := filepath.Abs(event.Name)
	if absPath == schemaPath {
		return true
	}
	if absPath == configPath {
		return true
	}

	// Check for .nexus extension
	if filepath.Ext(event.Name) == ".nexus" {
		return true
	}

	return false
}

// handleChange processes a file change event.
func handleChange(filename string, config *Config, opts DevOptions) {
	basename := filepath.Base(filename)
	fmt.Printf("[%s] Change detected: %s\n", timestamp(), basename)

	if opts.NoGen {
		fmt.Printf("[%s] ⏭ Generation disabled (--no-gen)\n", timestamp())
		fmt.Printf("[%s] Watching for changes...\n", timestamp())
		return
	}

	if err := runGeneration(config); err != nil {
		fmt.Printf("[%s] ❌ Error: %v\n", timestamp(), err)
	}

	fmt.Printf("[%s] Watching for changes...\n", timestamp())
}

// runGeneration runs the code generation pipeline.
func runGeneration(config *Config) error {
	// Parse schema
	s, err := schema.ParseFile(config.Schema.Path)
	if err != nil {
		return fmt.Errorf("parsing schema: %w", err)
	}

	// Validate
	if err := s.Validate(); err != nil {
		return fmt.Errorf("validating schema: %w", err)
	}
	fmt.Printf("[%s] ✓ Schema validated\n", timestamp())

	// Generate code
	gen := codegen.NewGenerator(s, config.Output.Package, config.Output.Dir)
	if err := gen.Generate(); err != nil {
		return fmt.Errorf("generating code: %w", err)
	}

	fmt.Printf("[%s] ✓ Generated code in %s/\n", timestamp(), config.Output.Dir)
	fmt.Printf("           - models.go (struct definitions)\n")
	fmt.Printf("           - queries.go (query methods)\n")

	return nil
}

// printDevBanner prints the startup banner.
func printDevBanner(schemaPath, outputDir string) {
	fmt.Println()
	fmt.Println("🚀 Nexus Dev Mode")
	fmt.Printf("   Watching: %s\n", schemaPath)
	fmt.Printf("   Output:   %s/\n", outputDir)
	fmt.Println()
	fmt.Println("   Press Ctrl+C to stop")
	fmt.Println()
}

// timestamp returns the current time formatted for logging.
func timestamp() string {
	return time.Now().Format("15:04:05")
}
