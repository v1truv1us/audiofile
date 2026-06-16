package wishlist

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/v1truv1us/audiofile/backend/internal/auth"
)

type WishlistItem struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Artist      string   `json:"artist"`
	Priority    int      `json:"priority"`
	TargetPrice *float64 `json:"targetPrice,omitempty"`
	Notes       string   `json:"notes,omitempty"`
	Label       string   `json:"label,omitempty"`
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

type CreateWishlistItemRequest struct {
	Title       string   `json:"title"`
	Artist      string   `json:"artist"`
	Priority    int      `json:"priority"`
	TargetPrice *float64 `json:"targetPrice"`
	Notes       string   `json:"notes"`
	Year        *int     `json:"year"`
	Label       string   `json:"label"`
	CoverURL    string   `json:"coverUrl"`
}

type UpdateWishlistItemRequest = CreateWishlistItemRequest

type PurchaseWishlistItemRequest struct {
	MediaCondition  string   `json:"mediaCondition"`
	SleeveCondition string   `json:"sleeveCondition"`
	PurchasePrice   *float64 `json:"purchasePrice"`
	Notes           string   `json:"notes"`
}

type CreateShareRequest struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

type ShareViewer struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type WishlistShare struct {
	ViewerID  string      `json:"viewerId"`
	Viewer    ShareViewer `json:"viewer"`
	Message   string      `json:"message"`
	CreatedAt string      `json:"createdAt"`
}

type SharedWishlistOwner struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type SharedWishlist struct {
	Owner SharedWishlistOwner `json:"owner"`
	Items []WishlistItem      `json:"items"`
}

func NewHandler(pool dbPool) *Handler {
	return &Handler{pool: pool}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Post("/shares", h.createShare)
	r.Get("/shares", h.listShares)
	r.Delete("/shares/{viewerID}", h.deleteShare)
	r.Get("/shared-with-me", h.sharedWithMe)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	r.Post("/{id}/purchase", h.purchase)
	return r
}

func (h *Handler) PublicRoutes() chi.Router {
	r := chi.NewRouter()
	r.Get("/{userID}", h.publicList)
	return r
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateWishlistItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Title == "" || req.Artist == "" {
		http.Error(w, "title and artist are required", http.StatusBadRequest)
		return
	}
	if req.Priority < 1 || req.Priority > 10 {
		req.Priority = 5
	}

	userID := auth.UserID(r.Context())

	var releaseID *string
	if req.Label != "" || req.CoverURL != "" || req.Year != nil {
		var id string
		coverURL := optionalString(req.CoverURL)
		if err := h.pool.QueryRow(r.Context(), `
			INSERT INTO public.releases (title, artist, year, label, cover_url)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id::text`, req.Title, req.Artist, req.Year, req.Label, coverURL).Scan(&id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		releaseID = &id
	}

	var itemID string
	if err := h.pool.QueryRow(r.Context(), `
		INSERT INTO public.wishlist_items (user_id, release_id, manual_title, manual_artist, priority, target_price, pressing_notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id::text`, userID, releaseID, req.Title, req.Artist, req.Priority, req.TargetPrice, req.Notes).Scan(&itemID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": itemID})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req UpdateWishlistItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if id == "" || req.Title == "" || req.Artist == "" {
		http.Error(w, "id, title, and artist are required", http.StatusBadRequest)
		return
	}
	if req.Priority < 1 || req.Priority > 10 {
		req.Priority = 5
	}

	userID := auth.UserID(r.Context())

	var release sql.NullString
	if err := h.pool.QueryRow(r.Context(), `
		SELECT release_id::text
		FROM public.wishlist_items
		WHERE id = $1 AND user_id = $2`, id, userID).Scan(&release); err != nil {
		http.Error(w, "wishlist item not found", http.StatusNotFound)
		return
	}
	var releaseID *string
	if release.Valid {
		releaseID = &release.String
	}

	hasReleaseMetadata := req.Label != "" || req.CoverURL != "" || req.Year != nil
	if releaseID == nil && hasReleaseMetadata {
		var newReleaseID string
		coverURL := optionalString(req.CoverURL)
		if err := h.pool.QueryRow(r.Context(), `
			INSERT INTO public.releases (title, artist, year, label, cover_url)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id::text`, req.Title, req.Artist, req.Year, req.Label, coverURL).Scan(&newReleaseID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		releaseID = &newReleaseID
	} else if releaseID != nil {
		coverURL := optionalString(req.CoverURL)
		if _, err := h.pool.Exec(r.Context(), `
			UPDATE public.releases
			SET title = $1, artist = $2, year = $3, label = $4, cover_url = COALESCE($5, cover_url), updated_at = now()
			WHERE id = $6`, req.Title, req.Artist, req.Year, req.Label, coverURL, *releaseID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	_, err := h.pool.Exec(r.Context(), `
		UPDATE public.wishlist_items
		SET release_id = $1,
		    manual_title = $2,
		    manual_artist = $3,
		    priority = $4,
		    target_price = $5,
		    pressing_notes = $6,
		    updated_at = now()
		WHERE id = $7 AND user_id = $8`, releaseID, req.Title, req.Artist, req.Priority, req.TargetPrice, req.Notes, id, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := auth.UserID(r.Context())
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}

	tag, err := h.pool.Exec(r.Context(), `
		DELETE FROM public.wishlist_items
		WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, "wishlist item not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) purchase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req PurchaseWishlistItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if id == "" {
		http.Error(w, "id is required", http.StatusBadRequest)
		return
	}
	if req.MediaCondition == "" {
		req.MediaCondition = "VG"
	}
	if req.SleeveCondition == "" {
		req.SleeveCondition = "VG"
	}

	userID := auth.UserID(r.Context())

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	var release sql.NullString
	var title, artist string
	var year *int
	var label, coverURL *string
	var wishlistNotes *string
	if err := tx.QueryRow(r.Context(), `
		SELECT w.release_id::text,
		       COALESCE(r.title, w.manual_title, '') AS title,
		       COALESCE(r.artist, w.manual_artist, '') AS artist,
		       r.year, r.label, r.cover_url, w.pressing_notes
		FROM public.wishlist_items w
		LEFT JOIN public.releases r ON r.id = w.release_id
		WHERE w.id = $1 AND w.user_id = $2`, id, userID).Scan(&release, &title, &artist, &year, &label, &coverURL, &wishlistNotes); err != nil {
		http.Error(w, "wishlist item not found", http.StatusNotFound)
		return
	}

	releaseID := release.String
	if !release.Valid {
		if err := tx.QueryRow(r.Context(), `
			INSERT INTO public.releases (title, artist, year, label, cover_url)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id::text`, title, artist, year, label, coverURL).Scan(&releaseID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	notes := req.Notes
	if notes == "" && wishlistNotes != nil {
		notes = *wishlistNotes
	}
	var itemID string
	if err := tx.QueryRow(r.Context(), `
		INSERT INTO public.collection_items (user_id, release_id, media_condition, sleeve_condition, purchase_price, notes)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id::text`, userID, releaseID, req.MediaCondition, req.SleeveCondition, req.PurchasePrice, notes).Scan(&itemID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := tx.Exec(r.Context(), `DELETE FROM public.wishlist_items WHERE id = $1 AND user_id = $2`, id, userID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": itemID})
}

func (h *Handler) createShare(w http.ResponseWriter, r *http.Request) {
	var req CreateShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}

	callerID := auth.UserID(r.Context())
	var viewerID string
	if err := h.pool.QueryRow(r.Context(), `
		SELECT id::text
		FROM public.profiles
		WHERE lower(username) = lower($1)`, req.Username).Scan(&viewerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if viewerID == callerID {
		http.Error(w, "cannot share to yourself", http.StatusBadRequest)
		return
	}

	if _, err := h.pool.Exec(r.Context(), `
		INSERT INTO public.wishlist_shares (owner_id, viewer_id, message)
		VALUES ($1, $2, $3)`, callerID, viewerID, req.Message); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			http.Error(w, "already shared", http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"viewerId": viewerID})
}

func (h *Handler) listShares(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	rows, err := h.pool.Query(r.Context(), `
		SELECT ws.viewer_id::text, p.username, p.display_name, ws.message, ws.created_at
		FROM public.wishlist_shares ws
		JOIN public.profiles p ON p.id = ws.viewer_id
		WHERE ws.owner_id = $1
		ORDER BY ws.created_at DESC`, callerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	shares := []WishlistShare{}
	for rows.Next() {
		var share WishlistShare
		var message *string
		var createdAt time.Time
		if err := rows.Scan(&share.ViewerID, &share.Viewer.Username, &share.Viewer.DisplayName, &message, &createdAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if message != nil {
			share.Message = *message
		}
		share.CreatedAt = createdAt.Format(time.RFC3339)
		shares = append(shares, share)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(shares)
}

func (h *Handler) deleteShare(w http.ResponseWriter, r *http.Request) {
	viewerID := chi.URLParam(r, "viewerID")
	callerID := auth.UserID(r.Context())

	tag, err := h.pool.Exec(r.Context(), `
		DELETE FROM public.wishlist_shares
		WHERE owner_id = $1 AND viewer_id = $2`, callerID, viewerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, "wishlist share not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) sharedWithMe(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	rows, err := h.pool.Query(r.Context(), `
		SELECT ws.owner_id::text, p.username, p.display_name
		FROM public.wishlist_shares ws
		JOIN public.profiles p ON p.id = ws.owner_id
		WHERE ws.viewer_id = $1
		ORDER BY ws.created_at DESC`, callerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	shared := []SharedWishlist{}
	for rows.Next() {
		var entry SharedWishlist
		if err := rows.Scan(&entry.Owner.ID, &entry.Owner.Username, &entry.Owner.DisplayName); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		items, err := h.queryWishlistItems(r.Context(), entry.Owner.ID, 50)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		entry.Items = items
		shared = append(shared, entry)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(shared)
}

func (h *Handler) publicList(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if userID == "" {
		http.Error(w, "userID is required", http.StatusBadRequest)
		return
	}
	h.listForUser(w, r, userID)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	h.listForUser(w, r, auth.UserID(r.Context()))
}

func (h *Handler) listForUser(w http.ResponseWriter, r *http.Request, userID string) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	items, err := h.queryWishlistItems(r.Context(), userID, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func (h *Handler) queryWishlistItems(ctx context.Context, userID string, limit int) ([]WishlistItem, error) {
	rows, err := h.pool.Query(ctx, `
		SELECT w.id::text, w.priority, w.target_price, w.pressing_notes,
		       COALESCE(r.title, w.manual_title, '') AS title,
		       COALESCE(r.artist, w.manual_artist, '') AS artist,
		       COALESCE(r.label, '') AS label
		FROM public.wishlist_items w
		LEFT JOIN public.releases r ON r.id = w.release_id
		WHERE w.user_id = $1
		ORDER BY w.priority ASC, w.created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []WishlistItem{}
	for rows.Next() {
		var it WishlistItem
		var price *float64
		var notes *string

		if err := rows.Scan(
			&it.ID, &it.Priority, &price, &notes,
			&it.Title, &it.Artist, &it.Label,
		); err != nil {
			return nil, err
		}

		it.TargetPrice = price
		if notes != nil {
			it.Notes = *notes
		}
		items = append(items, it)
	}
	return items, nil
}

func optionalString(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
