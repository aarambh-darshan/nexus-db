// Package schema provides relation detection utilities.
package schema

import (
	"strings"
	"unicode"
)

// DetectRelations scans all models and auto-detects relations based on
// field naming conventions. Convention: field_id → Field model.
// Example: author_id → Author, user_id → User, category_id → Category
func (s *Schema) DetectRelations() {
	for _, model := range s.modelList {
		s.detectModelRelations(model)
	}
}

func (s *Schema) detectModelRelations(model *Model) {
	for _, field := range model.fieldList {
		// Skip if already has explicit reference
		if field.References != "" {
			continue
		}

		// Only consider integer types as potential foreign keys
		if field.Type != FieldTypeInt && field.Type != FieldTypeBigInt {
			continue
		}

		// Convention: field_id, fieldId → Model
		targetModel := extractModelName(field.Name)
		if targetModel == "" {
			continue
		}

		// Check if target model exists
		target, exists := s.Models[targetModel]
		if !exists {
			continue
		}

		// Mark field as reference
		field.References = targetModel
		field.IsReference = true

		// Get primary key of target
		refKey := s.getPrimaryKeyName(target)

		// Add BelongsTo on source model
		model.Relations = append(model.Relations, &Relation{
			Type:         RelationBelongsTo,
			TargetModel:  targetModel,
			ForeignKey:   field.Name,
			ReferenceKey: refKey,
		})

		// Add HasMany on target model (reverse relation)
		target.Relations = append(target.Relations, &Relation{
			Type:         RelationHasMany,
			TargetModel:  model.Name,
			ForeignKey:   field.Name,
			ReferenceKey: refKey,
		})
	}
}

// extractModelName extracts the model name from a foreign key field name.
// Supports snake_case (author_id → Author) and camelCase (authorId → Author).
func extractModelName(fieldName string) string {
	// Check for snake_case: ends with _id
	if strings.HasSuffix(strings.ToLower(fieldName), "_id") {
		// Extract the part before _id
		idx := strings.LastIndex(strings.ToLower(fieldName), "_id")
		if idx > 0 {
			prefix := fieldName[:idx]
			return toPascalCase(prefix)
		}
	}

	// Check for camelCase: ends with Id (capital I)
	if strings.HasSuffix(fieldName, "Id") && len(fieldName) > 2 {
		prefix := fieldName[:len(fieldName)-2]
		return toPascalCase(prefix)
	}

	return ""
}

// toPascalCase converts a string to PascalCase.
// user → User, user_profile → UserProfile
func toPascalCase(s string) string {
	if s == "" {
		return ""
	}

	// Handle snake_case
	parts := strings.Split(s, "_")
	var result strings.Builder
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		// Capitalize first letter
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		result.WriteString(string(runes))
	}

	return result.String()
}

// getPrimaryKeyName returns the primary key field name for a model.
// Defaults to "id" if no explicit primary key is found.
func (s *Schema) getPrimaryKeyName(model *Model) string {
	for _, field := range model.fieldList {
		if field.IsPrimaryKey {
			return field.Name
		}
	}
	return "id"
}

// GetRelations returns all detected relations for a model.
func (m *Model) GetRelations() []*Relation {
	return m.Relations
}

// GetBelongsTo returns only the BelongsTo relations for a model.
func (m *Model) GetBelongsTo() []*Relation {
	var result []*Relation
	for _, rel := range m.Relations {
		if rel.Type == RelationBelongsTo {
			result = append(result, rel)
		}
	}
	return result
}

// GetHasMany returns only the HasMany relations for a model.
func (m *Model) GetHasMany() []*Relation {
	var result []*Relation
	for _, rel := range m.Relations {
		if rel.Type == RelationHasMany {
			result = append(result, rel)
		}
	}
	return result
}

// GetHasOne returns only the HasOne relations for a model.
func (m *Model) GetHasOne() []*Relation {
	var result []*Relation
	for _, rel := range m.Relations {
		if rel.Type == RelationHasOne {
			result = append(result, rel)
		}
	}
	return result
}

// GetReferencedModel returns the model this field references, if any.
func (f *Field) GetReferencedModel(schema *Schema) *Model {
	if f.References == "" {
		return nil
	}
	return schema.Models[f.References]
}
