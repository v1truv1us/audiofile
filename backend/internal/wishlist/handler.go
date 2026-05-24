package wishlist

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
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

	rows, err := h.pool.Query(r.Context(), `
		SELECT w.id::text, w.priority, w.target_price, w.pressing_notes,
		       COALESCE(r.title, w.manual_title, '') AS title,
		       COALESCE(r.artist, w.manual_artist, '') AS artist,
		       COALESCE(r.label, '') AS label
		FROM public.wishlist_items w
		LEFT JOIN public.releases r ON r.id = w.release_id
		ORDER BY w.priority ASC, w.created_at DESC
		LIMIT $1`, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		it.TargetPrice = price
		if notes != nil {
			it.Notes = *notes
		}
		items = append(items, it)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}
