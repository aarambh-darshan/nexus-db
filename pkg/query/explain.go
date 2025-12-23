// Package query provides a fluent query builder for database operations.
package query

import (
	"context"
	"regexp"
	"strings"

	"github.com/nexus-db/nexus/pkg/dialects"
)

// ExplainFormat defines the output format for EXPLAIN.
type ExplainFormat string

const (
	// ExplainFormatText is the default text format.
	ExplainFormatText ExplainFormat = "text"
	// ExplainFormatJSON returns plan as JSON.
	ExplainFormatJSON ExplainFormat = "json"
	// ExplainFormatTree is tree format (MySQL 8.0+).
	ExplainFormatTree ExplainFormat = "tree"
	// ExplainFormatYAML is YAML format (PostgreSQL).
	ExplainFormatYAML ExplainFormat = "yaml"
	// ExplainFormatXML is XML format (PostgreSQL).
	ExplainFormatXML ExplainFormat = "xml"
)

// ExplainOptions configures explain behavior.
type ExplainOptions struct {
	// Analyze executes the query and shows actual timings.
	Analyze bool
	// Verbose shows more detailed output.
	Verbose bool
	// Buffers includes buffer usage (PostgreSQL only).
	Buffers bool
	// Format specifies the output format.
	Format ExplainFormat
}

// QueryPlan represents a parsed execution plan.
type QueryPlan struct {
	// Raw is the raw EXPLAIN output.
	Raw string
	// Format is the format used for the plan.
	Format ExplainFormat
	// EstimatedCost is the estimated cost (if available).
	EstimatedCost float64
	// EstimatedRows is the estimated row count.
	EstimatedRows int64
	// ActualTime is the actual execution time in milliseconds (ANALYZE only).
	ActualTime float64
	// UsedIndexes lists indexes used in the plan.
	UsedIndexes []string
	// ScanTypes lists scan types (seq scan, index scan, etc.).
	ScanTypes []string
	// Warnings contains performance warnings/suggestions.
	Warnings []string
	// IsAnalyzed indicates if this was an EXPLAIN ANALYZE.
	IsAnalyzed bool
}

// Explain returns the query plan for the SELECT query.
func (s *SelectBuilder) Explain(ctx context.Context, opts ...ExplainOptions) (*QueryPlan, error) {
	opt := ExplainOptions{Format: ExplainFormatText}
	if len(opts) > 0 {
		opt = opts[0]
	}

	query, args := s.Build()
	return explain(ctx, s.conn, query, args, opt)
}

// Analyze executes the query with EXPLAIN ANALYZE (actual timings).
func (s *SelectBuilder) Analyze(ctx context.Context) (*QueryPlan, error) {
	query, args := s.Build()
	return explain(ctx, s.conn, query, args, ExplainOptions{
		Analyze: true,
		Format:  ExplainFormatText,
	})
}

// explain runs EXPLAIN on a query and parses the result.
func explain(ctx context.Context, conn *dialects.Connection, query string, args []interface{}, opts ExplainOptions) (*QueryPlan, error) {
	dialect := conn.Dialect

	// Build EXPLAIN query
	explainSQL := dialect.ExplainSQL(query, string(opts.Format), opts.Analyze)

	// Execute EXPLAIN
	rows, err := conn.Query(ctx, explainSQL, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Collect output
	var rawLines []string
	columns, _ := rows.Columns()

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// Convert row to string line
		var lineParts []string
		for _, v := range values {
			if v != nil {
				switch val := v.(type) {
				case []byte:
					lineParts = append(lineParts, string(val))
				case string:
					lineParts = append(lineParts, val)
				default:
					lineParts = append(lineParts, stringValue(v))
				}
			}
		}
		rawLines = append(rawLines, strings.Join(lineParts, " | "))
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	raw := strings.Join(rawLines, "\n")

	// Parse the plan
	plan := &QueryPlan{
		Raw:        raw,
		Format:     opts.Format,
		IsAnalyzed: opts.Analyze,
	}

	parsePlan(plan, raw, dialect.Name())

	return plan, nil
}

// stringValue converts an interface to string representation.
func stringValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int64:
		return strings.TrimSpace(strings.Repeat(" ", 10) + string(rune(val)))
	default:
		return ""
	}
}

// parsePlan extracts structured information from the raw plan.
func parsePlan(plan *QueryPlan, raw, dialectName string) {
	lower := strings.ToLower(raw)

	// Extract index usage
	plan.UsedIndexes = extractIndexes(raw, dialectName)

	// Extract scan types
	plan.ScanTypes = extractScanTypes(lower)

	// Generate warnings
	plan.Warnings = generateWarnings(lower, plan.ScanTypes, plan.UsedIndexes)
}

// extractIndexes finds index names used in the plan.
func extractIndexes(raw, dialectName string) []string {
	var indexes []string
	seen := make(map[string]bool)

	// Common patterns for index usage across dialects
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`USING INDEX\s+(\w+)`),            // SQLite
		regexp.MustCompile(`using\s+(\w+)`),                  // SQLite EXPLAIN QUERY PLAN
		regexp.MustCompile(`Index Scan using (\w+)`),         // PostgreSQL
		regexp.MustCompile(`Index Only Scan using (\w+)`),    // PostgreSQL
		regexp.MustCompile(`Bitmap Index Scan on (\w+)`),     // PostgreSQL
		regexp.MustCompile(`key:\s*(\w+)`),                   // MySQL
		regexp.MustCompile(`Using index\s*(\w*)`),            // MySQL
		regexp.MustCompile(`SEARCH .* USING .* INDEX (\w+)`), // SQLite
	}

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(raw, -1)
		for _, match := range matches {
			if len(match) > 1 && match[1] != "" && !seen[match[1]] {
				seen[match[1]] = true
				indexes = append(indexes, match[1])
			}
		}
	}

	return indexes
}

// extractScanTypes identifies scan types in the plan.
func extractScanTypes(lower string) []string {
	var scanTypes []string
	seen := make(map[string]bool)

	scanPatterns := map[string][]string{
		"sequential_scan": {"seq scan", "table scan", "full scan", "scan table"},
		"index_scan":      {"index scan", "using index", "search table"},
		"index_only_scan": {"index only scan", "covering index"},
		"bitmap_scan":     {"bitmap heap scan", "bitmap index scan"},
		"nested_loop":     {"nested loop"},
		"hash_join":       {"hash join"},
		"merge_join":      {"merge join"},
		"sort":            {"sort", "filesort"},
		"aggregate":       {"aggregate", "group"},
	}

	for scanType, patterns := range scanPatterns {
		for _, pattern := range patterns {
			if strings.Contains(lower, pattern) && !seen[scanType] {
				seen[scanType] = true
				scanTypes = append(scanTypes, scanType)
				break
			}
		}
	}

	return scanTypes
}

// generateWarnings creates performance warnings based on the plan.
func generateWarnings(lower string, scanTypes, indexes []string) []string {
	var warnings []string

	// Check for sequential/full table scans
	for _, st := range scanTypes {
		if st == "sequential_scan" {
			warnings = append(warnings, "Full table scan detected - consider adding an index")
		}
	}

	// Check for filesort (MySQL)
	if strings.Contains(lower, "filesort") {
		warnings = append(warnings, "Using filesort - consider adding an index for ORDER BY columns")
	}

	// Check for temporary table
	if strings.Contains(lower, "temporary") || strings.Contains(lower, "using temporary") {
		warnings = append(warnings, "Using temporary table - query may be slow for large datasets")
	}

	// Check for no indexes used
	if len(indexes) == 0 && (strings.Contains(lower, "scan") || strings.Contains(lower, "table")) {
		// Only warn if it looks like a table access without index
		if !strings.Contains(lower, "index") {
			warnings = append(warnings, "No indexes used in query")
		}
	}

	return warnings
}
