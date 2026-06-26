package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"

	"github.com/v1truv1us/audiofile/backend/internal/auth"
	"github.com/v1truv1us/audiofile/backend/internal/billing"
	"github.com/v1truv1us/audiofile/backend/internal/collection"
	"github.com/v1truv1us/audiofile/backend/internal/profiles"
	"github.com/v1truv1us/audiofile/backend/internal/releases"
	"github.com/v1truv1us/audiofile/backend/internal/wishlist"
)

func main() {
	_ = godotenv.Load()

	// Initialize Sentry
	sentryEnabled := false
	if dsn := os.Getenv("SENTRY_DSN"); dsn != "" {
		err := sentry.Init(sentry.ClientOptions{
			Dsn:              dsn,
			EnableTracing:    true,
			TracesSampleRate: 0.2,
			Environment:      os.Getenv("GO_ENV"),
			Release:          "audiofile@0.2.0",
		})
		if err != nil {
			log.Printf("Sentry init failed: %v", err)
		} else {
			sentryEnabled = true
			defer sentry.Flush(2 * time.Second)
			log.Println("Sentry initialized")
		}
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://postgres:postgres@localhost:54322/postgres"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	log.Println("Connected to Postgres")

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	if sentryEnabled {
		r.Use(sentryhttp.New(sentryhttp.Options{}).Handle)
	}
	r.Use(corsMiddleware)

	supabaseURL := os.Getenv("SUPABASE_URL")
	if supabaseURL == "" {
		supabaseURL = "https://bwzldaesynlruqukbnej.supabase.co"
		log.Println("Warning: SUPABASE_URL not set, using default project URL")
	}

	r.Route("/api", func(r chi.Router) {
		// Public routes
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "ok",
				"version": "0.2.0",
			})
		})
		r.Mount("/releases", func() chi.Router {
			discogs := releases.NewDiscogsClient("", os.Getenv("DISCOGS_CONSUMER_KEY"), os.Getenv("DISCOGS_CONSUMER_SECRET"))
			mb := releases.NewMusicBrainzClient("")
			searchers := []releases.ReleaseSearcher{}
			if discogs != nil {
				searchers = append(searchers, discogs)
			}
			searchers = append(searchers, mb)
			h := releases.NewHandler(searchers...)
			h.SetDiscogs(discogs)
			return h.Routes()
		}())
		r.Mount("/public/collection", collection.NewHandler(pool).PublicRoutes())
		r.Mount("/public/wishlist", wishlist.NewHandler(pool).PublicRoutes())

		// Billing webhook (outside auth — uses Paddle signature verification)
		var paddleClient billing.PaddleClient
		paddleEnvironment := os.Getenv("PADDLE_ENVIRONMENT")
		if paddleEnvironment == "" {
			paddleEnvironment = "sandbox"
		}
		if apiKey := os.Getenv("PADDLE_API_KEY"); apiKey != "" {
			appBaseURL := os.Getenv("APP_BASE_URL")
			if appBaseURL == "" {
				appBaseURL = "https://audiofile.app"
			}
			paddleClient = billing.NewPaddleClient(apiKey, paddleEnvironment, appBaseURL)
			log.Printf("Paddle client initialized (environment: %s)", paddleEnvironment)
		} else {
			paddleClient = &billing.NoOpPaddleClient{}
			log.Println("Paddle API key not set; billing endpoints will return errors (no-op client)")
		}
		billingHandler := billing.NewHandler(pool, paddleClient)

		webhookSecret := os.Getenv("PADDLE_WEBHOOK_SECRET")
		if webhookSecret == "" {
			if paddleEnvironment == "sandbox" {
				log.Println("Warning: PADDLE_WEBHOOK_SECRET is empty; webhook signature verification is disabled in sandbox")
			} else {
				log.Printf("Warning: PADDLE_WEBHOOK_SECRET is empty; webhooks will be rejected in %s environment", paddleEnvironment)
			}
		}

		r.Post("/billing/webhook", billingHandler.Webhook)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(supabaseURL))
			r.Mount("/collection", collection.NewHandler(pool).Routes())
			r.Mount("/profiles", profiles.NewHandler(pool).Routes())
			r.Mount("/wishlist", wishlist.NewHandler(pool).Routes())
			r.Mount("/billing", billingHandler.Routes())
			r.Mount("/admin/billing", billingHandler.AdminRoutes())
		})
	})

	// Serve frontend static files (embedded at build time via Docker)
	staticDir := os.Getenv("STATIC_DIR")
	if staticDir == "" {
		staticDir = "./frontend/dist"
	}
	if _, err := os.Stat(staticDir); err == nil {
		fileServer := http.FileServer(http.Dir(staticDir))
		r.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// Try exact file first, then fall back to index.html for SPA routing
			fp := filepath.Join(staticDir, filepath.Clean(req.URL.Path))
			if _, err := os.Stat(fp); err != nil {
				req.URL.Path = "/"
			}
			fileServer.ServeHTTP(w, req)
		}))
		log.Printf("Serving frontend from %s", staticDir)
	} else {
		log.Printf("No frontend dist found at %s (API-only mode)", staticDir)
	}

	host := os.Getenv("HOST")
	if host == "" {
		host = "0.0.0.0"
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("AudioFile API listening on %s:%s", host, port)
	if err := http.ListenAndServe(host+":"+port, r); err != nil {
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
