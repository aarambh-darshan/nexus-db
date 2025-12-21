// Package schema provides types and utilities for defining database schemas.
package schema

import (
	"fmt"
	"strings"
)

// Schema represents a complete database schema with models and relations.
type Schema struct {
	Models    map[string]*Model
	modelList []*Model // Preserve order
}

// NewSchema creates a new empty schema.
func NewSchema() *Schema {
	return &Schema{
		Models: make(map[string]*Model),
	}
}

// Model defines a model (table) in the schema using a fluent API.
func (s *Schema) Model(name string, fn func(m *Model)) *Schema {
	m := &Model{
		Name:   name,
		Fields: make(map[string]*Field),
	}
	fn(m)
	s.Models[name] = m
	s.modelList = append(s.modelList, m)
	return s
}

// GetModels returns models in definition order.
func (s *Schema) GetModels() []*Model {
	return s.modelList
}

// Model represents a database table.
type Model struct {
	Name      string
	Fields    map[string]*Field
	fieldList []*Field // Preserve order
	Indexes   []*Index
	Relations []*Relation
}

// GetFields returns fields in definition order.
func (m *Model) GetFields() []*Field {
	return m.fieldList
}

// addField adds a field to the model.
func (m *Model) addField(f *Field) *Field {
	f.Model = m
	m.Fields[f.Name] = f
	m.fieldList = append(m.fieldList, f)
	return f
}

// Int creates an integer field.
func (m *Model) Int(name string) *Field {
	return m.addField(&Field{Name: name, Type: FieldTypeInt})
}

// BigInt creates a big integer field.
func (m *Model) BigInt(name string) *Field {
	return m.addField(&Field{Name: name, Type: FieldTypeBigInt})
}

// String creates a string/varchar field.
func (m *Model) String(name string) *Field {
	return m.addField(&Field{Name: name, Type: FieldTypeString, Length: 255})
}

// Text creates a text field (unlimited length).
func (m *Model) Text(name string) *Field {
	return m.addField(&Field{Name: name, Type: FieldTypeText})
}

// Bool creates a boolean field.
func (m *Model) Bool(name string) *Field {
	return m.addField(&Field{Name: name, Type: FieldTypeBool})
}

// Float creates a floating-point field.
func (m *Model) Float(name string) *Field {
	return m.addField(&Field{Name: name, Type: FieldTypeFloat})
}

// Decimal creates a decimal/numeric field.
func (m *Model) Decimal(name string) *Field {
	return m.addField(&Field{Name: name, Type: FieldTypeDecimal, Precision: 10, Scale: 2})
}

// DateTime creates a datetime/timestamp field.
func (m *Model) DateTime(name string) *Field {
	return m.addField(&Field{Name: name, Type: FieldTypeDateTime})
}

// Date creates a date field.
func (m *Model) Date(name string) *Field {
	return m.addField(&Field{Name: name, Type: FieldTypeDate})
}

// Time creates a time field.
func (m *Model) Time(name string) *Field {
	return m.addField(&Field{Name: name, Type: FieldTypeTime})
}

// JSON creates a JSON field.
func (m *Model) JSON(name string) *Field {
	return m.addField(&Field{Name: name, Type: FieldTypeJSON})
}

// Bytes creates a binary/blob field.
func (m *Model) Bytes(name string) *Field {
	return m.addField(&Field{Name: name, Type: FieldTypeBytes})
}

// UUID creates a UUID field.
func (m *Model) UUID(name string) *Field {
	return m.addField(&Field{Name: name, Type: FieldTypeUUID})
}

// Index adds an index to the model.
func (m *Model) Index(name string, fields ...string) *Model {
	m.Indexes = append(m.Indexes, &Index{
		Name:   name,
		Fields: fields,
		Unique: false,
	})
	return m
}

// UniqueIndex adds a unique index to the model.
func (m *Model) UniqueIndex(name string, fields ...string) *Model {
	m.Indexes = append(m.Indexes, &Index{
		Name:   name,
		Fields: fields,
		Unique: true,
	})
	return m
}

// BelongsTo creates a belongs-to relation.
func (m *Model) BelongsTo(targetModel, foreignKey string) *Model {
	m.Relations = append(m.Relations, &Relation{
		Type:         RelationBelongsTo,
		TargetModel:  targetModel,
		ForeignKey:   foreignKey,
		ReferenceKey: "id",
	})
	return m
}

// HasMany creates a has-many relation.
func (m *Model) HasMany(targetModel, foreignKey string) *Model {
	m.Relations = append(m.Relations, &Relation{
		Type:         RelationHasMany,
		TargetModel:  targetModel,
		ForeignKey:   foreignKey,
		ReferenceKey: "id",
	})
	return m
}

// HasOne creates a has-one relation.
func (m *Model) HasOne(targetModel, foreignKey string) *Model {
	m.Relations = append(m.Relations, &Relation{
		Type:         RelationHasOne,
		TargetModel:  targetModel,
		ForeignKey:   foreignKey,
		ReferenceKey: "id",
	})
	return m
}

// FieldType represents a column data type.
type FieldType int

const (
	FieldTypeInt FieldType = iota
	FieldTypeBigInt
	FieldTypeString
	FieldTypeText
	FieldTypeBool
	FieldTypeFloat
	FieldTypeDecimal
	FieldTypeDateTime
	FieldTypeDate
	FieldTypeTime
	FieldTypeJSON
	FieldTypeBytes
	FieldTypeUUID
)

// String returns the string representation of a field type.
func (ft FieldType) String() string {
	names := []string{
		"Int", "BigInt", "String", "Text", "Bool", "Float",
		"Decimal", "DateTime", "Date", "Time", "JSON", "Bytes", "UUID",
	}
	if int(ft) < len(names) {
		return names[ft]
	}
	return "Unknown"
}

// Field represents a column in a model.
type Field struct {
	Name          string
	Type          FieldType
	Model         *Model
	Nullable      bool
	IsPrimaryKey  bool
	IsUnique      bool
	AutoIncrement bool
	Length        int
	Precision     int
	Scale         int
	DefaultValue  interface{}
	DefaultExpr   string // For expressions like NOW()

	// Relation detection
	References  string // Target model name (e.g., "User")
	IsReference bool   // True if this is a foreign key field
}

// PrimaryKey marks this field as the primary key.
func (f *Field) PrimaryKey() *Field {
	f.IsPrimaryKey = true
	return f
}

// AutoInc marks this field as auto-incrementing.
func (f *Field) AutoInc() *Field {
	f.AutoIncrement = true
	return f
}

// Unique marks this field as unique.
func (f *Field) Unique() *Field {
	f.IsUnique = true
	return f
}

// Null marks this field as nullable.
func (f *Field) Null() *Field {
	f.Nullable = true
	return f
}

// NotNull marks this field as not nullable (default).
func (f *Field) NotNull() *Field {
	f.Nullable = false
	return f
}

// Default sets the default value.
func (f *Field) Default(value interface{}) *Field {
	f.DefaultValue = value
	return f
}

// DefaultNow sets the default to the current timestamp.
func (f *Field) DefaultNow() *Field {
	f.DefaultExpr = "NOW()"
	return f
}

// DefaultUUID sets the default to a generated UUID.
func (f *Field) DefaultUUID() *Field {
	f.DefaultExpr = "UUID()"
	return f
}

// Size sets the length for string fields.
func (f *Field) Size(length int) *Field {
	f.Length = length
	return f
}

// Prec sets precision and scale for decimal fields.
func (f *Field) Prec(precision, scale int) *Field {
	f.Precision = precision
	f.Scale = scale
	return f
}

// Ref explicitly marks this field as referencing another model.
// This overrides auto-detection for cases where naming conventions don't apply.
func (f *Field) Ref(modelName string) *Field {
	f.References = modelName
	f.IsReference = true
	return f
}

// Index represents a database index.
type Index struct {
	Name   string
	Fields []string
	Unique bool
}

// RelationType represents the type of relation.
type RelationType int

const (
	RelationBelongsTo RelationType = iota
	RelationHasOne
	RelationHasMany
	RelationManyToMany
)

// CascadeAction defines what happens to related records on delete/update.
type CascadeAction int

const (
	// NoAction does nothing to related records (default).
	NoAction CascadeAction = iota
	// Cascade deletes/updates related records.
	Cascade
	// SetNull sets the foreign key to NULL.
	SetNull
	// Restrict prevents the operation if related records exist.
	Restrict
)

// Relation represents a relationship between models.
type Relation struct {
	Type             RelationType
	TargetModel      string
	ForeignKey       string
	ReferenceKey     string
	Through          string        // Junction table name for many-to-many
	ThroughSourceKey string        // FK in junction pointing to source model
	ThroughTargetKey string        // FK in junction pointing to target model
	OnDeleteAction   CascadeAction // Action on parent delete
	OnUpdateAction   CascadeAction // Action on parent update
}

// OnDelete sets the cascade action for delete operations.
func (r *Relation) OnDelete(action CascadeAction) *Relation {
	r.OnDeleteAction = action
	return r
}

// OnUpdate sets the cascade action for update operations.
func (r *Relation) OnUpdate(action CascadeAction) *Relation {
	r.OnUpdateAction = action
	return r
}

// BelongsToMany creates a many-to-many relation via a junction table.
// Example: m.BelongsToMany("Tag", "user_tags", "user_id", "tag_id")
func (m *Model) BelongsToMany(targetModel, through, sourceKey, targetKey string) *Relation {
	rel := &Relation{
		Type:             RelationManyToMany,
		TargetModel:      targetModel,
		Through:          through,
		ThroughSourceKey: sourceKey,
		ThroughTargetKey: targetKey,
		ReferenceKey:     "id",
	}
	m.Relations = append(m.Relations, rel)
	return rel
}

// Validate validates the schema for correctness.
func (s *Schema) Validate() error {
	var errors []string

	for _, model := range s.modelList {
		// Check for primary key
		hasPK := false
		for _, field := range model.fieldList {
			if field.IsPrimaryKey {
				hasPK = true
				break
			}
		}
		if !hasPK {
			errors = append(errors, fmt.Sprintf("model %q has no primary key", model.Name))
		}

		// Validate relations
		for _, rel := range model.Relations {
			if _, exists := s.Models[rel.TargetModel]; !exists {
				errors = append(errors, fmt.Sprintf("model %q references unknown model %q", model.Name, rel.TargetModel))
			}
		}

		// Validate indexes
		for _, idx := range model.Indexes {
			for _, fieldName := range idx.Fields {
				if _, exists := model.Fields[fieldName]; !exists {
					errors = append(errors, fmt.Sprintf("index %q references unknown field %q in model %q", idx.Name, fieldName, model.Name))
				}
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("schema validation failed:\n  - %s", strings.Join(errors, "\n  - "))
	}
	return nil
}
