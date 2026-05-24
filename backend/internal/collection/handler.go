package collection

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

type Handler struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Get("/stats", h.stats)
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

	// TODO: filter by authenticated user once auth is wired
	rows, err := h.pool.Query(r.Context(), `
		SELECT ci.id::text, ci.media_condition, ci.sleeve_condition,
		       ci.purchase_price, ci.notes, ci.is_for_sale,
		       r.id::text, r.title, r.artist, r.year, r.label, r.cover_url
		FROM public.collection_items ci
		JOIN public.releases r ON r.id = ci.release_id
		ORDER BY `+orderBy+`
		LIMIT $1 OFFSET $2`, limit, offset)
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

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	var collectionCount, forSaleCount, wishlistCount int
	var totalValue *float64

	h.pool.QueryRow(r.Context(), "SELECT COUNT(*) FROM public.collection_items").Scan(&collectionCount)
	h.pool.QueryRow(r.Context(), "SELECT COUNT(*) FROM public.collection_items WHERE is_for_sale = true").Scan(&forSaleCount)
	h.pool.QueryRow(r.Context(), "SELECT SUM(purchase_price) FROM public.collection_items").Scan(&totalValue)
	h.pool.QueryRow(r.Context(), "SELECT COUNT(*) FROM public.wishlist_items").Scan(&wishlistCount)

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

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Keep ctx alias for clarity in method signatures
var _ = context.Background
