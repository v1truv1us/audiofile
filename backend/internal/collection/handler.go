package collection

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
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
	ID             string  `json:"id"`
	Release        Release `json:"release"`
	MediaCondition string  `json:"mediaCondition"`
	SleeveCondition string `json:"sleeveCondition"`
	PurchasePrice  *float64 `json:"purchasePrice,omitempty"`
	Notes          string  `json:"notes,omitempty"`
	IsForSale      bool    `json:"isForSale"`
}

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
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

	// TODO: filter by authenticated user
	rows, err := h.db.Query(`
		SELECT ci.id, ci.media_condition, ci.sleeve_condition,
		       ci.purchase_price, ci.notes, ci.is_for_sale,
		       r.id, r.title, r.artist, r.year, r.label, r.cover_url
		FROM collection_items ci
		JOIN releases r ON r.id = ci.release_id
		ORDER BY `+orderBy+`
		LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := []CollectionItem{}
	for rows.Next() {
		var it CollectionItem
		var price sql.NullFloat64
		var year sql.NullInt64
		var coverURL, sleeveCond, notes sql.NullString
		var forSale int

		if err := rows.Scan(
			&it.ID, &it.MediaCondition, &sleeveCond,
			&price, &notes, &forSale,
			&it.Release.ID, &it.Release.Title, &it.Release.Artist, &year, &it.Release.Label, &coverURL,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if price.Valid {
			it.PurchasePrice = &price.Float64
		}
		it.Notes = notes.String
		it.SleeveCondition = sleeveCond.String
		it.IsForSale = forSale == 1
		it.Release.Year = int(year.Int64)
		it.Release.CoverURL = coverURL.String
		items = append(items, it)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	var collectionCount, forSaleCount int
	var totalValue sql.NullFloat64
	var wishlistCount int

	h.db.QueryRow("SELECT COUNT(*) FROM collection_items").Scan(&collectionCount)
	h.db.QueryRow("SELECT COUNT(*) FROM collection_items WHERE is_for_sale = 1").Scan(&forSaleCount)
	h.db.QueryRow("SELECT COALESCE(SUM(purchase_price), 0) FROM collection_items").Scan(&totalValue)
	h.db.QueryRow("SELECT COUNT(*) FROM wishlist_items").Scan(&wishlistCount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"collectionCount": collectionCount,
		"forSaleCount":    forSaleCount,
		"wishlistCount":   wishlistCount,
		"totalValue":      totalValue.Float64,
	})
}
