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

# Apply migrations
nexus migrate up

# Rollback
nexus migrate down

# Check status
nexus migrate status

# Generate Go types
nexus gen
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
