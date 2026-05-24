package wishlist

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
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

type Handler struct {
	db *sql.DB
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}

	rows, err := h.db.Query(`
		SELECT w.id, w.priority, w.target_price, w.pressing_notes,
		       COALESCE(r.title, w.manual_title, '') AS title,
		       COALESCE(r.artist, w.manual_artist, '') AS artist,
		       COALESCE(r.label, '') AS label
		FROM wishlist_items w
		LEFT JOIN releases r ON r.id = w.release_id
		ORDER BY w.priority ASC, w.created_at DESC
		LIMIT ?`, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	items := []WishlistItem{}
	for rows.Next() {
		var it WishlistItem
		var price sql.NullFloat64
		var notes sql.NullString

		if err := rows.Scan(
			&it.ID, &it.Priority, &price, &notes,
			&it.Title, &it.Artist, &it.Label,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if price.Valid {
			it.TargetPrice = &price.Float64
		}
		it.Notes = notes.String
		items = append(items, it)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}
