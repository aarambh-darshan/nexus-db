// Package studio provides the database browser web UI server.
package studio

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nexus-db/nexus/pkg/core/migration"
	"github.com/nexus-db/nexus/pkg/core/schema"
	"github.com/nexus-db/nexus/pkg/dialects"
)

// Server represents the studio web server.
type Server struct {
	conn       *dialects.Connection
	schema     *schema.Schema
	mux        *http.ServeMux
	port       int
	host       string
	migrations *migration.Engine
}

// Config holds the server configuration.
type Config struct {
	Port       int
	Host       string
	Connection *dialects.Connection
	Schema     *schema.Schema
	Migrations *migration.Engine
}

// NewServer creates a new studio server.
func NewServer(cfg Config) *Server {
	s := &Server{
		conn:       cfg.Connection,
		schema:     cfg.Schema,
		port:       cfg.Port,
		host:       cfg.Host,
		mux:        http.NewServeMux(),
		migrations: cfg.Migrations,
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures all HTTP routes.
func (s *Server) setupRoutes() {
	// API routes
	s.mux.HandleFunc("/api/tables", s.handleTables)
	s.mux.HandleFunc("/api/tables/", s.handleTableDetails)
	s.mux.HandleFunc("/api/query", s.handleQuery)
	s.mux.HandleFunc("/api/schema", s.handleSchema)
	s.mux.HandleFunc("/api/migrations", s.handleMigrations)
	s.mux.HandleFunc("/api/info", s.handleInfo)

	// Serve static files (embedded SvelteKit build)
	s.mux.HandleFunc("/", s.handleStatic)
}

// Addr returns the server address.
func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.host, s.port)
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	server := &http.Server{
		Addr:         s.Addr(),
		Handler:      s.corsMiddleware(s.mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	return server.ListenAndServe()
}

// StartWithContext starts the server with graceful shutdown support.
func (s *Server) StartWithContext(ctx context.Context) error {
	server := &http.Server{
		Addr:         s.Addr(),
		Handler:      s.corsMiddleware(s.mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	return server.ListenAndServe()
}

// corsMiddleware adds CORS headers for development.
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleTables returns a list of all tables.
func (s *Server) handleTables(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tables, err := s.getTables()
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]interface{}{
		"tables": tables,
	})
}

// handleTableDetails handles requests for specific table details and data.
func (s *Server) handleTableDetails(w http.ResponseWriter, r *http.Request) {
	// Extract table name from path: /api/tables/{name} or /api/tables/{name}/data
	path := strings.TrimPrefix(r.URL.Path, "/api/tables/")
	parts := strings.Split(path, "/")
	tableName := parts[0]

	if tableName == "" {
		http.Error(w, "Table name required", http.StatusBadRequest)
		return
	}

	if len(parts) > 1 && parts[1] == "data" {
		s.handleTableData(w, r, tableName)
		return
	}

	// Return table schema
	s.handleTableSchema(w, r, tableName)
}

// handleTableSchema returns the schema for a specific table.
func (s *Server) handleTableSchema(w http.ResponseWriter, r *http.Request, tableName string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	columns, err := s.getTableColumns(tableName)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]interface{}{
		"name":    tableName,
		"columns": columns,
	})
}

// handleTableData returns paginated data for a specific table.
func (s *Server) handleTableData(w http.ResponseWriter, r *http.Request, tableName string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse pagination parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 50
	}
	offset := (page - 1) * limit

	// Get total count
	total, err := s.getTableRowCount(tableName)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get data
	rows, columns, err := s.getTableData(tableName, limit, offset)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]interface{}{
		"data":    rows,
		"columns": columns,
		"total":   total,
		"page":    page,
		"limit":   limit,
		"pages":   (total + limit - 1) / limit,
	})
}

// handleQuery executes a SQL query.
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Query) == "" {
		s.jsonError(w, "Query is required", http.StatusBadRequest)
		return
	}

	start := time.Now()
	rows, columns, rowsAffected, err := s.executeQuery(req.Query)
	duration := time.Since(start)

	if err != nil {
		s.jsonResponse(w, map[string]interface{}{
			"error":    err.Error(),
			"duration": duration.Milliseconds(),
		})
		return
	}

	s.jsonResponse(w, map[string]interface{}{
		"data":         rows,
		"columns":      columns,
		"rowsAffected": rowsAffected,
		"duration":     duration.Milliseconds(),
	})
}

// handleSchema returns the full schema.
func (s *Server) handleSchema(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.schema != nil {
		models := make([]map[string]interface{}, 0)
		for _, model := range s.schema.GetModels() {
			fields := make([]map[string]interface{}, 0)
			for _, field := range model.GetFields() {
				fields = append(fields, map[string]interface{}{
					"name":       field.Name,
					"type":       field.Type.String(),
					"nullable":   field.Nullable,
					"primaryKey": field.PrimaryKey,
					"unique":     field.Unique,
					"default":    field.Default,
				})
			}

			relations := make([]map[string]interface{}, 0)
			for _, rel := range model.Relations {
				relations = append(relations, map[string]interface{}{
					"type":        relationTypeString(rel.Type),
					"targetModel": rel.TargetModel,
					"foreignKey":  rel.ForeignKey,
				})
			}

			models = append(models, map[string]interface{}{
				"name":      model.Name,
				"fields":    fields,
				"relations": relations,
			})
		}

		s.jsonResponse(w, map[string]interface{}{
			"models": models,
		})
		return
	}

	// Fall back to introspection
	tables, _ := s.getTables()
	s.jsonResponse(w, map[string]interface{}{
		"tables": tables,
	})
}

// handleMigrations returns migration status.
func (s *Server) handleMigrations(w http.ResponseWriter, r *http.Request) {
	if s.migrations == nil {
		s.jsonResponse(w, map[string]interface{}{
			"error": "Migrations not configured",
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		status, err := s.migrations.Status(r.Context())
		if err != nil {
			s.jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		migrations := make([]map[string]interface{}, 0)
		for _, m := range status {
			migrations = append(migrations, map[string]interface{}{
				"id":        m.ID,
				"name":      m.Name,
				"applied":   m.Applied,
				"appliedAt": m.AppliedAt,
			})
		}

		s.jsonResponse(w, map[string]interface{}{
			"migrations": migrations,
		})

	case http.MethodPost:
		var req struct {
			Action string `json:"action"` // "up" or "down"
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.jsonError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		switch req.Action {
		case "up":
			count, err := s.migrations.Up(r.Context())
			if err != nil {
				s.jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			s.jsonResponse(w, map[string]interface{}{
				"applied": count,
				"message": fmt.Sprintf("Applied %d migration(s)", count),
			})

		case "down":
			if err := s.migrations.Down(r.Context()); err != nil {
				s.jsonError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			s.jsonResponse(w, map[string]interface{}{
				"message": "Rolled back 1 migration",
			})

		default:
			s.jsonError(w, "Invalid action", http.StatusBadRequest)
		}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleInfo returns database connection info.
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dialect := "unknown"
	if s.conn != nil && s.conn.Dialect != nil {
		dialect = s.conn.Dialect.Name()
	}

	s.jsonResponse(w, map[string]interface{}{
		"dialect": dialect,
		"version": "0.5.0",
	})
}

// handleStatic serves static files or the SPA fallback.
func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	// Try to serve from embedded files first
	if staticHandler != nil {
		staticHandler.ServeHTTP(w, r)
		return
	}

	// Fallback: serve a simple HTML page indicating studio needs to be built
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
	<title>Nexus Studio</title>
	<style>
		body { font-family: system-ui, sans-serif; background: #0f172a; color: #e2e8f0; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; }
		.container { text-align: center; }
		h1 { color: #38bdf8; }
		p { color: #94a3b8; }
		.api { background: #1e293b; padding: 1rem 2rem; border-radius: 0.5rem; margin-top: 2rem; }
		a { color: #38bdf8; }
	</style>
</head>
<body>
	<div class="container">
		<h1>🔷 Nexus Studio</h1>
		<p>The studio UI is not yet built.</p>
		<div class="api">
			<p>API is running! Try:</p>
			<p><a href="/api/tables">/api/tables</a> | <a href="/api/info">/api/info</a></p>
		</div>
	</div>
</body>
</html>`))
}

// Helper methods

func (s *Server) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (s *Server) jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func (s *Server) getTables() ([]string, error) {
	if s.conn == nil {
		return nil, fmt.Errorf("no database connection")
	}

	var query string
	switch s.conn.Dialect.Name() {
	case "sqlite":
		query = "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE '_nexus_%' ORDER BY name"
	case "postgres":
		query = "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_name NOT LIKE '_nexus_%' ORDER BY table_name"
	case "mysql":
		query = "SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name NOT LIKE '_nexus_%' ORDER BY table_name"
	default:
		return nil, fmt.Errorf("unsupported dialect")
	}

	rows, err := s.conn.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}

	return tables, rows.Err()
}

func (s *Server) getTableColumns(tableName string) ([]map[string]interface{}, error) {
	if s.conn == nil {
		return nil, fmt.Errorf("no database connection")
	}

	var query string
	switch s.conn.Dialect.Name() {
	case "sqlite":
		query = fmt.Sprintf("PRAGMA table_info(%s)", tableName)
	case "postgres":
		query = fmt.Sprintf("SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_name = '%s' ORDER BY ordinal_position", tableName)
	case "mysql":
		query = fmt.Sprintf("SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_name = '%s' AND table_schema = DATABASE() ORDER BY ordinal_position", tableName)
	default:
		return nil, fmt.Errorf("unsupported dialect")
	}

	rows, err := s.conn.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := make([]map[string]interface{}, 0)

	if s.conn.Dialect.Name() == "sqlite" {
		for rows.Next() {
			var cid int
			var name, colType string
			var notNull, pk int
			var dfltValue sql.NullString
			if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
				return nil, err
			}
			columns = append(columns, map[string]interface{}{
				"name":       name,
				"type":       colType,
				"nullable":   notNull == 0,
				"primaryKey": pk == 1,
				"default":    dfltValue.String,
			})
		}
	} else {
		for rows.Next() {
			var name, colType, nullable string
			if err := rows.Scan(&name, &colType, &nullable); err != nil {
				return nil, err
			}
			columns = append(columns, map[string]interface{}{
				"name":     name,
				"type":     colType,
				"nullable": nullable == "YES",
			})
		}
	}

	return columns, rows.Err()
}

func (s *Server) getTableRowCount(tableName string) (int, error) {
	if s.conn == nil {
		return 0, fmt.Errorf("no database connection")
	}

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", s.conn.Dialect.Quote(tableName))
	var count int
	err := s.conn.DB.QueryRow(query).Scan(&count)
	return count, err
}

func (s *Server) getTableData(tableName string, limit, offset int) ([]map[string]interface{}, []string, error) {
	if s.conn == nil {
		return nil, nil, fmt.Errorf("no database connection")
	}

	query := fmt.Sprintf("SELECT * FROM %s LIMIT %d OFFSET %d",
		s.conn.Dialect.Quote(tableName), limit, offset)

	rows, err := s.conn.DB.Query(query)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	return results, columns, rows.Err()
}

func (s *Server) executeQuery(query string) ([]map[string]interface{}, []string, int64, error) {
	if s.conn == nil {
		return nil, nil, 0, fmt.Errorf("no database connection")
	}

	// Determine if it's a SELECT or other statement
	trimmedQuery := strings.TrimSpace(strings.ToUpper(query))
	isSelect := strings.HasPrefix(trimmedQuery, "SELECT") ||
		strings.HasPrefix(trimmedQuery, "WITH") ||
		strings.HasPrefix(trimmedQuery, "PRAGMA") ||
		strings.HasPrefix(trimmedQuery, "SHOW") ||
		strings.HasPrefix(trimmedQuery, "DESCRIBE") ||
		strings.HasPrefix(trimmedQuery, "EXPLAIN")

	if isSelect {
		rows, err := s.conn.DB.Query(query)
		if err != nil {
			return nil, nil, 0, err
		}
		defer rows.Close()

		columns, err := rows.Columns()
		if err != nil {
			return nil, nil, 0, err
		}

		var results []map[string]interface{}
		for rows.Next() {
			values := make([]interface{}, len(columns))
			valuePtrs := make([]interface{}, len(columns))
			for i := range values {
				valuePtrs[i] = &values[i]
			}

			if err := rows.Scan(valuePtrs...); err != nil {
				return nil, nil, 0, err
			}

			row := make(map[string]interface{})
			for i, col := range columns {
				row[col] = values[i]
			}
			results = append(results, row)
		}

		return results, columns, int64(len(results)), rows.Err()
	}

	// Execute non-SELECT statement
	result, err := s.conn.DB.Exec(query)
	if err != nil {
		return nil, nil, 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	return nil, nil, rowsAffected, nil
}

// staticHandler is set by embed.go when static files are available.
var staticHandler http.Handler

// relationTypeString converts a RelationType to a string representation.
func relationTypeString(t schema.RelationType) string {
	switch t {
	case schema.RelationBelongsTo:
		return "BelongsTo"
	case schema.RelationHasOne:
		return "HasOne"
	case schema.RelationHasMany:
		return "HasMany"
	case schema.RelationManyToMany:
		return "ManyToMany"
	default:
		return "Unknown"
	}
}
