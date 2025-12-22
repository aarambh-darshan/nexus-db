# Nexus-DB Roadmap

## Current Version: 0.2.0 (Alpha)

### ✅ Completed

**Core**
- Schema definition (Go API + DSL parser)
- Multi-dialect support (PostgreSQL, SQLite, MySQL)
- Fluent query builder (SELECT, INSERT, UPDATE, DELETE)
- JOINs, GROUP BY, HAVING, ORDER BY, LIMIT/OFFSET
- Transaction support with rollback
- RETURNING clause, UPSERT (ON CONFLICT)

**CLI**
- `nexus init` - Project scaffolding
- `nexus migrate new/up/down/status/reset`
- `nexus gen` - Code generation

**Infrastructure**
- Connection pooling
- Integration tests
- Examples (minimal + production REST API)

---

## ✅ v0.2.0 - Query Enhancements

- [x] Raw SQL queries with parameter binding
- [x] Subquery support (WhereIn, WhereExists, derived tables)
- [x] UNION/INTERSECT/EXCEPT
- [x] Common Table Expressions (CTEs)
- [x] Prepared statements caching
- [x] Query logging with timing

---

## 🔮 v0.3.0 - Relations & Eager Loading

- [x] Auto-detect relations from schema
- [x] Eager loading (Include/Preload)
- [x] Lazy loading with proxy
- [x] Cascade delete/update
- [x] Many-to-many with junction tables

---

## 🔮 v0.4.0 - Advanced Migrations

- [x] Schema diff detection (auto-generate migrations)
- [x] Migration squashing
- [x] Seed data support
- [x] Rollback to specific version
- [x] Migration locking (prevent concurrent runs)
- [x] SQL migration validation

---

## 🔮 v0.5.0 - Developer Experience

- [ ] `nexus dev` - Watch mode with hot reload
- [ ] `nexus studio` - Database browser UI
- [ ] Better error messages with suggestions
- [ ] Query plan analysis
- [ ] Performance profiler

---

## 🔮 v1.0.0 - Production Ready

- [ ] Full test coverage (>80%)
- [ ] Comprehensive documentation
- [ ] Benchmarks vs sqlx/gorm/ent
- [ ] Connection pool tuning guide
- [ ] Security audit
- [ ] Stable API guarantee

---

## Future Ideas

- **Plugins**: Custom validators, hooks, middleware
- **GraphQL**: Auto-generate GraphQL schema from models
- **REST**: Auto-generate REST API from schema
- **Caching**: Redis/Memcached query cache integration
- **Multi-tenant**: Built-in tenant isolation
- **Sharding**: Horizontal scaling support
- **Bindings**: WASM/FFI for use in other languages

---

## Contributing

Want to help? Pick an item from the roadmap and open a PR!

1. Fork the repository
2. Create a feature branch
3. Submit a pull request

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.
