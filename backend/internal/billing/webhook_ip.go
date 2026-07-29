package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// IPAllowLister decides whether a webhook request originates from an allowed Paddle source IP.
type IPAllowLister func(r *http.Request) error

const paddleIPsURL = "https://api.paddle.com/ips"

var (
	paddleIPsMu       sync.Mutex
	paddleIPsCache    []string
	paddleIPsCachedAt time.Time
	paddleIPsClient   = &http.Client{Timeout: 10 * time.Second}
)

const paddleIPsTTL = 1 * time.Hour

// paddleSourceIP extracts the originating client IP, preferring Cloudflare's
// header (the app runs behind Cloudflare) and then X-Forwarded-For.
func paddleSourceIP(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); v != "" {
		return v
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i]
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// ipInCIDRList reports whether ip is contained in any of the supplied CIDR ranges.
func ipInCIDRList(ip string, cidrs []string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, cidr := range cidrs {
		if _, network, err := net.ParseCIDR(cidr); err == nil && network.Contains(parsed) {
			return true
		}
	}
	return false
}

// fetchPaddleIPs returns Paddle's current webhook source IPv4 CIDRs, cached for paddleIPsTTL.
// The endpoint is the source of truth — the list can change, so it is never hard-coded.
func fetchPaddleIPs(ctx context.Context) ([]string, error) {
	paddleIPsMu.Lock()
	defer paddleIPsMu.Unlock()
	if time.Since(paddleIPsCachedAt) < paddleIPsTTL && len(paddleIPsCache) > 0 {
		return paddleIPsCache, nil
	}
	req, err := http.NewRequestWithContext(ctx, "GET", paddleIPsURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := paddleIPsClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("paddle ips endpoint returned status %d", resp.StatusCode)
	}
	var out struct {
		Data struct {
			IPv4CIDRs []string `json:"ipv4_cidrs"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	paddleIPsCache = out.Data.IPv4CIDRs
	paddleIPsCachedAt = time.Now()
	return paddleIPsCache, nil
}

// defaultPaddleIPAllowLister enforces Paddle's published webhook source IPs in production.
//   - Skipped when PADDLE_ENVIRONMENT != "production" (sandbox/dev).
//   - Skipped for loopback connections (local development and tests).
//   - If the IP list cannot be fetched, it fails OPEN (logs a warning) and defers to
//     HMAC signature verification, which remains the primary auth check. This avoids
//     blocking all webhooks during a transient outage of the Paddle IPs endpoint.
//   - Otherwise it rejects any request whose source IP is not in Paddle's list.
func defaultPaddleIPAllowLister(r *http.Request) error {
	if os.Getenv("PADDLE_ENVIRONMENT") != "production" {
		return nil
	}
	ip := paddleSourceIP(r)
	if parsed := net.ParseIP(ip); parsed != nil && parsed.IsLoopback() {
		return nil
	}
	cidrs, err := fetchPaddleIPs(r.Context())
	if err != nil {
		log.Printf("paddle webhook: IP allowlist fetch failed (%v); falling back to signature verification only", err)
		return nil
	}
	if !ipInCIDRList(ip, cidrs) {
		return fmt.Errorf("webhook source IP %s not in Paddle allowlist", ip)
	}
	return nil
}
