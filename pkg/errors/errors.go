// Package errors provides structured error types with helpful suggestions.
package errors

import (
	"fmt"
	"strings"
)

// ErrorCode represents the category of error.
type ErrorCode string

const (
	// Schema errors
	ErrSchemaInvalidModel    ErrorCode = "SCHEMA_INVALID_MODEL"
	ErrSchemaInvalidField    ErrorCode = "SCHEMA_INVALID_FIELD"
	ErrSchemaUnknownType     ErrorCode = "SCHEMA_UNKNOWN_TYPE"
	ErrSchemaInvalidModifier ErrorCode = "SCHEMA_INVALID_MODIFIER"
	ErrSchemaMissingPK       ErrorCode = "SCHEMA_MISSING_PRIMARY_KEY"
	ErrSchemaDuplicateField  ErrorCode = "SCHEMA_DUPLICATE_FIELD"
	ErrSchemaValidation      ErrorCode = "SCHEMA_VALIDATION"

	// Migration errors
	ErrMigrationNotFound      ErrorCode = "MIGRATION_NOT_FOUND"
	ErrMigrationNoChanges     ErrorCode = "MIGRATION_NO_CHANGES"
	ErrMigrationLocked        ErrorCode = "MIGRATION_LOCKED"
	ErrMigrationNoRollback    ErrorCode = "MIGRATION_NO_ROLLBACK"
	ErrMigrationInvalidFormat ErrorCode = "MIGRATION_INVALID_FORMAT"
	ErrMigrationApplyFailed   ErrorCode = "MIGRATION_APPLY_FAILED"

	// Query errors
	ErrQueryDialectUnsupported ErrorCode = "QUERY_DIALECT_UNSUPPORTED"
	ErrQueryCascadeRestrict    ErrorCode = "QUERY_CASCADE_RESTRICT"

	// General errors
	ErrGeneral ErrorCode = "GENERAL_ERROR"
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// NexusError is a structured error with location and suggestion.
type NexusError struct {
	Code       ErrorCode
	Message    string
	Suggestion string
	Line       int
	Column     int
	Context    string // The line of code with the error
}

// Error implements the error interface.
func (e *NexusError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("[%s] line %d: %s", e.Code, e.Line, e.Message)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Print outputs the error in a user-friendly colored format.
func (e *NexusError) Print() string {
	var sb strings.Builder

	// Error header
	sb.WriteString(fmt.Sprintf("%s%sError:%s %s\n", colorBold, colorRed, colorReset, e.Message))

	// Location with context
	if e.Line > 0 && e.Context != "" {
		sb.WriteString(fmt.Sprintf("\n  %s%d |%s  %s\n", colorGray, e.Line, colorReset, e.Context))

		// Underline the error position if column is set
		if e.Column > 0 {
			padding := strings.Repeat(" ", len(fmt.Sprintf("%d", e.Line))+4+e.Column-1)
			sb.WriteString(fmt.Sprintf("%s%s^%s\n", colorRed, padding, colorReset))
		}
	} else if e.Line > 0 {
		sb.WriteString(fmt.Sprintf("\n  %sAt line %d%s\n", colorGray, e.Line, colorReset))
	}

	// Suggestion
	if e.Suggestion != "" {
		sb.WriteString(fmt.Sprintf("\n%sSuggestion:%s %s\n", colorCyan, colorReset, e.Suggestion))
	}

	return sb.String()
}

// NewSchemaError creates a schema-related error.
func NewSchemaError(code ErrorCode, message string, line int, context string) *NexusError {
	return &NexusError{
		Code:    code,
		Message: message,
		Line:    line,
		Context: context,
	}
}

// WithSuggestion adds a suggestion to the error.
func (e *NexusError) WithSuggestion(suggestion string) *NexusError {
	e.Suggestion = suggestion
	return e
}

// WithColumn sets the column for precise error location.
func (e *NexusError) WithColumn(col int) *NexusError {
	e.Column = col
	return e
}

// NewMigrationError creates a migration-related error.
func NewMigrationError(code ErrorCode, message string) *NexusError {
	return &NexusError{
		Code:    code,
		Message: message,
	}
}

// NewQueryError creates a query-related error.
func NewQueryError(code ErrorCode, message string) *NexusError {
	return &NexusError{
		Code:    code,
		Message: message,
	}
}

// Suggestions provides common suggestion messages.
var Suggestions = map[ErrorCode]string{
	ErrSchemaUnknownType:       "Valid types: Int, BigInt, String, Text, Bool, Float, Decimal, DateTime, Date, Time, JSON, Bytes, UUID",
	ErrSchemaInvalidModifier:   "Valid modifiers: @id, @unique, @autoincrement, @default(value)",
	ErrSchemaMissingPK:         "Add a primary key field with @id modifier, e.g.: id Int @id @autoincrement",
	ErrMigrationNoRollback:     "Run 'nexus migrate up' first to apply migrations",
	ErrMigrationLocked:         "Use 'nexus migrate --force' to override the lock (use with caution)",
	ErrMigrationNoChanges:      "Your schema matches the database. No migration needed.",
	ErrMigrationNotFound:       "Check the migrations/ directory exists and contains .sql files",
	ErrQueryDialectUnsupported: "This feature is not supported by your database dialect",
}

// SuggestSimilar finds similar strings using Levenshtein distance.
func SuggestSimilar(input string, options []string) string {
	input = strings.ToLower(input)
	var best string
	bestDist := len(input) + 1

	for _, opt := range options {
		dist := levenshtein(input, strings.ToLower(opt))
		if dist < bestDist && dist <= 3 { // Only suggest if close enough
			bestDist = dist
			best = opt
		}
	}

	if best != "" {
		return fmt.Sprintf("Did you mean '%s'?", best)
	}
	return ""
}

// levenshtein calculates the edit distance between two strings.
func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(a)][len(b)]
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// ValidTypes lists all valid schema field types.
var ValidTypes = []string{
	"Int", "BigInt", "String", "Text", "Bool", "Float",
	"Decimal", "DateTime", "Date", "Time", "JSON", "Bytes", "UUID",
}

// ValidModifiers lists all valid field modifiers.
var ValidModifiers = []string{
	"id", "unique", "autoincrement", "auto", "default", "db", "map", "relation", "length", "size", "precision",
}
