// Package schema provides a DSL parser for .nexus schema files.
package schema

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Parser parses .nexus schema files.
type Parser struct {
	input  string
	pos    int
	line   int
	col    int
	errors []error
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
				p.errors = append(p.errors, fmt.Errorf("line %d: invalid model definition", p.line))
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
			field, err := p.parseField(line)
			if err != nil {
				p.errors = append(p.errors, fmt.Errorf("line %d: %w", p.line, err))
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
		return nil, fmt.Errorf("parse errors: %v", p.errors)
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

func (p *Parser) parseField(line string) (*Field, error) {
	// Skip closing brace or empty
	if line == "}" || line == "{" {
		return nil, nil
	}

	// Parse: fieldName Type @modifier1 @modifier2(arg)
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid field definition: %s", line)
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

	field := &Field{
		Name:     fieldName,
		Type:     p.parseFieldType(fieldType),
		Nullable: nullable,
	}

	// Parse modifiers
	for i := 2; i < len(parts); i++ {
		modifier := parts[i]
		if err := p.applyModifier(field, modifier); err != nil {
			return nil, err
		}
	}

	return field, nil
}

func (p *Parser) parseFieldType(typeName string) FieldType {
	switch strings.ToLower(typeName) {
	case "int", "integer":
		return FieldTypeInt
	case "bigint":
		return FieldTypeBigInt
	case "string", "varchar":
		return FieldTypeString
	case "text":
		return FieldTypeText
	case "bool", "boolean":
		return FieldTypeBool
	case "float", "double":
		return FieldTypeFloat
	case "decimal", "numeric":
		return FieldTypeDecimal
	case "datetime", "timestamp":
		return FieldTypeDateTime
	case "date":
		return FieldTypeDate
	case "time":
		return FieldTypeTime
	case "json", "jsonb":
		return FieldTypeJSON
	case "bytes", "blob", "binary":
		return FieldTypeBytes
	case "uuid":
		return FieldTypeUUID
	default:
		// Could be a relation, treat as string for now
		return FieldTypeString
	}
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
