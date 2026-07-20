package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/v1truv1us/audiofile/backend/internal/auth"
)

type dbPool interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type NotificationActor struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

type Notification struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Actor     NotificationActor `json:"actor"`
	Data      json.RawMessage   `json:"data"`
	ReadAt    *string           `json:"readAt"`
	CreatedAt string            `json:"createdAt"`
}

type Handler struct {
	pool dbPool
}

func NewHandler(pool dbPool) *Handler {
	return &Handler{pool: pool}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/", h.list)
	r.Get("/unread-count", h.unreadCount)
	r.Post("/read-all", h.markAllRead)
	r.Post("/{id}/read", h.markRead)
	return r
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	rows, err := h.pool.Query(r.Context(), `
		SELECT n.id::text, n.type, n.actor_id::text, p.username, p.display_name, n.data, n.read_at, n.created_at
		FROM public.notifications n
		JOIN public.profiles p ON p.id = n.actor_id
		WHERE n.user_id = $1
		ORDER BY n.created_at DESC
		LIMIT 50`, callerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	notifications := []Notification{}
	for rows.Next() {
		var n Notification
		var username, displayName sql.NullString
		var readAt *time.Time
		var createdAt time.Time
		if err := rows.Scan(&n.ID, &n.Type, &n.Actor.ID, &username, &displayName, &n.Data, &readAt, &createdAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		n.Actor.Username = username.String
		n.Actor.DisplayName = displayName.String
		if readAt != nil {
			formatted := readAt.Format(time.RFC3339)
			n.ReadAt = &formatted
		}
		n.CreatedAt = createdAt.Format(time.RFC3339)
		notifications = append(notifications, n)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notifications)
}

func (h *Handler) unreadCount(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())
	var count int
	if err := h.pool.QueryRow(r.Context(), `
		SELECT COUNT(*)
		FROM public.notifications
		WHERE user_id = $1 AND read_at IS NULL`, callerID).Scan(&count); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"count": count})
}

func (h *Handler) markRead(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	callerID := auth.UserID(r.Context())

	tag, err := h.pool.Exec(r.Context(), `
		UPDATE public.notifications
		SET read_at = now()
		WHERE id = $1 AND user_id = $2 AND read_at IS NULL`, id, callerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tag.RowsAffected() == 0 {
		http.Error(w, "notification not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) markAllRead(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserID(r.Context())

	if _, err := h.pool.Exec(r.Context(), `
		UPDATE public.notifications
		SET read_at = now()
		WHERE user_id = $1 AND read_at IS NULL`, callerID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
