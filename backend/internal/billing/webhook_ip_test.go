package billing

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPaddleSourceIP_PrefersCloudflare(t *testing.T) {
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("CF-Connecting-IP", "203.0.113.9")
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	if got := paddleSourceIP(req); got != "203.0.113.9" {
		t.Fatalf("expected CF-Connecting-IP 203.0.113.9, got %q", got)
	}
}

func TestPaddleSourceIP_XFFFirstHop(t *testing.T) {
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.1")
	if got := paddleSourceIP(req); got != "203.0.113.9" {
		t.Fatalf("expected first XFF hop 203.0.113.9, got %q", got)
	}
}

func TestPaddleSourceIP_RemoteAddrFallback(t *testing.T) {
	req := httptest.NewRequest("POST", "/", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	if got := paddleSourceIP(req); got != "127.0.0.1" {
		t.Fatalf("expected 127.0.0.1, got %q", got)
	}
}

func TestIPInCIDRList(t *testing.T) {
	cidrs := []string{"34.237.3.244/32", "52.11.166.252/32"}
	cases := []struct {
		ip   string
		want bool
	}{
		{"34.237.3.244", true},
		{"52.11.166.252", true},
		{"8.8.8.8", false},
		{"34.237.3.245", false},
		{"not-an-ip", false},
	}
	for _, c := range cases {
		if got := ipInCIDRList(c.ip, cidrs); got != c.want {
			t.Errorf("ipInCIDRList(%q) = %v, want %v", c.ip, got, c.want)
		}
	}
}

func TestDefaultPaddleIPAllowLister_SkipsSandbox(t *testing.T) {
	t.Setenv("PADDLE_ENVIRONMENT", "sandbox")
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("CF-Connecting-IP", "8.8.8.8")
	if err := defaultPaddleIPAllowLister(req); err != nil {
		t.Fatalf("expected skip in sandbox, got %v", err)
	}
}

func TestDefaultPaddleIPAllowLister_SkipsLoopbackInProduction(t *testing.T) {
	t.Setenv("PADDLE_ENVIRONMENT", "production")
	req := httptest.NewRequest("POST", "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	if err := defaultPaddleIPAllowLister(req); err != nil {
		t.Fatalf("expected skip for loopback in production, got %v", err)
	}
}

func TestWebhookRejectsDisallowedIP(t *testing.T) {
	h := NewHandler(nil, nil)
	h.SetIPAllowLister(func(r *http.Request) error { return errors.New("blocked by allowlist") })
	h.SetSignatureValidator(func([]byte, string) error { return nil })

	req := httptest.NewRequest("POST", "/webhook", strings.NewReader("{}"))
	req.Header.Set("paddle-signature", "ts=1;h1=x")
	w := httptest.NewRecorder()
	h.webhook(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disallowed IP, got %d (body=%s)", w.Code, w.Body.String())
	}
}
