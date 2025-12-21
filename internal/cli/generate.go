package cli

import (
	"fmt"

	"github.com/nexus-db/nexus/internal/codegen"
	"github.com/nexus-db/nexus/pkg/core/schema"
)

// Generate generates Go code from the schema.
func Generate() error {
	config, err := LoadConfig()
	if err != nil {
		return err
	}

	// Parse schema
	s, err := schema.ParseFile(config.Schema.Path)
	if err != nil {
		return fmt.Errorf("parsing schema: %w", err)
	}

	// Validate
	if err := s.Validate(); err != nil {
		return fmt.Errorf("validating schema: %w", err)
	}

	// Generate code
	gen := codegen.NewGenerator(s, config.Output.Package, config.Output.Dir)
	if err := gen.Generate(); err != nil {
		return fmt.Errorf("generating code: %w", err)
	}

	fmt.Printf("✓ Generated code in %s/\n", config.Output.Dir)
	fmt.Printf("  - models.go (struct definitions)\n")
	fmt.Printf("  - queries.go (query methods)\n")

	return nil
}
