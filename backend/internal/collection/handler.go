package collection

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/v1truv1us/cratekeeper/backend/internal/auth"
)

type Release struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist"`
	Year     int    `json:"year"`
	Label    string `json:"label"`
	CoverURL string `json:"coverUrl,omitempty"`
}

type CollectionItem struct {
	ID              string   `json:"id"`
	Release         Release  `json:"release"`
	MediaCondition  string   `json:"mediaCondition"`
	SleeveCondition string   `json:"sleeveCondition"`
	PurchasePrice   *float64 `json:"purchasePrice,omitempty"`
	Notes           string   `json:"notes,omitempty"`
	IsForSale       bool     `json:"isForSale"`
}

type dbPool interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Handler struct {
	pool        dbPool
	coverFinder func(ctx context.Context, title, artist string) *string
}

type CreateCollectionItemRequest struct {
	Title           string   `json:"title"`
	Artist          string   `json:"artist"`
	Year            *int     `json:"year"`
	Label           string   `json:"label"`
	MediaCondition  string   `json:"mediaCondition"`
	SleeveCondition string   `json:"sleeveCondition"`
	PurchasePrice   *float64 `json:"purchasePrice"`
	Notes           string   `json:"notes"`
	CoverURL        string   `json:"coverUrl"`
}

func NewHandler(pool dbPool) *Handler {
	return &Handler{pool: pool, coverFinder: findCoverURL}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Put("/{id}", h.update)
	r.Delete("/{id}", h.delete)
	r.Post("/backfill-covers", h.backfillCovers)
	r.Get("/stats", h.stats)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/condition", h.listCondition)
		r.Post("/condition", h.createCondition)
	})
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	sort := r.URL.Query().Get("sort")
	orderBy := "ci.created_at DESC"
	switch sort {
	case "artist":
		orderBy = "r.artist ASC"
	case "year":
		orderBy = "r.year ASC"
	case "condition":
		orderBy = "ci.media_condition ASC"
	}

	// Filter by authenticated user
	userID := auth.UserID(r.Context())

	rows, err := h.pool.Query(r.Context(), `
		SELECT ci.id::text, ci.media_condition, ci.sleeve_condition,
		       ci.purchase_price, ci.notes, ci.is_for_sale,
		       r.id::text, r.title, r.artist, r.year, r.label, r.cover_url
		FROM public.collection_items ci
		JOIN public.releases r ON r.id = ci.release_id
		WHERE ci.user_id = $1
		ORDER BY `+orderBy+`
		LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := []CollectionItem{}
	for rows.Next() {
		var it CollectionItem
		var year *int
		var coverURL, sleeveCond, notes *string
		var price *float64
		var forSale bool

		if err := rows.Scan(
			&it.ID, &it.MediaCondition, &sleeveCond,
			&price, &notes, &forSale,
			&it.Release.ID, &it.Release.Title, &it.Release.Artist, &year, &it.Release.Label, &coverURL,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		it.PurchasePrice = price
		it.Notes = derefStr(notes)
		it.SleeveCondition = derefStr(sleeveCond)
		it.IsForSale = forSale
		if year != nil {
			it.Release.Year = *year
		}
		it.Release.CoverURL = derefStr(coverURL)
		items = append(items, it)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req CreateCollectionItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Title == "" || req.Artist == "" {
		http.Error(w, "title and artist are required", http.StatusBadRequest)
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

	var releaseID string
	coverURL := optionalString(req.CoverURL)
	if coverURL == nil {
		coverURL = findCoverURL(r.Context(), req.Title, req.Artist)
	}
	if err := tx.QueryRow(r.Context(), `
		INSERT INTO public.releases (title, artist, year, label, cover_url)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text`, req.Title, req.Artist, req.Year, req.Label, coverURL).Scan(&releaseID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var itemID string
	if err := tx.QueryRow(r.Context(), `
		INSERT INTO public.collection_items (user_id, release_id, media_condition, sleeve_condition, purchase_price, notes)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id::text`, userID, releaseID, req.MediaCondition, req.SleeveCondition, req.PurchasePrice, req.Notes).Scan(&itemID); err != nil {
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

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	userID := auth.UserID(r.Context())

	// Verify ownership
	var ownerCheck string
	if err := h.pool.QueryRow(r.Context(),
		"SELECT id::text FROM public.collection_items WHERE id = $1 AND user_id = $2",
		id, userID).Scan(&ownerCheck); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var req struct {
		MediaCondition  string   `json:"mediaCondition"`
		SleeveCondition string   `json:"sleeveCondition"`
		PurchasePrice   *float64 `json:"purchasePrice"`
		Notes           string   `json:"notes"`
		IsForSale       *bool    `json:"isForSale"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	_, err := h.pool.Exec(r.Context(), `
		UPDATE public.collection_items
		SET media_condition = COALESCE(NULLIF($1, ''), media_condition),
		    sleeve_condition = COALESCE(NULLIF($2, ''), sleeve_condition),
		    purchase_price = $3,
		    notes = $4,
		    is_for_sale = COALESCE($5, is_for_sale),
		    updated_at = now()
		WHERE id = $6 AND user_id = $7`,
		req.MediaCondition, req.SleeveCondition,
		req.PurchasePrice, req.Notes, req.IsForSale,
		id, userID)
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

	tag, err := h.pool.Exec(r.Context(),
		"DELETE FROM public.collection_items WHERE id = $1 AND user_id = $2",
		id, userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())

	var collectionCount, forSaleCount, wishlistCount int
	var totalValue *float64

	h.pool.QueryRow(r.Context(), "SELECT COUNT(*) FROM public.collection_items WHERE user_id = $1", userID).Scan(&collectionCount)
	h.pool.QueryRow(r.Context(), "SELECT COUNT(*) FROM public.collection_items WHERE user_id = $1 AND is_for_sale = true", userID).Scan(&forSaleCount)
	h.pool.QueryRow(r.Context(), "SELECT SUM(purchase_price) FROM public.collection_items WHERE user_id = $1", userID).Scan(&totalValue)
	h.pool.QueryRow(r.Context(), "SELECT COUNT(*) FROM public.wishlist_items WHERE user_id = $1", userID).Scan(&wishlistCount)

	tv := 0.0
	if totalValue != nil {
		tv = *totalValue
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"collectionCount": collectionCount,
		"forSaleCount":    forSaleCount,
		"wishlistCount":   wishlistCount,
		"totalValue":      tv,
	})
}

func (h *Handler) backfillCovers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), `
		SELECT id::text, title, artist
		FROM public.releases
		WHERE cover_url IS NULL OR cover_url = ''
		ORDER BY created_at ASC
		LIMIT 25`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	checked := 0
	updated := 0
	for rows.Next() {
		var id, title, artist string
		if err := rows.Scan(&id, &title, &artist); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		checked++
		coverURL := h.coverFinder(r.Context(), title, artist)
		if coverURL == nil {
			continue
		}
		if _, err := h.pool.Exec(r.Context(), `UPDATE public.releases SET cover_url = $1, updated_at = now() WHERE id = $2`, coverURL, id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		updated++
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"checked": checked, "updated": updated})
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func optionalString(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

type musicBrainzReleaseSearch struct {
	Releases []struct {
		ID string `json:"id"`
	} `json:"releases"`
}

type coverCacheEntry struct {
	coverURL *string
	expires  time.Time
}

var coverCache = struct {
	sync.Mutex
	items map[string]coverCacheEntry
}{items: map[string]coverCacheEntry{}}

var coverHTTPClient = &http.Client{Timeout: 5 * time.Second}
var musicBrainzCoverBaseURL = "https://musicbrainz.org"
var coverArtBaseURL = "https://coverartarchive.org"

func findCoverURL(ctx context.Context, title, artist string) *string {
	key := strings.ToLower(strings.TrimSpace(title) + "\x00" + strings.TrimSpace(artist))
	now := time.Now()
	coverCache.Lock()
	entry, ok := coverCache.items[key]
	if ok && now.Before(entry.expires) {
		coverCache.Unlock()
		return entry.coverURL
	}
	coverCache.Unlock()

	query := fmt.Sprintf(`release:"%s" AND artist:"%s"`, title, artist)
	reqURL := musicBrainzCoverBaseURL + "/ws/2/release/?fmt=json&limit=1&query=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "AudioFile/0.2.0 (https://github.com/v1truv1us/record-keeper)")

	res, err := coverHTTPClient.Do(req)
	if err != nil || res.StatusCode != http.StatusOK {
		cacheCoverURL(key, nil, now)
		return nil
	}
	defer res.Body.Close()

	var search musicBrainzReleaseSearch
	if err := json.NewDecoder(res.Body).Decode(&search); err != nil || len(search.Releases) == 0 || search.Releases[0].ID == "" {
		cacheCoverURL(key, nil, now)
		return nil
	}

	coverURL := coverArtBaseURL + "/release/" + search.Releases[0].ID + "/front-500"
	coverReq, err := http.NewRequestWithContext(ctx, http.MethodHead, coverURL, nil)
	if err != nil {
		return nil
	}
	coverRes, err := coverHTTPClient.Do(coverReq)
	if err != nil {
		cacheCoverURL(key, nil, now)
		return nil
	}
	coverRes.Body.Close()
	if coverRes.StatusCode >= http.StatusBadRequest {
		cacheCoverURL(key, nil, now)
		return nil
	}
	cacheCoverURL(key, &coverURL, now)
	return &coverURL
}

func cacheCoverURL(key string, coverURL *string, now time.Time) {
	ttl := 24 * time.Hour
	if coverURL == nil {
		ttl = time.Hour
	}
	coverCache.Lock()
	coverCache.items[key] = coverCacheEntry{coverURL: coverURL, expires: now.Add(ttl)}
	coverCache.Unlock()
}

// Keep ctx alias for clarity in method signatures
var _ = context.Background

type ConditionEntry struct {
	ID              string `json:"id"`
	MediaCondition  string `json:"mediaCondition"`
	SleeveCondition string `json:"sleeveCondition"`
	WarpNotes       string `json:"warpNotes,omitempty"`
	ScratchNotes    string `json:"scratchNotes,omitempty"`
	CleaningNotes   string `json:"cleaningNotes,omitempty"`
	PlaybackNotes   string `json:"playbackNotes,omitempty"`
	CreatedAt       string `json:"createdAt"`
}

type CreateConditionRequest struct {
	MediaCondition  string `json:"mediaCondition"`
	SleeveCondition string `json:"sleeveCondition"`
	WarpNotes       string `json:"warpNotes"`
	ScratchNotes    string `json:"scratchNotes"`
	CleaningNotes   string `json:"cleaningNotes"`
	PlaybackNotes   string `json:"playbackNotes"`
}

func (h *Handler) listCondition(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")
	userID := auth.UserID(r.Context())

	// Verify ownership
	var ownerCheck string
	if err := h.pool.QueryRow(r.Context(),
		"SELECT id::text FROM public.collection_items WHERE id = $1 AND user_id = $2",
		itemID, userID).Scan(&ownerCheck); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	rows, err := h.pool.Query(r.Context(), `
		SELECT id::text, media_condition, sleeve_condition,
		       COALESCE(warp_notes, ''), COALESCE(scratch_notes, ''),
		       COALESCE(cleaning_notes, ''), COALESCE(playback_notes, ''),
		       created_at
		FROM public.condition_history
		WHERE collection_item_id = $1
		ORDER BY created_at DESC`, itemID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	entries := []ConditionEntry{}
	for rows.Next() {
		var e ConditionEntry
		if err := rows.Scan(&e.ID, &e.MediaCondition, &e.SleeveCondition,
			&e.WarpNotes, &e.ScratchNotes, &e.CleaningNotes, &e.PlaybackNotes,
			&e.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		entries = append(entries, e)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (h *Handler) createCondition(w http.ResponseWriter, r *http.Request) {
	itemID := chi.URLParam(r, "id")
	userID := auth.UserID(r.Context())

	// Verify ownership
	var ownerCheck string
	if err := h.pool.QueryRow(r.Context(),
		"SELECT id::text FROM public.collection_items WHERE id = $1 AND user_id = $2",
		itemID, userID).Scan(&ownerCheck); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var req CreateConditionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.MediaCondition == "" {
		req.MediaCondition = "VG"
	}
	if req.SleeveCondition == "" {
		req.SleeveCondition = "VG"
	}

	var entryID string
	if err := h.pool.QueryRow(r.Context(), `
		INSERT INTO public.condition_history
			(collection_item_id, media_condition, sleeve_condition, warp_notes, scratch_notes, cleaning_notes, playback_notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id::text`,
		itemID, req.MediaCondition, req.SleeveCondition,
		optionalString(req.WarpNotes), optionalString(req.ScratchNotes),
		optionalString(req.CleaningNotes), optionalString(req.PlaybackNotes)).Scan(&entryID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": entryID})
}
