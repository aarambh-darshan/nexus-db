// Minimal example demonstrating Nexus-DB usage.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"

	"github.com/nexus-db/nexus/pkg/core/migration"
	"github.com/nexus-db/nexus/pkg/core/schema"
	"github.com/nexus-db/nexus/pkg/dialects"
	"github.com/nexus-db/nexus/pkg/dialects/sqlite"
	"github.com/nexus-db/nexus/pkg/query"
)

func main() {
	ctx := context.Background()

	// 1. Define schema using Go API
	s := schema.NewSchema()

	s.Model("User", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("email").Unique()
		m.String("name").Size(100)
		m.DateTime("created_at").DefaultNow()
	})

	s.Model("Post", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("title")
		m.Text("content").Null()
		m.Int("author_id")
		m.DateTime("created_at").DefaultNow()
	})

	// 2. Validate schema
	if err := s.Validate(); err != nil {
		log.Fatalf("Schema validation failed: %v", err)
	}
	fmt.Println("✓ Schema validated successfully")

	// 3. Connect to SQLite
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	dialect := sqlite.New()
	conn := dialects.NewConnection(db, dialect)

	// 4. Generate and apply migrations
	engine := migration.NewEngine(conn)
	if err := engine.Init(ctx); err != nil {
		log.Fatal(err)
	}

	// Create tables manually (in production, use migration files)
	for _, model := range s.GetModels() {
		sql := dialect.CreateTableSQL(model)
		fmt.Printf("Creating table: %s\n", model.Name)
		if _, err := conn.Exec(ctx, sql); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("✓ Tables created successfully")

	// 5. Use query builder
	users := query.New(conn, "User")

	// Insert a user
	_, err = users.Insert(map[string]interface{}{
		"email": "alice@example.com",
		"name":  "Alice",
	}).Exec(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("✓ User inserted")

	// Insert another user
	_, err = users.Insert(map[string]interface{}{
		"email": "bob@example.com",
		"name":  "Bob",
	}).Exec(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Query all users
	allUsers, err := users.Select("id", "email", "name").All(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Found %d users:\n", len(allUsers))
	for _, u := range allUsers {
		fmt.Printf("  - ID: %v, Email: %v, Name: %v\n", u["id"], u["email"], u["name"])
	}

	// Query with condition
	alice, err := users.Select().Where(query.Eq("email", "alice@example.com")).One(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Found Alice: %v\n", alice["name"])

	// Update user
	affected, err := users.Update(map[string]interface{}{
		"name": "Alice Smith",
	}).Where(query.Eq("id", 1)).Exec(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Updated %d user(s)\n", affected)

	// Delete user
	affected, err = users.Delete().Where(query.Eq("id", 2)).Exec(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Deleted %d user(s)\n", affected)

	// Count remaining users
	count, err := users.Select().Count(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("✓ Remaining users: %d\n", count)

	fmt.Println("\n🎉 Example completed successfully!")
}
