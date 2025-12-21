package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-db/nexus/pkg/core/schema"
)

// preloadRelations loads related data for the given results based on includes.
func (s *SelectBuilder) preloadRelations(ctx context.Context, results Results) error {
	if s.schema == nil || len(s.includes) == 0 {
		return nil
	}

	model, exists := s.schema.Models[s.tableName]
	if !exists {
		// Try lowercase table name as model lookup
		for name, m := range s.schema.Models {
			if strings.EqualFold(name, s.tableName) || strings.EqualFold(name+"s", s.tableName) {
				model = m
				exists = true
				break
			}
		}
	}
	if !exists {
		return nil
	}

	for _, include := range s.includes {
		rel := findRelation(model, include)
		if rel == nil {
			continue
		}

		switch rel.Type {
		case schema.RelationBelongsTo:
			if err := s.preloadBelongsTo(ctx, results, rel); err != nil {
				return err
			}
		case schema.RelationHasMany:
			if err := s.preloadHasMany(ctx, results, rel); err != nil {
				return err
			}
		case schema.RelationHasOne:
			if err := s.preloadHasOne(ctx, results, rel); err != nil {
				return err
			}
		}
	}

	return nil
}

// findRelation finds a relation by name (target model name).
func findRelation(model *schema.Model, name string) *schema.Relation {
	for _, rel := range model.GetRelations() {
		if strings.EqualFold(rel.TargetModel, name) {
			return rel
		}
	}
	return nil
}

// preloadBelongsTo loads parent records for BelongsTo relations.
// Example: For Posts with user_id, load corresponding Users.
func (s *SelectBuilder) preloadBelongsTo(ctx context.Context, results Results, rel *schema.Relation) error {
	if len(results) == 0 {
		return nil
	}

	// Collect foreign key values from results
	fkValues := collectFieldValues(results, rel.ForeignKey)
	if len(fkValues) == 0 {
		return nil
	}

	// Query related records
	targetTable := toTableName(rel.TargetModel)
	related, err := s.queryRelated(ctx, targetTable, rel.ReferenceKey, fkValues)
	if err != nil {
		return err
	}

	// Build lookup map: referenceKey -> related record
	lookup := make(map[interface{}]Result)
	for _, r := range related {
		key := r[rel.ReferenceKey]
		lookup[key] = r
	}

	// Associate related records to parent results
	for i := range results {
		fkValue := results[i][rel.ForeignKey]
		if relatedRecord, ok := lookup[fkValue]; ok {
			results[i][rel.TargetModel] = relatedRecord
		}
	}

	return nil
}

// preloadHasMany loads child records for HasMany relations.
// Example: For Users, load all their Posts.
func (s *SelectBuilder) preloadHasMany(ctx context.Context, results Results, rel *schema.Relation) error {
	if len(results) == 0 {
		return nil
	}

	// Collect primary key values from results
	pkValues := collectFieldValues(results, rel.ReferenceKey)
	if len(pkValues) == 0 {
		return nil
	}

	// Query related records
	targetTable := toTableName(rel.TargetModel)
	related, err := s.queryRelated(ctx, targetTable, rel.ForeignKey, pkValues)
	if err != nil {
		return err
	}

	// Build lookup map: foreignKey -> []related records
	lookup := make(map[interface{}]Results)
	for _, r := range related {
		fkValue := r[rel.ForeignKey]
		lookup[fkValue] = append(lookup[fkValue], r)
	}

	// Associate related records to parent results
	for i := range results {
		pkValue := results[i][rel.ReferenceKey]
		if relatedRecords, ok := lookup[pkValue]; ok {
			results[i][rel.TargetModel] = relatedRecords
		} else {
			results[i][rel.TargetModel] = Results{}
		}
	}

	return nil
}

// preloadHasOne loads single child record for HasOne relations.
func (s *SelectBuilder) preloadHasOne(ctx context.Context, results Results, rel *schema.Relation) error {
	if len(results) == 0 {
		return nil
	}

	// Collect primary key values from results
	pkValues := collectFieldValues(results, rel.ReferenceKey)
	if len(pkValues) == 0 {
		return nil
	}

	// Query related records
	targetTable := toTableName(rel.TargetModel)
	related, err := s.queryRelated(ctx, targetTable, rel.ForeignKey, pkValues)
	if err != nil {
		return err
	}

	// Build lookup map: foreignKey -> related record (first match)
	lookup := make(map[interface{}]Result)
	for _, r := range related {
		fkValue := r[rel.ForeignKey]
		if _, exists := lookup[fkValue]; !exists {
			lookup[fkValue] = r
		}
	}

	// Associate related records to parent results
	for i := range results {
		pkValue := results[i][rel.ReferenceKey]
		if relatedRecord, ok := lookup[pkValue]; ok {
			results[i][rel.TargetModel] = relatedRecord
		}
	}

	return nil
}

// queryRelated executes a query for related records using IN clause.
func (s *SelectBuilder) queryRelated(ctx context.Context, table, column string, values []interface{}) (Results, error) {
	if len(values) == 0 {
		return nil, nil
	}

	dialect := s.conn.Dialect

	// Build placeholders
	placeholders := make([]string, len(values))
	for i := range values {
		placeholders[i] = dialect.Placeholder(i + 1)
	}

	query := fmt.Sprintf("SELECT * FROM %s WHERE %s IN (%s)",
		dialect.Quote(table),
		dialect.Quote(column),
		strings.Join(placeholders, ", "))

	rows, err := s.conn.Query(ctx, query, values...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRows(rows)
}

// collectFieldValues extracts unique values of a field from results.
func collectFieldValues(results Results, fieldName string) []interface{} {
	seen := make(map[interface{}]bool)
	var values []interface{}

	for _, r := range results {
		if val, ok := r[fieldName]; ok && val != nil {
			if !seen[val] {
				seen[val] = true
				values = append(values, val)
			}
		}
	}

	return values
}

// toTableName converts a model name to table name (lowercase + 's').
// User -> users, Post -> posts
func toTableName(modelName string) string {
	return strings.ToLower(modelName) + "s"
}
