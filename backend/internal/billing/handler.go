package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/v1truv1us/audiofile/backend/internal/auth"
)

// SignatureValidator validates a Paddle webhook signature.
// Replace with real Paddle SDK verification when the SDK is added.
type SignatureValidator func(payload []byte, header string) error

type handlerPool interface {
	dbPool
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Handler handles billing-related HTTP endpoints.
type Handler struct {
	pool        handlerPool
	paddle      PaddleClient
	validateSig SignatureValidator
}

// NewHandler creates a new billing Handler.
func NewHandler(pool handlerPool, paddle PaddleClient) *Handler {
	return &Handler{
		pool:   pool,
		paddle: paddle,
		validateSig: func(payload []byte, header string) error {
			if header == "" {
				return errors.New("missing paddle-signature header")
			}
			// Parse ts=...;h1=... format
			var ts, h1 string
			for _, part := range strings.Split(header, ";") {
				part = strings.TrimSpace(part)
				if strings.HasPrefix(part, "ts=") {
					ts = strings.TrimPrefix(part, "ts=")
				} else if strings.HasPrefix(part, "h1=") {
					h1 = strings.TrimPrefix(part, "h1=")
				}
			}
			if ts == "" || h1 == "" {
				return errors.New("invalid paddle-signature header format")
			}
			secret := os.Getenv("PADDLE_WEBHOOK_SECRET")
			if secret == "" {
				if os.Getenv("PADDLE_ENVIRONMENT") == "sandbox" {
					// In sandbox, allow unsigned webhooks for development
					return nil
				}
				return errors.New("paddle webhook secret not configured")
			}
			signedPayload := ts + ":" + string(payload)
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write([]byte(signedPayload))
			expected := hex.EncodeToString(mac.Sum(nil))
			if !hmac.Equal([]byte(expected), []byte(h1)) {
				return errors.New("signature mismatch")
			}
			return nil
		},
	}
}

// SetSignatureValidator replaces the default webhook signature validator.
func (h *Handler) SetSignatureValidator(v SignatureValidator) {
	h.validateSig = v
}

// Routes returns the authenticated billing routes.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/config", h.config)
	r.Post("/checkout", h.checkout)
	r.Post("/portal", h.portal)
	r.Get("/status", h.status)
	r.Get("/test", h.test)
	r.Post("/webhook", h.webhook)
	return r
}

// AdminRoutes returns admin-only billing routes.
func (h *Handler) AdminRoutes() chi.Router {
	r := chi.NewRouter()
	r.Post("/vip", h.setVIP)
	return r
}

type checkoutRequest struct {
	PriceID string `json:"priceId"`
}

// config returns the configured billing configuration (price IDs, client token, etc.)
func (h *Handler) config(w http.ResponseWriter, r *http.Request) {
	priceID := os.Getenv("PADDLE_PREMIUM_MONTHLY_PRICE_ID")
	environment := os.Getenv("PADDLE_ENVIRONMENT")
	if environment == "" {
		environment = "sandbox"
	}

	// Client-side token is safe to expose to the browser — used by Paddle.js
	clientToken := os.Getenv("PADDLE_CLIENT_TOKEN")

	writeJSON(w, http.StatusOK, map[string]string{
		"premiumMonthlyPriceId": priceID,
		"environment":           environment,
		"clientToken":           clientToken,
	})
}

// test verifies Paddle API connectivity
func (h *Handler) test(w http.ResponseWriter, r *http.Request) {
	if h.paddle == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "error",
			"error":  "Paddle client not configured",
		})
		return
	}

	// Try to create a transaction with a dummy price ID to test connectivity
	// This will fail with a specific error if the API key is invalid
	ctx := context.Background()
	_, err := h.paddle.CreateTransaction(ctx, "pri_test_connectivity", map[string]string{"test": "true"})
	if err != nil {
		// Check if it's an authentication error
		if strings.Contains(err.Error(), "authentication") || strings.Contains(err.Error(), "401") {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"status": "error",
				"error":  "Invalid Paddle API key",
			})
			return
		}
		// Other errors are expected (invalid price ID) but mean connectivity works
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"message": "Paddle API is reachable (expected error for test price ID)",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"message": "Paddle API connectivity successful",
	})
}

func (h *Handler) checkout(w http.ResponseWriter, r *http.Request) {
	var req checkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if strings.TrimSpace(req.PriceID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "priceId is required"})
		return
	}

	userID := auth.UserID(r.Context())

	if h.paddle == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "paddle client not configured"})
		return
	}

	customData := map[string]string{"user_id": userID}
	checkoutURL, err := h.paddle.CreateTransaction(r.Context(), req.PriceID, customData)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create checkout session"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"checkoutUrl": checkoutURL})
}

func (h *Handler) portal(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())

	var customerID string
	err := h.pool.QueryRow(r.Context(),
		"SELECT paddle_customer_id FROM public.subscriptions WHERE user_id = $1", userID).Scan(&customerID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no billing account found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to lookup subscription"})
		return
	}
	if customerID == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no billing account found"})
		return
	}

	if h.paddle == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "paddle client not configured"})
		return
	}

	portalURL, err := h.paddle.GetCustomerPortalURL(r.Context(), customerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create portal session"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"portalUrl": portalURL})
}

type limitDetail struct {
	Used       int  `json:"used"`
	Limit      int  `json:"limit"`
	IsExceeded bool `json:"isExceeded"`
}

type statusResponse struct {
	UserID           string `json:"userId"`
	Tier             string `json:"tier"`
	Status           string `json:"status"`
	CurrentPeriodEnd string `json:"currentPeriodEnd"`
	IsVIP            bool   `json:"isVip"`
	IsAdmin          bool   `json:"isAdmin"`
	Limits           struct {
		Collection limitDetail `json:"collection"`
		Wishlist   limitDetail `json:"wishlist"`
		Shares     limitDetail `json:"shares"`
	} `json:"limits"`
}

func (h *Handler) status(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserID(r.Context())

	us, err := FetchStatus(r.Context(), h.pool, userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch status"})
		return
	}

	resp := statusResponse{
		UserID:           us.UserID,
		Tier:             us.Tier,
		Status:           us.Status,
		CurrentPeriodEnd: us.CurrentPeriodEnd.Format("2006-01-02T15:04:05Z07:00"),
		IsVIP:            us.IsVIP,
		IsAdmin:          us.IsAdmin,
	}

	// If premium but period end is zero, backfill from Paddle API.
	// Webhook delivery ordering can leave current_period_end null;
	// this self-heals on the next status poll.
	if us.IsPremium() && us.CurrentPeriodEnd.IsZero() && h.paddle != nil {
		var subID string
		if err := h.pool.QueryRow(r.Context(),
			"SELECT id FROM public.subscriptions WHERE user_id = $1", userID).Scan(&subID); err == nil && subID != "" {
			if endsAt, err := h.paddle.GetSubscriptionPeriodEnd(r.Context(), subID); err == nil && endsAt != "" {
				if t, err := time.Parse(time.RFC3339, endsAt); err == nil {
					resp.CurrentPeriodEnd = t.Format("2006-01-02T15:04:05Z07:00")
					// Persist so future calls don't need to hit Paddle
					h.pool.Exec(r.Context(),
						"UPDATE public.subscriptions SET current_period_end = $1, updated_at = now() WHERE user_id = $2",
						t, userID)
				}
			}
		}
	}

	// Fetch usage counts
	var collectionCount, wishlistCount, shareCount int

	h.pool.QueryRow(r.Context(), "SELECT COUNT(*) FROM public.collection_items WHERE user_id = $1", userID).Scan(&collectionCount)
	h.pool.QueryRow(r.Context(), "SELECT COUNT(*) FROM public.wishlist_items WHERE user_id = $1", userID).Scan(&wishlistCount)
	h.pool.QueryRow(r.Context(), "SELECT COUNT(*) FROM public.wishlist_shares WHERE owner_id = $1", userID).Scan(&shareCount)

	collLimit := FreeCollectionLimit
	wishLimit := FreeWishlistLimit
	shareLimit := FreeShareLimit
	if us.IsPremium() {
		collLimit = -1 // unlimited
		wishLimit = -1
		shareLimit = -1
	}

	resp.Limits.Collection = limitDetail{Used: collectionCount, Limit: collLimit, IsExceeded: collLimit > 0 && collectionCount >= collLimit}
	resp.Limits.Wishlist = limitDetail{Used: wishlistCount, Limit: wishLimit, IsExceeded: wishLimit > 0 && wishlistCount >= wishLimit}
	resp.Limits.Shares = limitDetail{Used: shareCount, Limit: shareLimit, IsExceeded: shareLimit > 0 && shareCount >= shareLimit}

	writeJSON(w, http.StatusOK, resp)
}

type vipRequest struct {
	UserID string `json:"userId"`
	IsVIP  bool   `json:"isVip"`
}

func (h *Handler) setVIP(w http.ResponseWriter, r *http.Request) {
	// Verify caller is admin
	callerID := auth.UserID(r.Context())
	var isAdmin bool
	err := h.pool.QueryRow(r.Context(), "SELECT is_admin FROM public.profiles WHERE id = $1", callerID).Scan(&isAdmin)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to verify admin status"})
		return
	}
	if !isAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
		return
	}

	var req vipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if strings.TrimSpace(req.UserID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "userId is required"})
		return
	}

	_, err = h.pool.Exec(r.Context(), "UPDATE public.profiles SET is_vip = $1 WHERE id = $2", req.IsVIP, req.UserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update VIP status"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "success",
		"userId": req.UserID,
		"isVip":  req.IsVIP,
	})
}

// paddleWebhookEvent represents the structure of a Paddle webhook notification.
type paddleWebhookEvent struct {
	EventID    string          `json:"event_id"`
	EventType  string          `json:"event_type"`
	OccurredAt string          `json:"occurred_at"`
	Data       json.RawMessage `json:"data"`
}

// paddleSubscriptionData represents subscription/transaction data from Paddle webhooks.
type paddleSubscriptionData struct {
	ID               string `json:"id"`
	SubscriptionID   string `json:"subscription_id"`
	CustomerID       string `json:"customer_id"`
	Status           string `json:"status"`
	CurrentBillingPeriod struct {
		EndsAt string `json:"ends_at"`
	} `json:"current_billing_period"`
	CustomData map[string]string `json:"custom_data"`
	Items      []struct {
		Price struct {
			ID string `json:"id"`
		} `json:"price"`
	} `json:"items"`
}

func (h *Handler) webhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}

	sigHeader := r.Header.Get("paddle-signature")
	if err := h.validateSig(body, sigHeader); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid signature"})
		return
	}

	var event paddleWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid event payload"})
		return
	}

	var subData paddleSubscriptionData
	if err := json.Unmarshal(event.Data, &subData); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid event data"})
		return
	}

	// Extract user_id from custom_data
	userID := subData.CustomData["user_id"]
	if userID == "" {
		// Cannot process without user_id
		writeJSON(w, http.StatusOK, map[string]bool{"received": true})
		return
	}

	// Map Paddle event types to internal status
	var tier, status string
	var currentPeriodEnd *time.Time

	switch event.EventType {
	case "transaction.completed":
		tier = "premium"
		status = "active"
	case "subscription.created":
		tier = "premium"
		status = mapPaddleStatus(subData.Status)
	case "subscription.updated":
		tier = "premium"
		status = mapPaddleStatus(subData.Status)
	case "subscription.canceled":
		tier = "premium"
		status = "canceled"
	case "subscription.paused":
		tier = "premium"
		status = "paused"
	default:
		// Acknowledge unknown events
		writeJSON(w, http.StatusOK, map[string]bool{"received": true})
		return
	}

	if subData.CurrentBillingPeriod.EndsAt != "" {
		if t, err := time.Parse(time.RFC3339, subData.CurrentBillingPeriod.EndsAt); err == nil {
			currentPeriodEnd = &t
		}
	}

	priceID := ""
	if len(subData.Items) > 0 {
		priceID = subData.Items[0].Price.ID
	}

	subID := subData.ID
	if event.EventType == "transaction.completed" && subData.SubscriptionID != "" {
		subID = subData.SubscriptionID
	}

	ctx := r.Context()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to begin transaction"})
		return
	}
	defer tx.Rollback(ctx)

	insertResult, err := tx.Exec(ctx, `
		INSERT INTO public.paddle_webhook_events (event_id, event_type)
		VALUES ($1, $2)
		ON CONFLICT (event_id) DO NOTHING`,
		event.EventID, event.EventType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to record webhook event"})
		return
	}
	if insertResult.RowsAffected() == 0 {
		writeJSON(w, http.StatusOK, map[string]bool{"received": true})
		return
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO public.subscriptions (id, user_id, paddle_customer_id, price_id, tier, status, current_period_end)
		VALUES ($1, $2::uuid, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id) DO UPDATE SET
			id = EXCLUDED.id,
			paddle_customer_id = COALESCE(EXCLUDED.paddle_customer_id, public.subscriptions.paddle_customer_id),
			price_id = COALESCE(EXCLUDED.price_id, public.subscriptions.price_id),
			tier = EXCLUDED.tier,
			status = EXCLUDED.status,
			current_period_end = COALESCE(EXCLUDED.current_period_end, public.subscriptions.current_period_end),
			updated_at = now()`,
		subID, userID, subData.CustomerID, priceID, tier, status, currentPeriodEnd)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update subscription"})
		return
	}

	if err := tx.Commit(ctx); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to commit transaction"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"received": true})
}

func mapPaddleStatus(paddleStatus string) string {
	switch paddleStatus {
	case "active":
		return "active"
	case "trialing":
		return "trialing"
	case "past_due":
		return "past_due"
	case "canceled":
		return "canceled"
	case "paused":
		return "paused"
	case "unpaid":
		return "unpaid"
	default:
		return "inactive"
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
