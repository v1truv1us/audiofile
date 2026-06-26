package billing

import "net/http"

// Webhook is the exported handler for the Paddle webhook endpoint.
// It delegates to the internal webhook method so it can be mounted
// outside the authenticated route group in main.go.
func (h *Handler) Webhook(w http.ResponseWriter, r *http.Request) {
	h.webhook(w, r)
}
