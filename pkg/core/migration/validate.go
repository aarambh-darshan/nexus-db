// Package migration provides database migration functionality.
package migration

import (
	"regexp"
	"strings"
)

// ValidationSeverity indicates how serious a validation finding is.
type ValidationSeverity int

const (
	SeverityError   ValidationSeverity = iota // Fatal - blocks execution
	SeverityWarning                           // Non-fatal but concerning
)

// ValidationIssue represents a single validation finding.
type ValidationIssue struct {
	Severity   ValidationSeverity
	Message    string
	Line       int    // Optional line number
	Suggestion string // Optional fix suggestion
}

// ValidationResult contains all validation findings for a migration.
type ValidationResult struct {
	MigrationID string
	Issues      []ValidationIssue
	Valid       bool // True if no errors (warnings are OK)
}

// HasErrors returns true if there are any error-level issues.
func (r *ValidationResult) HasErrors() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			return true
		}
	}
	return false
}

// HasWarnings returns true if there are any warning-level issues.
func (r *ValidationResult) HasWarnings() bool {
	for _, issue := range r.Issues {
		if issue.Severity == SeverityWarning {
			return true
		}
	}
	return false
}

// Errors returns only error-level issues.
func (r *ValidationResult) Errors() []ValidationIssue {
	var errors []ValidationIssue
	for _, issue := range r.Issues {
		if issue.Severity == SeverityError {
			errors = append(errors, issue)
		}
	}
	return errors
}

// Warnings returns only warning-level issues.
func (r *ValidationResult) Warnings() []ValidationIssue {
	var warnings []ValidationIssue
	for _, issue := range r.Issues {
		if issue.Severity == SeverityWarning {
			warnings = append(warnings, issue)
		}
	}
	return warnings
}

// Validate checks a migration for issues.
func Validate(m *Migration) *ValidationResult {
	result := &ValidationResult{
		MigrationID: m.ID,
		Valid:       true,
	}

	// Validate UP SQL
	upIssues := ValidateSQL(m.UpSQL, "UP")
	result.Issues = append(result.Issues, upIssues...)

	// Validate DOWN SQL (less strict - can be empty for irreversible migrations)
	if m.DownSQL != "" {
		downIssues := ValidateSQL(m.DownSQL, "DOWN")
		result.Issues = append(result.Issues, downIssues...)
	}

	// Set valid based on errors
	result.Valid = !result.HasErrors()

	return result
}

// ValidateSQL checks raw SQL for issues.
func ValidateSQL(sql string, section string) []ValidationIssue {
	var issues []ValidationIssue

	// Check for empty SQL
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" && section == "UP" {
		issues = append(issues, ValidationIssue{
			Severity:   SeverityError,
			Message:    section + " SQL is empty",
			Suggestion: "Add SQL statements to the " + section + " section",
		})
		return issues
	}

	// Check for unbalanced quotes
	if !areQuotesBalanced(sql) {
		issues = append(issues, ValidationIssue{
			Severity:   SeverityError,
			Message:    "Unbalanced quotes in " + section + " SQL",
			Suggestion: "Check for missing closing quotes",
		})
	}

	// Check for unbalanced parentheses
	if !areParenthesesBalanced(sql) {
		issues = append(issues, ValidationIssue{
			Severity:   SeverityError,
			Message:    "Unbalanced parentheses in " + section + " SQL",
			Suggestion: "Check for missing closing parentheses",
		})
	}

	// Check for dangerous operations
	dangerousPatterns := []struct {
		pattern    string
		message    string
		suggestion string
	}{
		{
			pattern:    `(?i)\bDROP\s+DATABASE\b`,
			message:    "DROP DATABASE detected in " + section,
			suggestion: "This will delete the entire database. Are you sure?",
		},
		{
			pattern:    `(?i)\bDROP\s+SCHEMA\b`,
			message:    "DROP SCHEMA detected in " + section,
			suggestion: "This will delete an entire schema. Are you sure?",
		},
		{
			pattern:    `(?i)\bTRUNCATE\s+TABLE\b`,
			message:    "TRUNCATE TABLE detected in " + section,
			suggestion: "This will delete all data in the table. Are you sure?",
		},
		{
			pattern:    `(?i)\bDELETE\s+FROM\s+\w+\s*(?:;|$)`,
			message:    "DELETE without WHERE clause in " + section,
			suggestion: "This will delete all rows. Add a WHERE clause if unintended.",
		},
		{
			pattern:    `(?i)\bUPDATE\s+\w+\s+SET\s+[^;]+(?:;|$)`,
			message:    "UPDATE may be missing WHERE clause in " + section,
			suggestion: "Verify this UPDATE has the intended scope.",
		},
	}

	for _, dp := range dangerousPatterns {
		re := regexp.MustCompile(dp.pattern)
		if re.MatchString(sql) {
			// Special check for UPDATE - see if it has WHERE
			if strings.Contains(dp.pattern, "UPDATE") {
				if hasWhereClause(sql, "UPDATE") {
					continue
				}
			}
			issues = append(issues, ValidationIssue{
				Severity:   SeverityWarning,
				Message:    dp.message,
				Suggestion: dp.suggestion,
			})
		}
	}

	// Check for DROP TABLE (just a reminder, not dangerous)
	dropTableRe := regexp.MustCompile(`(?i)\bDROP\s+TABLE\b`)
	if dropTableRe.MatchString(sql) {
		issues = append(issues, ValidationIssue{
			Severity:   SeverityWarning,
			Message:    "DROP TABLE detected in " + section,
			Suggestion: "Ensure you have a backup or the DOWN migration recreates the table.",
		})
	}

	return issues
}

// areQuotesBalanced checks if single and double quotes are balanced.
func areQuotesBalanced(sql string) bool {
	singleQuotes := 0
	doubleQuotes := 0
	escaped := false

	for i, r := range sql {
		if escaped {
			escaped = false
			continue
		}

		if r == '\\' {
			escaped = true
			continue
		}

		// Handle escaped quotes ('' or "")
		if r == '\'' {
			if i+1 < len(sql) && sql[i+1] == '\'' {
				// Escaped single quote
				continue
			}
			singleQuotes++
		} else if r == '"' {
			if i+1 < len(sql) && sql[i+1] == '"' {
				// Escaped double quote
				continue
			}
			doubleQuotes++
		}
	}

	return singleQuotes%2 == 0 && doubleQuotes%2 == 0
}

// areParenthesesBalanced checks if parentheses are balanced.
func areParenthesesBalanced(sql string) bool {
	count := 0
	inString := false
	stringChar := rune(0)

	for _, r := range sql {
		// Track string state
		if r == '\'' || r == '"' {
			if !inString {
				inString = true
				stringChar = r
			} else if r == stringChar {
				inString = false
			}
			continue
		}

		if inString {
			continue
		}

		if r == '(' {
			count++
		} else if r == ')' {
			count--
			if count < 0 {
				return false
			}
		}
	}

	return count == 0
}

// hasWhereClause checks if an UPDATE statement has a WHERE clause.
func hasWhereClause(sql, stmtType string) bool {
	// Simple check - look for WHERE after UPDATE/DELETE
	upper := strings.ToUpper(sql)
	idx := strings.Index(upper, stmtType)
	if idx == -1 {
		return true
	}
	remainder := upper[idx:]
	return strings.Contains(remainder, "WHERE")
}

// ValidateMigrations validates multiple migrations.
func ValidateMigrations(migrations []*Migration) []*ValidationResult {
	var results []*ValidationResult
	for _, m := range migrations {
		results = append(results, Validate(m))
	}
	return results
}
