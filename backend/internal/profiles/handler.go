package profiles

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/v1truv1us/audiofile/backend/internal/auth"
)

// usernamePattern mirrors the character class used by handle_new_user()
// in migration 00002 ([^a-z0-9_] stripped) and the lowercased unique index.
var usernamePattern = regexp.MustCompile(`^[a-z0-9_]+$`)

const (
	usernameMinLen = 3
	usernameMaxLen = 20
)

// validateUsername normalizes and validates a user-provided username.
// Rules: lowercase, [a-z0-9_] only, 3-20 chars. Returns the cleaned value.
func validateUsername(raw string) (string, error) {
	u := strings.ToLower(strings.TrimSpace(raw))
	if len(u) < usernameMinLen || len(u) > usernameMaxLen {
		return "", errors.New("username must be 3-20 characters")
	}
	if !usernamePattern.MatchString(u) {
		return "", errors.New("username may only contain lowercase letters, numbers, and underscores")
	}
	return u, nil
}

type dbPool interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Handler struct {
	pool dbPool
}

type ProfileSearchResult struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

// Profile is the caller's own profile shape returned by /me and updated by PUT /me.
type Profile struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type UpdateProfileRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

func NewHandler(pool dbPool) *Handler {
	return &Handler{pool: pool}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/me", h.getMe)
	r.Put("/me", h.updateMe)
	r.Get("/search", h.search)
	return r
}

func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	var p Profile
	err := h.pool.QueryRow(r.Context(), `
		SELECT id::text, username, display_name
		FROM public.profiles
		WHERE id = $1`, callerID).Scan(&p.ID, &p.Username, &p.DisplayName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// The caller authenticated but has no profile row. Return an empty
			// profile rather than 500 so the UI can still prompt them to set one.
			p = Profile{ID: callerID}
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func (h *Handler) updateMe(w http.ResponseWriter, r *http.Request) {
	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	username, err := validateUsername(req.Username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	// display_name is optional; cap its length to keep it sane.
	displayName := strings.TrimSpace(req.DisplayName)
	if len(displayName) > 50 {
		http.Error(w, "display name must be 50 characters or fewer", http.StatusBadRequest)
		return
	}

	callerID := auth.UserID(r.Context())
	var p Profile
	err = h.pool.QueryRow(r.Context(), `
		UPDATE public.profiles
		SET username = $1, display_name = $2, updated_at = now()
		WHERE id = $3
		RETURNING id::text, username, display_name`, username, displayName, callerID).Scan(&p.ID, &p.Username, &p.DisplayName)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			http.Error(w, "that username is taken", http.StatusConflict)
			return
		}
		if errors.Is(err, pgx.ErrNoRows) {
			// No profile row for this user (pre-trigger account). Create one
			// so editing still works instead of erroring.
			err = h.pool.QueryRow(r.Context(), `
				INSERT INTO public.profiles (id, username, display_name)
				VALUES ($1, $2, $3)
				ON CONFLICT (id) DO UPDATE
				SET username = EXCLUDED.username, display_name = EXCLUDED.display_name, updated_at = now()
				RETURNING id::text, username, display_name`, callerID, username, displayName).Scan(&p.ID, &p.Username, &p.DisplayName)
			if err != nil {
				var pgErr2 *pgconn.PgError
				if errors.As(err, &pgErr2) && pgErr2.Code == "23505" {
					http.Error(w, "that username is taken", http.StatusConflict)
					return
				}
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "query must be at least 2 characters"})
		return
	}

	callerID := auth.UserID(r.Context())
	rows, err := h.pool.Query(r.Context(), `
		SELECT id::text, username, display_name
		FROM public.profiles
		WHERE lower(username) LIKE lower('%'||$1||'%') AND id <> $2
		ORDER BY username
		LIMIT 10`, q, callerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	results := []ProfileSearchResult{}
	for rows.Next() {
		var result ProfileSearchResult
		if err := rows.Scan(&result.ID, &result.Username, &result.DisplayName); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		results = append(results, result)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
