// Production example: REST API with Nexus-DB
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"github.com/nexus-db/nexus/pkg/core/schema"
	"github.com/nexus-db/nexus/pkg/dialects"
	"github.com/nexus-db/nexus/pkg/dialects/sqlite"
	"github.com/nexus-db/nexus/pkg/query"
)

// Global connection
var conn *dialects.Connection

func main() {
	ctx := context.Background()

	// 1. Define schema
	s := schema.NewSchema()

	s.Model("User", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("email").Unique()
		m.String("name")
		m.Bool("active").Default(true)
		m.DateTime("created_at").DefaultNow()
	})

	s.Model("Post", func(m *schema.Model) {
		m.Int("id").PrimaryKey().AutoInc()
		m.String("title")
		m.Text("content").Null()
		m.Int("author_id")
		m.Bool("published").Default(false)
		m.DateTime("created_at").DefaultNow()
	})

	// Validate
	if err := s.Validate(); err != nil {
		log.Fatal(err)
	}

	// 2. Connect to SQLite (use file for persistence)
	db, err := sql.Open("sqlite3", "./production.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	dialect := sqlite.New()
	conn = dialects.NewConnection(db, dialect)

	// 3. Create tables
	for _, model := range s.GetModels() {
		sql := dialect.CreateTableSQL(model)
		if _, err := conn.Exec(ctx, sql); err != nil {
			log.Fatal(err)
		}
	}
	log.Println("Database initialized")

	// 4. Set up HTTP routes
	http.HandleFunc("/users", usersHandler)
	http.HandleFunc("/users/", userHandler)
	http.HandleFunc("/posts", postsHandler)
	http.HandleFunc("/posts/", postHandler)
	http.HandleFunc("/health", healthHandler)

	// 5. Start server
	port := ":8080"
	log.Printf("Server running on http://localhost%s", port)
	log.Println("\nAPI Endpoints:")
	log.Println("  GET    /users          - List all users")
	log.Println("  POST   /users          - Create user")
	log.Println("  GET    /users/:id      - Get user by ID")
	log.Println("  PUT    /users/:id      - Update user")
	log.Println("  DELETE /users/:id      - Delete user")
	log.Println("  GET    /posts          - List all posts")
	log.Println("  POST   /posts          - Create post")
	log.Fatal(http.ListenAndServe(port, nil))
}

// healthHandler returns server health status
func healthHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// usersHandler handles /users endpoint
func usersHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	users := query.New(conn, "User")

	switch r.Method {
	case http.MethodGet:
		// List all users
		results, err := users.Select().OrderBy("id", query.Desc).All(ctx)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}
		jsonResponse(w, results)

	case http.MethodPost:
		// Create user
		var input struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			httpError(w, err, http.StatusBadRequest)
			return
		}

		// Check if email exists
		exists, _ := users.Select().Where(query.Eq("email", input.Email)).Exists(ctx)
		if exists {
			httpError(w, fmt.Errorf("email already exists"), http.StatusConflict)
			return
		}

		id, err := users.Insert(map[string]interface{}{
			"email": input.Email,
			"name":  input.Name,
		}).LastInsertId(ctx)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}

		// Fetch created user
		user, _ := users.Select().Where(query.Eq("id", id)).One(ctx)
		w.WriteHeader(http.StatusCreated)
		jsonResponse(w, user)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// userHandler handles /users/:id endpoint
func userHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract ID from path
	idStr := strings.TrimPrefix(r.URL.Path, "/users/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		httpError(w, fmt.Errorf("invalid user ID"), http.StatusBadRequest)
		return
	}

	users := query.New(conn, "User")

	switch r.Method {
	case http.MethodGet:
		// Get user by ID
		user, err := users.Select().Where(query.Eq("id", id)).One(ctx)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}
		if user == nil {
			httpError(w, fmt.Errorf("user not found"), http.StatusNotFound)
			return
		}
		jsonResponse(w, user)

	case http.MethodPut:
		// Update user
		var input struct {
			Name   *string `json:"name"`
			Active *bool   `json:"active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			httpError(w, err, http.StatusBadRequest)
			return
		}

		updates := make(map[string]interface{})
		if input.Name != nil {
			updates["name"] = *input.Name
		}
		if input.Active != nil {
			if *input.Active {
				updates["active"] = 1
			} else {
				updates["active"] = 0
			}
		}

		if len(updates) == 0 {
			httpError(w, fmt.Errorf("no updates provided"), http.StatusBadRequest)
			return
		}

		affected, err := users.Update(updates).Where(query.Eq("id", id)).Exec(ctx)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}
		if affected == 0 {
			httpError(w, fmt.Errorf("user not found"), http.StatusNotFound)
			return
		}

		// Fetch updated user
		user, _ := users.Select().Where(query.Eq("id", id)).One(ctx)
		jsonResponse(w, user)

	case http.MethodDelete:
		// Delete user
		affected, err := users.Delete().Where(query.Eq("id", id)).Exec(ctx)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}
		if affected == 0 {
			httpError(w, fmt.Errorf("user not found"), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// postsHandler handles /posts endpoint
func postsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	posts := query.New(conn, "Post")

	switch r.Method {
	case http.MethodGet:
		// List all posts with optional filters
		q := posts.Select()

		// Filter by author_id
		if authorID := r.URL.Query().Get("author_id"); authorID != "" {
			if id, err := strconv.Atoi(authorID); err == nil {
				q = q.Where(query.Eq("author_id", id))
			}
		}

		// Filter by published
		if pub := r.URL.Query().Get("published"); pub != "" {
			if pub == "true" {
				q = q.Where(query.Eq("published", 1))
			} else {
				q = q.Where(query.Eq("published", 0))
			}
		}

		results, err := q.OrderBy("created_at", query.Desc).All(ctx)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}
		jsonResponse(w, results)

	case http.MethodPost:
		// Create post
		var input struct {
			Title    string `json:"title"`
			Content  string `json:"content"`
			AuthorID int    `json:"author_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			httpError(w, err, http.StatusBadRequest)
			return
		}

		// Verify author exists
		users := query.New(conn, "User")
		authorExists, _ := users.Select().Where(query.Eq("id", input.AuthorID)).Exists(ctx)
		if !authorExists {
			httpError(w, fmt.Errorf("author not found"), http.StatusBadRequest)
			return
		}

		id, err := posts.Insert(map[string]interface{}{
			"title":     input.Title,
			"content":   input.Content,
			"author_id": input.AuthorID,
		}).LastInsertId(ctx)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}

		post, _ := posts.Select().Where(query.Eq("id", id)).One(ctx)
		w.WriteHeader(http.StatusCreated)
		jsonResponse(w, post)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// postHandler handles /posts/:id endpoint
func postHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := strings.TrimPrefix(r.URL.Path, "/posts/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		httpError(w, fmt.Errorf("invalid post ID"), http.StatusBadRequest)
		return
	}

	posts := query.New(conn, "Post")

	switch r.Method {
	case http.MethodGet:
		post, err := posts.Select().Where(query.Eq("id", id)).One(ctx)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}
		if post == nil {
			httpError(w, fmt.Errorf("post not found"), http.StatusNotFound)
			return
		}
		jsonResponse(w, post)

	case http.MethodPut:
		var input struct {
			Title     *string `json:"title"`
			Content   *string `json:"content"`
			Published *bool   `json:"published"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			httpError(w, err, http.StatusBadRequest)
			return
		}

		updates := make(map[string]interface{})
		if input.Title != nil {
			updates["title"] = *input.Title
		}
		if input.Content != nil {
			updates["content"] = *input.Content
		}
		if input.Published != nil {
			if *input.Published {
				updates["published"] = 1
			} else {
				updates["published"] = 0
			}
		}

		if len(updates) == 0 {
			httpError(w, fmt.Errorf("no updates provided"), http.StatusBadRequest)
			return
		}

		affected, err := posts.Update(updates).Where(query.Eq("id", id)).Exec(ctx)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}
		if affected == 0 {
			httpError(w, fmt.Errorf("post not found"), http.StatusNotFound)
			return
		}

		post, _ := posts.Select().Where(query.Eq("id", id)).One(ctx)
		jsonResponse(w, post)

	case http.MethodDelete:
		affected, err := posts.Delete().Where(query.Eq("id", id)).Exec(ctx)
		if err != nil {
			httpError(w, err, http.StatusInternalServerError)
			return
		}
		if affected == 0 {
			httpError(w, fmt.Errorf("post not found"), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// Helper functions
func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func httpError(w http.ResponseWriter, err error, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
