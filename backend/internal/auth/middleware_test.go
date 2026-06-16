package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUserIDReturnsContextValue(t *testing.T) {
	if got := UserID(context.Background()); got != "" {
		t.Fatalf("expected missing user id to be empty, got %q", got)
	}

	ctx := context.WithValue(context.Background(), UserIDKey, "user-123")
	if got := UserID(ctx); got != "user-123" {
		t.Fatalf("expected user id from context, got %q", got)
	}
}

func TestMiddlewareRejectsMissingAuthorization(t *testing.T) {
	h := Middleware("http://unused.invalid")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, res.Code)
	}
	if !strings.Contains(res.Body.String(), "missing authorization header") {
		t.Fatalf("expected missing header message, got %q", res.Body.String())
	}
}

func TestMiddlewareRejectsInvalidAuthorizationFormat(t *testing.T) {
	h := Middleware("http://unused.invalid")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Token abc")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, res.Code)
	}
	if !strings.Contains(res.Body.String(), "invalid authorization format") {
		t.Fatalf("expected invalid format message, got %q", res.Body.String())
	}
}

func TestMiddlewareInjectsVerifiedUserID(t *testing.T) {
	t.Setenv("PUBLIC_SUPABASE_ANON_KEY", "anon-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/v1/user" {
			t.Fatalf("expected auth user path, got %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer valid-token" {
			t.Fatalf("expected bearer token, got %q", got)
		}
		if got := r.Header.Get("apikey"); got != "anon-key" {
			t.Fatalf("expected anon key header, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"user-123"}`))
	}))
	defer server.Close()

	h := Middleware(server.URL)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := UserID(r.Context()); got != "user-123" {
			t.Fatalf("expected injected user id, got %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.Code)
	}
}

func TestMiddlewareRejectsSupabaseNonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	h := Middleware(server.URL)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer expired-token")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, res.Code)
	}
	if !strings.Contains(res.Body.String(), "invalid or expired token") {
		t.Fatalf("expected invalid token message, got %q", res.Body.String())
	}
}

func TestMiddlewareRejectsSupabaseMissingID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"email":"user@example.com"}`))
	}))
	defer server.Close()

	h := Middleware(server.URL)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer no-id-token")
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, res.Code)
	}
}

func TestVerifyTokenReturnsDecodeErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{`))
	}))
	defer server.Close()

	if _, err := verifyToken(context.Background(), server.URL, "bad-json-token"); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestVerifyTokenReturnsRequestErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close()

	if _, err := verifyToken(context.Background(), url, "token"); err == nil {
		t.Fatal("expected request error")
	}
}
