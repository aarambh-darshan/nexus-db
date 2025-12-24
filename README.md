# Nexus-DB

<div align="center">

**Schema-first database framework for Go**

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

</div>

---

A Prisma/Drizzle-inspired database toolkit providing:

- 🔷 **Schema-first design** - Define models in DSL or Go code
- 🔄 **Auto-migrations** - Generate and track schema changes
- 🔍 **Type-safe queries** - Fluent query builder API
- 🗃️ **Multi-dialect** - PostgreSQL, SQLite, MySQL support
- ⚡ **Code generation** - Generate Go types from schemas

## Quick Start

### Installation

```bash
go get github.com/nexus-db/nexus
```

### Define Your Schema

**Option 1: Using Go API**

```go
schema := nexus.NewSchema()

schema.Model("User", func(m *nexus.Model) {
    m.Int("id").PrimaryKey().AutoInc()
    m.String("email").Unique()
    m.String("name").Null()
    m.DateTime("createdAt").DefaultNow()
})
```

**Option 2: Using DSL (schema.nexus)**

```prisma
model User {
  id        Int       @id @autoincrement
  email     String    @unique
  name      String?
  createdAt DateTime  @default(now())
}
```

### Connect & Query

```go
import (
    "github.com/nexus-db/nexus/pkg/dialects"
    "github.com/nexus-db/nexus/pkg/dialects/sqlite"
    "github.com/nexus-db/nexus/pkg/query"
)

// Connect
db, _ := sql.Open("sqlite3", "app.db")
conn := dialects.NewConnection(db, sqlite.New())

// Query
users := query.New(conn, "User")

// SELECT
all, _ := users.Select("id", "email").
    Where(query.Like("email", "%@example.com")).
    OrderBy("id", query.Desc).
    Limit(10).
    All(ctx)

// INSERT
users.Insert(map[string]any{
    "email": "alice@example.com",
    "name":  "Alice",
}).Exec(ctx)

// UPDATE
users.Update(map[string]any{"name": "Alice Smith"}).
    Where(query.Eq("id", 1)).
    Exec(ctx)

// DELETE
users.Delete().Where(query.Eq("id", 1)).Exec(ctx)
```

## Project Structure

```
nexus-db/
├── cmd/nexus/           # CLI tool
├── pkg/
│   ├── core/
│   │   ├── schema/      # Schema engine & DSL parser
│   │   └── migration/   # Migration engine
│   ├── dialects/        # PostgreSQL, SQLite, MySQL
│   └── query/           # Query builder
├── internal/codegen/    # Code generation
└── examples/            # Usage examples
```

## CLI Commands

```bash
# Initialize project
nexus init

# Create migration
nexus migrate new create_users

# Auto-generate migration from schema changes (v0.4.0+)
nexus migrate diff add_posts

# Squash migrations into one (v0.4.0+)
nexus migrate squash initial_schema

# Apply migrations
nexus migrate up

# Rollback last migration
nexus migrate down

# Rollback to specific version (v0.4.0+)
nexus migrate down --to 20231201_100000

# Rollback multiple migrations (v0.4.0+)
nexus migrate down -n 3

# Check status
nexus migrate status

# Validate migrations (v0.4.0+)
nexus migrate validate

# Force break stale locks (v0.4.0+)
nexus migrate up --force

# Run seed data (v0.4.0+)
nexus seed

# Run seeds for specific environment
nexus seed --env dev

# Create new seed file
nexus seed new users

# Generate Go types
nexus gen

# Watch mode with hot reload (v0.5.0+)
nexus dev

# Watch mode options
nexus dev --no-gen          # Watch without auto-generation
nexus dev --poll            # Use polling (for network drives)
nexus dev --interval 1s     # Set debounce interval

# Database browser UI (v0.5.0+)
nexus studio

# Studio options
nexus studio --port 3000    # Use custom port
nexus studio --no-open      # Don't auto-open browser
```

## Features

### Fluent Query Builder

```go
// Complex queries
users.Select("u.id", "u.name", "COUNT(p.id) as post_count").
    Join("Post", "u.id = Post.author_id").
    Where(query.Gte("u.created_at", startDate)).
    GroupBy("u.id").
    Having(query.Gt("COUNT(p.id)", 5)).
    OrderBy("post_count", query.Desc).
    All(ctx)

// Transactions
query.Transaction(ctx, conn, func(tx *dialects.Tx) error {
    // All queries use transaction
    return nil
})
```

### v0.2.0 Features

```go
// Raw SQL
query.NewRawQuery(conn, "SELECT * FROM users WHERE id = ?", 1).All(ctx)
query.RawExec(ctx, conn, "UPDATE users SET active = ?", true)

// Subqueries
users.Select().WhereIn("id", 
    orders.Select("user_id").Where(query.Gt("total", 100)))

// UNION / INTERSECT / EXCEPT
q1.Select("id", "name").Union(q2.Select("id", "name")).All(ctx)

// Common Table Expressions (CTEs)
query.With(conn, "active_users", 
    users.Select().Where(query.Eq("active", true))).
    Select("*").From("active_users").All(ctx)

// Statement caching
cache := query.NewStmtCacheWithStats(db, 100)

// Query logging
logger := query.NewLogger(os.Stdout, query.LogDebug)
```

### v0.3.0 Features

```go
// Auto-detect relations from schema
s := schema.NewSchema()

s.Model("User", func(m *schema.Model) {
    m.Int("id").PrimaryKey().AutoInc()
    m.String("name")
})

s.Model("Post", func(m *schema.Model) {
    m.Int("id").PrimaryKey().AutoInc()
    m.String("title")
    m.Int("user_id")  // Auto-detected → User
})

// Detect relations based on naming conventions
s.DetectRelations()

// Query relations
post := s.Models["Post"]
for _, rel := range post.GetBelongsTo() {
    fmt.Printf("Post belongs to %s via %s\n", rel.TargetModel, rel.ForeignKey)
}

// Explicit references (when conventions don't apply)
m.Int("author").Ref("User")

// Eager loading - automatically fetch related data
posts := query.NewWithSchema(conn, "posts", s)
results, _ := posts.Select().Include("User").All(ctx)

for _, post := range results {
    user := post["User"].(query.Result)
    fmt.Printf("Post '%s' by %s\n", post["title"], user["name"])
}

// HasMany - load children for each parent
users := query.NewWithSchema(conn, "users", s)
results, _ = users.Select().Include("Post").All(ctx)

// Lazy loading - defer queries until accessed
lazyPosts, _ := posts.Select().AllLazy(ctx)
for _, post := range lazyPosts {
    // User is NOT loaded yet
    user, _ := post.GetRelation(ctx, "User")  // Query happens here
    if user != nil {
        fmt.Printf("Post by %s\n", user.(*query.LazyResult).Get("name"))
    }
}

// Cascade delete - automatically delete related records
// Configure: rel.OnDelete(schema.Cascade) or schema.SetNull or schema.Restrict
users := query.NewWithSchema(conn, "users", s)
users.Delete().Where(query.Eq("id", 1)).Cascade().Exec(ctx)  // Deletes user AND posts

// Many-to-many relations via junction tables
s.Model("User", func(m *schema.Model) {
    m.BelongsToMany("Tag", "user_tags", "user_id", "tag_id")
})
results, _ = users.Select().Include("Tag").All(ctx)  // Load users with their tags
```

### v0.5.0 Features

```go
// Query Plan Analysis - understand and optimize your queries
users := query.New(conn, "users")

// Get query plan without executing
plan, _ := users.Select("id", "email").
    Where(query.Eq("email", "test@example.com")).
    Explain(ctx)

fmt.Println(plan.Raw)         // Raw EXPLAIN output
fmt.Println(plan.UsedIndexes) // Indexes used (e.g., ["idx_users_email"])
fmt.Println(plan.Warnings)    // Performance hints

// Execute and get actual timings (EXPLAIN ANALYZE)
plan, _ = users.Select().
    Where(query.Like("name", "%John%")).
    Analyze(ctx)

// Custom explain options
plan, _ = users.Select().Explain(ctx, query.ExplainOptions{
    Analyze: true,               // Execute query
    Format:  query.ExplainFormatJSON, // JSON output (PostgreSQL)
})

// Performance Profiler - track and analyze query performance
profiler := query.NewProfiler(query.DefaultProfilerOptions())
profiler.Start()

// Attach profiler to builders
users = query.New(conn, "users").WithProfiler(profiler)

// Execute queries - they're automatically profiled
users.Select("id", "email").All(ctx)
users.Insert(map[string]any{"email": "test@example.com"}).Exec(ctx)

// Get performance report
profiler.Stop()
report := profiler.Report()

fmt.Println(report.String())          // Formatted text report
fmt.Println(report.TotalQueries)      // Number of queries
fmt.Println(report.SlowQueries)       // Queries exceeding threshold
fmt.Println(report.NPlusOneWarnings)  // Detected N+1 patterns
fmt.Println(report.Suggestions)       // Optimization tips
```

### Dialect Support

| Feature | PostgreSQL | SQLite | MySQL |
|---------|:----------:|:------:|:-----:|
| RETURNING | ✅ | ✅ | ❌ |
| UPSERT | ✅ | ✅ | ✅ |
| JSON | JSONB | TEXT | JSON |
| UUID | Native | TEXT | CHAR(36) |

## Examples

See the [`examples/`](examples/) directory:

- [`minimal/`](examples/minimal/) - Basic usage
- [`production/`](examples/production/) - Full-featured app

## License

MIT License - see [LICENSE](LICENSE) for details.
