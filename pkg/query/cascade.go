package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-db/nexus/pkg/core/schema"
	"github.com/nexus-db/nexus/pkg/dialects"
)

// cascadeDelete handles cascade operations for a delete.
// It processes HasMany and HasOne relations with cascade/setNull actions.
func cascadeDelete(ctx context.Context, conn *dialects.Connection, sch *schema.Schema,
	tableName string, deletedRows Results) error {

	model := findModelByTable(sch, tableName)
	if model == nil {
		return nil
	}

	for _, rel := range model.GetRelations() {
		// Only process HasMany and HasOne (parent -> children)
		if rel.Type != schema.RelationHasMany && rel.Type != schema.RelationHasOne {
			continue
		}

		switch rel.OnDeleteAction {
		case schema.Cascade:
			if err := cascadeDeleteRelated(ctx, conn, rel, deletedRows); err != nil {
				return err
			}
		case schema.SetNull:
			if err := setNullRelated(ctx, conn, rel, deletedRows); err != nil {
				return err
			}
		case schema.Restrict:
			hasRelated, err := hasRelatedRecords(ctx, conn, rel, deletedRows)
			if err != nil {
				return err
			}
			if hasRelated {
				return fmt.Errorf("cannot delete: related %s records exist (restrict)", rel.TargetModel)
			}
		}
	}

	return nil
}

// cascadeDeleteRelated deletes related records for cascade action.
func cascadeDeleteRelated(ctx context.Context, conn *dialects.Connection,
	rel *schema.Relation, parentRows Results) error {

	pkValues := collectFieldValues(parentRows, rel.ReferenceKey)
	if len(pkValues) == 0 {
		return nil
	}

	targetTable := toTableName(rel.TargetModel)
	dialect := conn.Dialect

	// Build DELETE ... WHERE fk IN (...)
	placeholders := make([]string, len(pkValues))
	for i := range pkValues {
		placeholders[i] = dialect.Placeholder(i + 1)
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)",
		dialect.Quote(targetTable),
		dialect.Quote(rel.ForeignKey),
		strings.Join(placeholders, ", "))

	_, err := conn.Exec(ctx, query, pkValues...)
	return err
}

// setNullRelated sets foreign keys to NULL for setNull action.
func setNullRelated(ctx context.Context, conn *dialects.Connection,
	rel *schema.Relation, parentRows Results) error {

	pkValues := collectFieldValues(parentRows, rel.ReferenceKey)
	if len(pkValues) == 0 {
		return nil
	}

	targetTable := toTableName(rel.TargetModel)
	dialect := conn.Dialect

	// Build UPDATE ... SET fk = NULL WHERE fk IN (...)
	placeholders := make([]string, len(pkValues))
	for i := range pkValues {
		placeholders[i] = dialect.Placeholder(i + 1)
	}

	query := fmt.Sprintf("UPDATE %s SET %s = NULL WHERE %s IN (%s)",
		dialect.Quote(targetTable),
		dialect.Quote(rel.ForeignKey),
		dialect.Quote(rel.ForeignKey),
		strings.Join(placeholders, ", "))

	_, err := conn.Exec(ctx, query, pkValues...)
	return err
}

// hasRelatedRecords checks if any related records exist.
func hasRelatedRecords(ctx context.Context, conn *dialects.Connection,
	rel *schema.Relation, parentRows Results) (bool, error) {

	pkValues := collectFieldValues(parentRows, rel.ReferenceKey)
	if len(pkValues) == 0 {
		return false, nil
	}

	targetTable := toTableName(rel.TargetModel)
	dialect := conn.Dialect

	placeholders := make([]string, len(pkValues))
	for i := range pkValues {
		placeholders[i] = dialect.Placeholder(i + 1)
	}

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s IN (%s)",
		dialect.Quote(targetTable),
		dialect.Quote(rel.ForeignKey),
		strings.Join(placeholders, ", "))

	row := conn.QueryRow(ctx, query, pkValues...)
	var count int64
	if err := row.Scan(&count); err != nil {
		return false, err
	}

	return count > 0, nil
}

// cascadeUpdate handles cascade operations for an update.
// If the reference key (usually primary key) is being updated,
// propagate the change to related records.
func cascadeUpdate(ctx context.Context, conn *dialects.Connection, sch *schema.Schema,
	tableName string, oldRows Results, newData map[string]interface{}) error {

	model := findModelByTable(sch, tableName)
	if model == nil {
		return nil
	}

	// Find the primary key field
	pkField := ""
	for _, field := range model.GetFields() {
		if field.IsPrimaryKey {
			pkField = field.Name
			break
		}
	}

	// Check if PK is being updated
	newPKValue, pkUpdated := newData[pkField]
	if !pkUpdated {
		return nil
	}

	for _, rel := range model.GetRelations() {
		if rel.Type != schema.RelationHasMany && rel.Type != schema.RelationHasOne {
			continue
		}

		switch rel.OnUpdateAction {
		case schema.Cascade:
			if err := cascadeUpdateRelated(ctx, conn, rel, oldRows, pkField, newPKValue); err != nil {
				return err
			}
		case schema.SetNull:
			if err := setNullRelated(ctx, conn, rel, oldRows); err != nil {
				return err
			}
		case schema.Restrict:
			hasRelated, err := hasRelatedRecords(ctx, conn, rel, oldRows)
			if err != nil {
				return err
			}
			if hasRelated {
				return fmt.Errorf("cannot update: related %s records exist (restrict)", rel.TargetModel)
			}
		}
	}

	return nil
}

// cascadeUpdateRelated updates foreign keys in related records.
func cascadeUpdateRelated(ctx context.Context, conn *dialects.Connection,
	rel *schema.Relation, parentRows Results, pkField string, newPKValue interface{}) error {

	for _, row := range parentRows {
		oldPKValue := row[pkField]
		if oldPKValue == nil {
			continue
		}

		targetTable := toTableName(rel.TargetModel)
		dialect := conn.Dialect

		query := fmt.Sprintf("UPDATE %s SET %s = %s WHERE %s = %s",
			dialect.Quote(targetTable),
			dialect.Quote(rel.ForeignKey),
			dialect.Placeholder(1),
			dialect.Quote(rel.ForeignKey),
			dialect.Placeholder(2))

		_, err := conn.Exec(ctx, query, newPKValue, oldPKValue)
		if err != nil {
			return err
		}
	}

	return nil
}

// findModelByTable finds a model by table name (case-insensitive, with pluralization).
func findModelByTable(sch *schema.Schema, tableName string) *schema.Model {
	if sch == nil {
		return nil
	}

	// Direct lookup
	if model, exists := sch.Models[tableName]; exists {
		return model
	}

	// Case-insensitive and plural matching
	for name, model := range sch.Models {
		if strings.EqualFold(name, tableName) ||
			strings.EqualFold(name+"s", tableName) ||
			strings.EqualFold(name, strings.TrimSuffix(tableName, "s")) {
			return model
		}
	}

	return nil
}
