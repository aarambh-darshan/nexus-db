// Package schema provides a DSL parser for .nexus schema files.
package schema

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	nxerr "github.com/nexus-db/nexus/pkg/errors"
)

// Parser parses .nexus schema files.
type Parser struct {
	input  string
	lines  []string // All lines for context
	pos    int
	line   int
	col    int
	errors []*nxerr.NexusError
}

// NewParser creates a new parser for the given input.
func NewParser(input string) *Parser {
	return &Parser{
		input: input,
		line:  1,
		col:   1,
	}
}

// Parse parses the input and returns a Schema.
func (p *Parser) Parse() (*Schema, error) {
	schema := NewSchema()

	scanner := bufio.NewScanner(strings.NewReader(p.input))
	var currentModel *Model
	var inModel bool

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		p.line++

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Model definition start
		if strings.HasPrefix(line, "model ") {
			name := p.parseModelName(line)
			if name == "" {
				p.addError(nxerr.ErrSchemaInvalidModel, "Invalid model definition", line).
					WithSuggestion("Use format: model ModelName {")
				continue
			}
			currentModel = &Model{
				Name:   name,
				Fields: make(map[string]*Field),
			}
			inModel = true
			continue
		}

		// Model definition end
		if line == "}" && inModel {
			schema.Models[currentModel.Name] = currentModel
			schema.modelList = append(schema.modelList, currentModel)
			currentModel = nil
			inModel = false
			continue
		}

		// Field definition inside model
		if inModel && currentModel != nil {
			field, nxErr := p.parseField(line)
			if nxErr != nil {
				p.errors = append(p.errors, nxErr)
				continue
			}
			if field != nil {
				field.Model = currentModel
				currentModel.Fields[field.Name] = field
				currentModel.fieldList = append(currentModel.fieldList, field)
			}
		}
	}

	if len(p.errors) > 0 {
		return nil, p.formatErrors()
	}

	return schema, nil
}

func (p *Parser) parseModelName(line string) string {
	// model User { or model User{
	re := regexp.MustCompile(`^model\s+(\w+)\s*\{?$`)
	matches := re.FindStringSubmatch(line)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func (p *Parser) parseField(line string) (*Field, *nxerr.NexusError) {
	// Skip closing brace or empty
	if line == "}" || line == "{" {
		return nil, nil
	}

	// Parse: fieldName Type @modifier1 @modifier2(arg)
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil, p.makeError(nxerr.ErrSchemaInvalidField, "Invalid field definition", line).
			WithSuggestion("Use format: fieldName Type @modifier")
	}

	fieldName := parts[0]
	fieldType := parts[1]

	// Handle nullable types (Type?)
	nullable := strings.HasSuffix(fieldType, "?")
	if nullable {
		fieldType = strings.TrimSuffix(fieldType, "?")
	}

	// Handle array types (Type[])
	isArray := strings.HasSuffix(fieldType, "[]")
	if isArray {
		fieldType = strings.TrimSuffix(fieldType, "[]")
		// Array types represent relations, skip for now
		return nil, nil
	}

	// Validate type and get suggestions for unknown types
	parsedType, typeErr := p.parseFieldTypeWithValidation(fieldType, line)
	if typeErr != nil {
		return nil, typeErr
	}

	field := &Field{
		Name:     fieldName,
		Type:     parsedType,
		Nullable: nullable,
	}

	// Parse modifiers
	for i := 2; i < len(parts); i++ {
		modifier := parts[i]
		if err := p.applyModifier(field, modifier); err != nil {
			return nil, p.makeError(nxerr.ErrSchemaInvalidModifier, err.Error(), line).
				WithSuggestion(nxerr.Suggestions[nxerr.ErrSchemaInvalidModifier])
		}
	}

	return field, nil
}

func (p *Parser) parseFieldTypeWithValidation(typeName, context string) (FieldType, *nxerr.NexusError) {
	switch strings.ToLower(typeName) {
	case "int", "integer":
		return FieldTypeInt, nil
	case "bigint":
		return FieldTypeBigInt, nil
	case "string", "varchar":
		return FieldTypeString, nil
	case "text":
		return FieldTypeText, nil
	case "bool", "boolean":
		return FieldTypeBool, nil
	case "float", "double":
		return FieldTypeFloat, nil
	case "decimal", "numeric":
		return FieldTypeDecimal, nil
	case "datetime", "timestamp":
		return FieldTypeDateTime, nil
	case "date":
		return FieldTypeDate, nil
	case "time":
		return FieldTypeTime, nil
	case "json", "jsonb":
		return FieldTypeJSON, nil
	case "bytes", "blob", "binary":
		return FieldTypeBytes, nil
	case "uuid":
		return FieldTypeUUID, nil
	default:
		// Check if it looks like a relation (capitalized) - allow it
		if len(typeName) > 0 && typeName[0] >= 'A' && typeName[0] <= 'Z' {
			return FieldTypeString, nil // Treat as relation reference
		}

		// Unknown type - suggest similar
		suggestion := nxerr.SuggestSimilar(typeName, nxerr.ValidTypes)
		if suggestion == "" {
			suggestion = nxerr.Suggestions[nxerr.ErrSchemaUnknownType]
		}
		return FieldTypeString, p.makeError(nxerr.ErrSchemaUnknownType,
			fmt.Sprintf("Unknown type '%s'", typeName), context).WithSuggestion(suggestion)
	}
}

func (p *Parser) parseFieldType(typeName string) FieldType {
	ft, _ := p.parseFieldTypeWithValidation(typeName, "")
	return ft
}

func (p *Parser) applyModifier(field *Field, modifier string) error {
	// Handle modifiers like @id, @unique, @default(value)
	modifier = strings.TrimPrefix(modifier, "@")

	// Check for parentheses (modifier with args)
	if strings.Contains(modifier, "(") {
		name := strings.Split(modifier, "(")[0]
		argPart := strings.TrimSuffix(strings.Split(modifier, "(")[1], ")")

		switch strings.ToLower(name) {
		case "default":
			return p.parseDefault(field, argPart)
		case "db", "map":
			// Column name mapping, ignore for now
		case "relation":
			// Relation config, ignore for now
		case "length", "size":
			if length, err := strconv.Atoi(argPart); err == nil {
				field.Length = length
			}
		case "precision":
			parts := strings.Split(argPart, ",")
			if len(parts) >= 1 {
				if prec, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
					field.Precision = prec
				}
			}
			if len(parts) >= 2 {
				if scale, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
					field.Scale = scale
				}
			}
		}
	} else {
		// Simple modifier without args
		switch strings.ToLower(modifier) {
		case "id":
			field.IsPrimaryKey = true
		case "unique":
			field.IsUnique = true
		case "autoincrement", "auto":
			field.AutoIncrement = true
		}
	}

	return nil
}

func (p *Parser) parseDefault(field *Field, value string) error {
	value = strings.TrimSpace(value)

	// Check for function calls
	if strings.HasSuffix(value, "()") {
		funcName := strings.TrimSuffix(value, "()")
		switch strings.ToLower(funcName) {
		case "now", "current_timestamp":
			field.DefaultExpr = "NOW()"
		case "uuid", "gen_random_uuid":
			field.DefaultExpr = "UUID()"
		default:
			field.DefaultExpr = value
		}
		return nil
	}

	// String literal
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		field.DefaultValue = strings.Trim(value, "\"")
		return nil
	}
	if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") {
		field.DefaultValue = strings.Trim(value, "'")
		return nil
	}

	// Boolean
	if value == "true" {
		field.DefaultValue = true
		return nil
	}
	if value == "false" {
		field.DefaultValue = false
		return nil
	}

	// Number
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		field.DefaultValue = i
		return nil
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		field.DefaultValue = f
		return nil
	}

	field.DefaultValue = value
	return nil
}

// ParseFile parses a .nexus file from the given path.
func ParseFile(path string) (*Schema, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return NewParser(string(content)).Parse()
}

// Helper methods for structured errors

func (p *Parser) addError(code nxerr.ErrorCode, message, context string) *nxerr.NexusError {
	err := p.makeError(code, message, context)
	p.errors = append(p.errors, err)
	return err
}

func (p *Parser) makeError(code nxerr.ErrorCode, message, context string) *nxerr.NexusError {
	return &nxerr.NexusError{
		Code:    code,
		Message: message,
		Line:    p.line,
		Context: context,
	}
}

func (p *Parser) formatErrors() error {
	if len(p.errors) == 0 {
		return nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Schema parsing failed with %d error(s):\n\n", len(p.errors)))

	for _, e := range p.errors {
		sb.WriteString(e.Print())
		sb.WriteString("\n")
	}

	return fmt.Errorf(sb.String())
}
