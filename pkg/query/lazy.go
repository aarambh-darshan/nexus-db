package query

import (
	"context"
	"fmt"
	"strings"

	"github.com/nexus-db/nexus/pkg/core/schema"
	"github.com/nexus-db/nexus/pkg/dialects"
)

// LazyResult wraps a Result with lazy loading capabilities.
// Unlike eager loading which fetches all relations upfront, lazy loading
// defers relation queries until GetRelation() is called.
type LazyResult struct {
	data      Result                 // Base data
	conn      *dialects.Connection   // For database queries
	schema    *schema.Schema         // For relation lookups
	tableName string                 // Source table name
	loaded    map[string]interface{} // Cache for loaded relations
}

// NewLazyResult creates a new LazyResult from a Result.
func NewLazyResult(data Result, conn *dialects.Connection, sch *schema.Schema, tableName string) *LazyResult {
	return &LazyResult{
		data:      data,
		conn:      conn,
		schema:    sch,
		tableName: tableName,
		loaded:    make(map[string]interface{}),
	}
}

// Get returns a field value from the result.
func (lr *LazyResult) Get(key string) interface{} {
	return lr.data[key]
}

// Data returns the underlying Result map.
func (lr *LazyResult) Data() Result {
	return lr.data
}

// IsLoaded returns true if the relation has been loaded.
func (lr *LazyResult) IsLoaded(name string) bool {
	_, ok := lr.loaded[name]
	return ok
}

// GetRelation lazily loads and returns a related record.
// Returns the cached value if already loaded.
// For BelongsTo relations, returns *LazyResult.
// For HasMany relations, returns LazyResults.
func (lr *LazyResult) GetRelation(ctx context.Context, name string) (interface{}, error) {
	// Return cached if already loaded
	if cached, ok := lr.loaded[name]; ok {
		return cached, nil
	}

	// Find the model for this result
	model := lr.findModel()
	if model == nil {
		return nil, nil
	}

	// Find the relation
	rel := findRelation(model, name)
	if rel == nil {
		return nil, nil
	}

	// Load based on relation type
	var result interface{}
	var err error

	switch rel.Type {
	case schema.RelationBelongsTo:
		result, err = lr.loadBelongsTo(ctx, rel)
	case schema.RelationHasMany:
		result, err = lr.loadHasMany(ctx, rel)
	case schema.RelationHasOne:
		result, err = lr.loadHasOne(ctx, rel)
	case schema.RelationManyToMany:
		result, err = lr.loadBelongsToMany(ctx, rel)
	default:
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	// Cache the result
	lr.loaded[name] = result
	return result, nil
}

// findModel finds the schema model for this result.
func (lr *LazyResult) findModel() *schema.Model {
	if lr.schema == nil {
		return nil
	}

	// Direct lookup
	if model, exists := lr.schema.Models[lr.tableName]; exists {
		return model
	}

	// Try case-insensitive and plural matching
	for name, model := range lr.schema.Models {
		if strings.EqualFold(name, lr.tableName) ||
			strings.EqualFold(name+"s", lr.tableName) ||
			strings.EqualFold(name, strings.TrimSuffix(lr.tableName, "s")) {
			return model
		}
	}

	return nil
}

// loadBelongsTo loads a parent record for a BelongsTo relation.
func (lr *LazyResult) loadBelongsTo(ctx context.Context, rel *schema.Relation) (*LazyResult, error) {
	fkValue := lr.data[rel.ForeignKey]
	if fkValue == nil {
		return nil, nil
	}

	targetTable := toTableName(rel.TargetModel)
	related, err := lr.queryOne(ctx, targetTable, rel.ReferenceKey, fkValue)
	if err != nil {
		return nil, err
	}

	if related == nil {
		return nil, nil
	}

	return NewLazyResult(related, lr.conn, lr.schema, targetTable), nil
}

// loadHasMany loads child records for a HasMany relation.
func (lr *LazyResult) loadHasMany(ctx context.Context, rel *schema.Relation) (LazyResults, error) {
	pkValue := lr.data[rel.ReferenceKey]
	if pkValue == nil {
		return LazyResults{}, nil
	}

	targetTable := toTableName(rel.TargetModel)
	related, err := lr.queryMany(ctx, targetTable, rel.ForeignKey, pkValue)
	if err != nil {
		return nil, err
	}

	results := make(LazyResults, len(related))
	for i, r := range related {
		results[i] = NewLazyResult(r, lr.conn, lr.schema, targetTable)
	}

	return results, nil
}

// loadHasOne loads a single child record for a HasOne relation.
func (lr *LazyResult) loadHasOne(ctx context.Context, rel *schema.Relation) (*LazyResult, error) {
	pkValue := lr.data[rel.ReferenceKey]
	if pkValue == nil {
		return nil, nil
	}

	targetTable := toTableName(rel.TargetModel)
	related, err := lr.queryOne(ctx, targetTable, rel.ForeignKey, pkValue)
	if err != nil {
		return nil, err
	}

	if related == nil {
		return nil, nil
	}

	return NewLazyResult(related, lr.conn, lr.schema, targetTable), nil
}

// queryOne executes a query that returns at most one result.
func (lr *LazyResult) queryOne(ctx context.Context, table, column string, value interface{}) (Result, error) {
	dialect := lr.conn.Dialect

	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = %s LIMIT 1",
		dialect.Quote(table),
		dialect.Quote(column),
		dialect.Placeholder(1))

	rows, err := lr.conn.Query(ctx, query, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results, err := scanRows(rows)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	return results[0], nil
}

// queryMany executes a query that returns multiple results.
func (lr *LazyResult) queryMany(ctx context.Context, table, column string, value interface{}) (Results, error) {
	dialect := lr.conn.Dialect

	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = %s",
		dialect.Quote(table),
		dialect.Quote(column),
		dialect.Placeholder(1))

	rows, err := lr.conn.Query(ctx, query, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRows(rows)
}

// LazyResults is a slice of LazyResult pointers.
type LazyResults []*LazyResult

// ToResults converts LazyResults to regular Results (data only, no lazy loading).
func (lr LazyResults) ToResults() Results {
	results := make(Results, len(lr))
	for i, r := range lr {
		results[i] = r.Data()
	}
	return results
}

// loadBelongsToMany loads related records via a junction table.
func (lr *LazyResult) loadBelongsToMany(ctx context.Context, rel *schema.Relation) (LazyResults, error) {
	pkValue := lr.data[rel.ReferenceKey]
	if pkValue == nil {
		return LazyResults{}, nil
	}

	dialect := lr.conn.Dialect

	// Query junction table
	junctionQuery := fmt.Sprintf("SELECT %s FROM %s WHERE %s = %s",
		dialect.Quote(rel.ThroughTargetKey),
		dialect.Quote(rel.Through),
		dialect.Quote(rel.ThroughSourceKey),
		dialect.Placeholder(1))

	junctionRows, err := lr.conn.Query(ctx, junctionQuery, pkValue)
	if err != nil {
		return nil, err
	}
	defer junctionRows.Close()

	junctionResults, err := scanRows(junctionRows)
	if err != nil {
		return nil, err
	}

	if len(junctionResults) == 0 {
		return LazyResults{}, nil
	}

	// Collect target IDs
	targetIDs := make([]interface{}, 0, len(junctionResults))
	for _, jr := range junctionResults {
		if tid := jr[rel.ThroughTargetKey]; tid != nil {
			targetIDs = append(targetIDs, tid)
		}
	}

	if len(targetIDs) == 0 {
		return LazyResults{}, nil
	}

	// Query target table
	targetTable := toTableName(rel.TargetModel)
	placeholders := make([]string, len(targetIDs))
	for i := range targetIDs {
		placeholders[i] = dialect.Placeholder(i + 1)
	}

	targetQuery := fmt.Sprintf("SELECT * FROM %s WHERE %s IN (%s)",
		dialect.Quote(targetTable),
		dialect.Quote("id"),
		strings.Join(placeholders, ", "))

	targetRows, err := lr.conn.Query(ctx, targetQuery, targetIDs...)
	if err != nil {
		return nil, err
	}
	defer targetRows.Close()

	targetResults, err := scanRows(targetRows)
	if err != nil {
		return nil, err
	}

	results := make(LazyResults, len(targetResults))
	for i, r := range targetResults {
		results[i] = NewLazyResult(r, lr.conn, lr.schema, targetTable)
	}

	return results, nil
}
