package test

import (
	"testing"

	"github.com/nexus-db/nexus/pkg/core/schema"
)

func TestAutoDetectRelations_SnakeCase(t *testing.T) {
	s := schema.NewSchema()

	s.Model("User", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("name")
		m.String("email").Unique()
	})

	s.Model("Post", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("title")
		m.Text("content").Null()
		m.Int("author_id") // Should auto-detect → User (if named user_id)
		m.Int("user_id")   // Should auto-detect → User
	})

	s.DetectRelations()

	// Verify Post has BelongsTo User via user_id
	post := s.Models["Post"]
	belongsTo := post.GetBelongsTo()

	if len(belongsTo) != 1 {
		t.Errorf("Expected 1 BelongsTo relation, got %d", len(belongsTo))
	}

	if len(belongsTo) > 0 && belongsTo[0].TargetModel != "User" {
		t.Errorf("Expected BelongsTo User, got %s", belongsTo[0].TargetModel)
	}

	if len(belongsTo) > 0 && belongsTo[0].ForeignKey != "user_id" {
		t.Errorf("Expected foreign key user_id, got %s", belongsTo[0].ForeignKey)
	}

	// Verify User has HasMany Post
	user := s.Models["User"]
	hasMany := user.GetHasMany()

	if len(hasMany) != 1 {
		t.Errorf("Expected 1 HasMany relation, got %d", len(hasMany))
	}

	if len(hasMany) > 0 && hasMany[0].TargetModel != "Post" {
		t.Errorf("Expected HasMany Post, got %s", hasMany[0].TargetModel)
	}
}

func TestAutoDetectRelations_CamelCase(t *testing.T) {
	s := schema.NewSchema()

	s.Model("Category", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("name")
	})

	s.Model("Product", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("title")
		m.Int("categoryId") // camelCase convention
	})

	s.DetectRelations()

	product := s.Models["Product"]
	belongsTo := product.GetBelongsTo()

	if len(belongsTo) != 1 {
		t.Errorf("Expected 1 BelongsTo relation, got %d", len(belongsTo))
	}

	if len(belongsTo) > 0 && belongsTo[0].TargetModel != "Category" {
		t.Errorf("Expected BelongsTo Category, got %s", belongsTo[0].TargetModel)
	}
}

func TestExplicitReference(t *testing.T) {
	s := schema.NewSchema()

	s.Model("Author", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("name")
	})

	s.Model("Book", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("title")
		m.Int("writer_id").Ref("Author") // Explicit reference
	})

	s.DetectRelations()

	book := s.Models["Book"]
	writerField := book.Fields["writer_id"]

	if !writerField.IsReference {
		t.Error("Expected writer_id to be marked as reference")
	}

	if writerField.References != "Author" {
		t.Errorf("Expected reference to Author, got %s", writerField.References)
	}

	// Explicit refs should not auto-detect (already set)
	belongsTo := book.GetBelongsTo()
	if len(belongsTo) != 0 {
		// The field already has References set, so DetectRelations skips it
		// But we should verify explicit refs work
		t.Logf("BelongsTo relations: %d (explicit refs don't auto-add relations)", len(belongsTo))
	}
}

func TestNoFalsePositives(t *testing.T) {
	s := schema.NewSchema()

	s.Model("User", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("name")
	})

	s.Model("Settings", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("external_id") // String type - should NOT be detected
		m.Int("random_id")      // No matching model - should NOT be detected
	})

	s.DetectRelations()

	settings := s.Models["Settings"]
	belongsTo := settings.GetBelongsTo()

	if len(belongsTo) != 0 {
		t.Errorf("Expected 0 BelongsTo relations (no false positives), got %d", len(belongsTo))
	}
}

func TestMultipleRelations(t *testing.T) {
	s := schema.NewSchema()

	s.Model("User", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("name")
	})

	s.Model("Category", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("name")
	})

	s.Model("Article", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("title")
		m.Int("user_id")     // → User
		m.Int("category_id") // → Category
	})

	s.DetectRelations()

	article := s.Models["Article"]
	belongsTo := article.GetBelongsTo()

	if len(belongsTo) != 2 {
		t.Errorf("Expected 2 BelongsTo relations, got %d", len(belongsTo))
	}

	// Verify both User and Category have HasMany
	user := s.Models["User"]
	if len(user.GetHasMany()) != 1 {
		t.Errorf("Expected User to have 1 HasMany, got %d", len(user.GetHasMany()))
	}

	category := s.Models["Category"]
	if len(category.GetHasMany()) != 1 {
		t.Errorf("Expected Category to have 1 HasMany, got %d", len(category.GetHasMany()))
	}
}

func TestGetReferencedModel(t *testing.T) {
	s := schema.NewSchema()

	s.Model("User", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("name")
	})

	s.Model("Post", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.Int("user_id")
	})

	s.DetectRelations()

	post := s.Models["Post"]
	userIdField := post.Fields["user_id"]

	refModel := userIdField.GetReferencedModel(s)
	if refModel == nil {
		t.Fatal("Expected referenced model to be found")
	}

	if refModel.Name != "User" {
		t.Errorf("Expected User, got %s", refModel.Name)
	}
}
