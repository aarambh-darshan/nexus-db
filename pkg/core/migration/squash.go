// Package migration provides database migration functionality.
package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// SquashOptions configures the squash operation.
type SquashOptions struct {
	FromID     string // Optional: start from this migration (inclusive)
	ToID       string // Optional: end at this migration (inclusive)
	OutputName string // Name for the squashed migration
}

// SquashResult contains the squashed migration and metadata.
type SquashResult struct {
	Migration      *Migration
	OriginalCount  int
	OptimizedCount int // Number of statements after optimization
	RemovedCount   int // Number of redundant statements removed
	OriginalIDs    []string
}

// SquashMigrations combines multiple migrations into a single optimized migration.
func SquashMigrations(migrations []*Migration, opts SquashOptions) (*SquashResult, error) {
	if len(migrations) == 0 {
		return nil, fmt.Errorf("no migrations to squash")
	}

	// Sort migrations by ID
	sorted := make([]*Migration, len(migrations))
	copy(sorted, migrations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID < sorted[j].ID
	})

	// Filter by range if specified
	filtered := filterMigrationRange(sorted, opts.FromID, opts.ToID)
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no migrations in specified range")
	}

	if len(filtered) == 1 {
		return nil, fmt.Errorf("need at least 2 migrations to squash")
	}

	// Collect all UP statements
	var allUpStatements []string
	var allDownStatements []string
	var originalIDs []string

	for _, m := range filtered {
		originalIDs = append(originalIDs, m.ID)

		// Split SQL into individual statements
		upStmts := splitStatements(m.UpSQL)
		allUpStatements = append(allUpStatements, upStmts...)

		// For DOWN, we need to reverse order (LIFO)
		downStmts := splitStatements(m.DownSQL)
		allDownStatements = append(downStmts, allDownStatements...)
	}

	// Optimize statements (remove redundant CREATE/DROP pairs)
	optimizedUp := optimizeStatements(allUpStatements)
	optimizedDown := optimizeStatements(allDownStatements)

	if len(optimizedUp) == 0 {
		return nil, fmt.Errorf("all statements cancelled out - nothing to squash")
	}

	// Generate new migration
	now := time.Now()
	id := now.Format("20060102_150405")

	upSQL := strings.Join(optimizedUp, ";\n\n") + ";"
	downSQL := strings.Join(optimizedDown, ";\n\n") + ";"

	hash := sha256.Sum256([]byte(upSQL))
	checksum := hex.EncodeToString(hash[:])

	name := opts.OutputName
	if name == "" {
		name = "squashed"
	}

	return &SquashResult{
		Migration: &Migration{
			ID:       id,
			Name:     name,
			UpSQL:    upSQL,
			DownSQL:  downSQL,
			Checksum: checksum,
		},
		OriginalCount:  len(filtered),
		OptimizedCount: len(optimizedUp),
		RemovedCount:   len(allUpStatements) - len(optimizedUp),
		OriginalIDs:    originalIDs,
	}, nil
}

// filterMigrationRange filters migrations to only include those in the specified range.
func filterMigrationRange(migrations []*Migration, fromID, toID string) []*Migration {
	if fromID == "" && toID == "" {
		return migrations
	}

	var result []*Migration
	inRange := fromID == ""

	for _, m := range migrations {
		if fromID != "" && strings.HasPrefix(m.ID, fromID) {
			inRange = true
		}

		if inRange {
			result = append(result, m)
		}

		if toID != "" && strings.HasPrefix(m.ID, toID) {
			break
		}
	}

	return result
}

// splitStatements splits SQL into individual statements.
func splitStatements(sql string) []string {
	// Remove comments
	sql = removeComments(sql)

	// Split by semicolon, but be careful with strings
	var statements []string
	var current strings.Builder
	inString := false
	stringChar := rune(0)

	for _, r := range sql {
		switch {
		case r == '\'' || r == '"':
			if !inString {
				inString = true
				stringChar = r
			} else if r == stringChar {
				inString = false
			}
			current.WriteRune(r)
		case r == ';' && !inString:
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}

	// Don't forget the last statement
	if stmt := strings.TrimSpace(current.String()); stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}

// removeComments removes SQL comments from the input.
func removeComments(sql string) string {
	// Remove single-line comments
	re := regexp.MustCompile(`--[^\n]*`)
	sql = re.ReplaceAllString(sql, "")

	// Remove multi-line comments
	re = regexp.MustCompile(`/\*[\s\S]*?\*/`)
	sql = re.ReplaceAllString(sql, "")

	return sql
}

// optimizeStatements removes redundant operations.
func optimizeStatements(statements []string) []string {
	// Track table operations
	tableCreates := make(map[string]int) // table name -> statement index
	tableDrops := make(map[string]int)   // table name -> statement index
	columnAdds := make(map[string]int)   // table.column -> statement index
	columnDrops := make(map[string]int)  // table.column -> statement index
	indexCreates := make(map[string]int) // index name -> statement index
	indexDrops := make(map[string]int)   // index name -> statement index

	// First pass: identify all operations
	for i, stmt := range statements {
		stmtUpper := strings.ToUpper(stmt)

		if tableName := extractCreateTable(stmtUpper); tableName != "" {
			tableCreates[tableName] = i
		} else if tableName := extractDropTable(stmtUpper); tableName != "" {
			tableDrops[tableName] = i
		} else if key := extractAddColumn(stmtUpper); key != "" {
			columnAdds[key] = i
		} else if key := extractDropColumn(stmtUpper); key != "" {
			columnDrops[key] = i
		} else if indexName := extractCreateIndex(stmtUpper); indexName != "" {
			indexCreates[indexName] = i
		} else if indexName := extractDropIndex(stmtUpper); indexName != "" {
			indexDrops[indexName] = i
		}
	}

	// Second pass: mark statements to skip
	skip := make(map[int]bool)

	// Remove CREATE TABLE + DROP TABLE pairs
	for table, createIdx := range tableCreates {
		if dropIdx, exists := tableDrops[table]; exists && dropIdx > createIdx {
			skip[createIdx] = true
			skip[dropIdx] = true
		}
	}

	// Remove ADD COLUMN + DROP COLUMN pairs
	for col, addIdx := range columnAdds {
		if dropIdx, exists := columnDrops[col]; exists && dropIdx > addIdx {
			skip[addIdx] = true
			skip[dropIdx] = true
		}
	}

	// Remove CREATE INDEX + DROP INDEX pairs
	for idx, createIdx := range indexCreates {
		if dropIdx, exists := indexDrops[idx]; exists && dropIdx > createIdx {
			skip[createIdx] = true
			skip[dropIdx] = true
		}
	}

	// Build result, skipping redundant statements
	var result []string
	for i, stmt := range statements {
		if !skip[i] {
			result = append(result, stmt)
		}
	}

	return result
}

// extractCreateTable extracts table name from CREATE TABLE statement.
func extractCreateTable(stmt string) string {
	re := regexp.MustCompile(`CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?["'\x60]?(\w+)["'\x60]?`)
	matches := re.FindStringSubmatch(stmt)
	if len(matches) >= 2 {
		return strings.ToUpper(matches[1])
	}
	return ""
}

// extractDropTable extracts table name from DROP TABLE statement.
func extractDropTable(stmt string) string {
	re := regexp.MustCompile(`DROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?["'\x60]?(\w+)["'\x60]?`)
	matches := re.FindStringSubmatch(stmt)
	if len(matches) >= 2 {
		return strings.ToUpper(matches[1])
	}
	return ""
}

// extractAddColumn extracts table.column from ADD COLUMN statement.
func extractAddColumn(stmt string) string {
	re := regexp.MustCompile(`ALTER\s+TABLE\s+["'\x60]?(\w+)["'\x60]?\s+ADD\s+(?:COLUMN\s+)?["'\x60]?(\w+)["'\x60]?`)
	matches := re.FindStringSubmatch(stmt)
	if len(matches) >= 3 {
		return strings.ToUpper(matches[1] + "." + matches[2])
	}
	return ""
}

// extractDropColumn extracts table.column from DROP COLUMN statement.
func extractDropColumn(stmt string) string {
	re := regexp.MustCompile(`ALTER\s+TABLE\s+["'\x60]?(\w+)["'\x60]?\s+DROP\s+(?:COLUMN\s+)?["'\x60]?(\w+)["'\x60]?`)
	matches := re.FindStringSubmatch(stmt)
	if len(matches) >= 3 {
		return strings.ToUpper(matches[1] + "." + matches[2])
	}
	return ""
}

// extractCreateIndex extracts index name from CREATE INDEX statement.
func extractCreateIndex(stmt string) string {
	re := regexp.MustCompile(`CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?["'\x60]?(\w+)["'\x60]?`)
	matches := re.FindStringSubmatch(stmt)
	if len(matches) >= 2 {
		return strings.ToUpper(matches[1])
	}
	return ""
}

// extractDropIndex extracts index name from DROP INDEX statement.
func extractDropIndex(stmt string) string {
	re := regexp.MustCompile(`DROP\s+INDEX\s+(?:IF\s+EXISTS\s+)?["'\x60]?(\w+)["'\x60]?`)
	matches := re.FindStringSubmatch(stmt)
	if len(matches) >= 2 {
		return strings.ToUpper(matches[1])
	}
	return ""
}
