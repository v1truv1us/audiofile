package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "modernc.org/sqlite"

	"github.com/v1truv1us/record-keeper/backend/internal/collection"
	"github.com/v1truv1us/record-keeper/backend/internal/seed"
	"github.com/v1truv1us/record-keeper/backend/internal/wishlist"
)

func main() {
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./cratekeeper.db"
	}

	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	schemaPath := os.Getenv("SCHEMA_PATH")
	if schemaPath == "" {
		schemaPath = "internal/db/schema.sql"
	}
	if err := applySchema(db, schemaPath); err != nil {
		log.Fatalf("failed to apply schema: %v", err)
	}

	seed.Seed(db)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(corsMiddleware)

	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"version": "0.1.0",
		})
	})

	r.Mount("/api/collection", collection.NewHandler(db).Routes())
	r.Mount("/api/wishlist", wishlist.NewHandler(db).Routes())

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("CrateKeeper API listening on :%s", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func applySchema(db *sql.DB, path string) error {
	schema, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	_, err = db.Exec(string(schema))
	return err
}
