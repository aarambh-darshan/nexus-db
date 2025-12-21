// Package cli implements the CLI command handlers.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the nexus configuration file.
type Config struct {
	Database DatabaseConfig `json:"database"`
	Schema   SchemaConfig   `json:"schema"`
	Output   OutputConfig   `json:"output"`
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	Dialect string `json:"dialect"` // postgres, sqlite, mysql
	URL     string `json:"url"`     // Connection string
}

// SchemaConfig holds schema file settings.
type SchemaConfig struct {
	Path string `json:"path"` // Path to schema.nexus file
}

// OutputConfig holds code generation output settings.
type OutputConfig struct {
	Dir     string `json:"dir"`     // Output directory for generated code
	Package string `json:"package"` // Go package name
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Dialect: "sqlite",
			URL:     "file:./nexus.db",
		},
		Schema: SchemaConfig{
			Path: "./schema.nexus",
		},
		Output: OutputConfig{
			Dir:     "./generated",
			Package: "db",
		},
	}
}

const configFileName = "nexus.json"

// Init initializes a new Nexus project.
func Init(dir string) error {
	// Create directory if needed
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
	}

	// Check if config already exists
	configPath := filepath.Join(dir, configFileName)
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("project already initialized: %s exists", configFileName)
	}

	// Write default config
	config := DefaultConfig()
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return err
	}
	fmt.Printf("✓ Created %s\n", configPath)

	// Create schema file
	schemaPath := filepath.Join(dir, "schema.nexus")
	schemaContent := `// Nexus Schema File
// Define your models here

model User {
  id        Int       @id @autoincrement
  email     String    @unique
  name      String?
  createdAt DateTime  @default(now())
  updatedAt DateTime  @default(now())
}

// Add more models below...
`
	if err := os.WriteFile(schemaPath, []byte(schemaContent), 0644); err != nil {
		return err
	}
	fmt.Printf("✓ Created %s\n", schemaPath)

	// Create migrations directory
	migrationsDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		return err
	}
	fmt.Printf("✓ Created %s/\n", migrationsDir)

	// Create generated directory
	generatedDir := filepath.Join(dir, "generated")
	if err := os.MkdirAll(generatedDir, 0755); err != nil {
		return err
	}
	fmt.Printf("✓ Created %s/\n", generatedDir)

	fmt.Println("\n🎉 Nexus project initialized!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Edit schema.nexus to define your models")
	fmt.Println("  2. Run 'nexus migrate new init' to create initial migration")
	fmt.Println("  3. Run 'nexus migrate up' to apply migrations")
	fmt.Println("  4. Run 'nexus gen' to generate Go code")

	return nil
}

// LoadConfig loads the configuration from the current directory.
func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(configFileName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("not a Nexus project (nexus.json not found). Run 'nexus init' first")
		}
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &config, nil
}
