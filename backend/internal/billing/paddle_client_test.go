package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewPaddleClient_Production(t *testing.T) {
	client := NewPaddleClient("test-key", "production", "https://audiofile.app")

	if client.apiKey != "test-key" {
		t.Errorf("expected apiKey 'test-key', got %s", client.apiKey)
	}
	if client.baseURL != "https://api.paddle.com" {
		t.Errorf("expected baseURL 'https://api.paddle.com', got %s", client.baseURL)
	}
	if client.successURL != "https://audiofile.app/account?checkout=success#billing" {
		t.Errorf("unexpected successURL: %s", client.successURL)
	}
	if client.cancelURL != "https://audiofile.app/account?checkout=canceled#billing" {
		t.Errorf("unexpected cancelURL: %s", client.cancelURL)
	}
}

func TestNewPaddleClient_Sandbox(t *testing.T) {
	client := NewPaddleClient("test-key", "sandbox", "https://staging.audiofile.app")

	if client.baseURL != "https://sandbox-api.paddle.com" {
		t.Errorf("expected baseURL 'https://sandbox-api.paddle.com', got %s", client.baseURL)
	}
}

func TestPaddleClient_CreateTransaction_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/transactions" {
			t.Errorf("expected path /transactions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization header 'Bearer test-key', got %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got %s", r.Header.Get("Content-Type"))
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		items := payload["items"].([]any)
		if len(items) != 1 {
			t.Errorf("expected 1 item, got %d", len(items))
		}
		item := items[0].(map[string]any)
		if item["price_id"] != "pri_test" {
			t.Errorf("expected price_id 'pri_test', got %v", item["price_id"])
		}

		customData := payload["custom_data"].(map[string]any)
		if customData["user_id"] != "user-123" {
			t.Errorf("expected user_id 'user-123', got %v", customData["user_id"])
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"checkout": map[string]any{
					"url": "https://checkout.paddle.com/test",
				},
			},
		})
	}))
	defer server.Close()

	client := &PaddleHTTPClient{
		apiKey:     "test-key",
		baseURL:    server.URL,
		httpClient: server.Client(),
		successURL: "https://audiofile.app/settings/billing?checkout=success",
	}

	url, err := client.CreateTransaction(context.Background(), "pri_test", map[string]string{"user_id": "user-123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://checkout.paddle.com/test" {
		t.Errorf("expected URL 'https://checkout.paddle.com/test', got %s", url)
	}
}

func TestPaddleClient_CreateTransaction_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"type":   "invalid_request",
				"detail": "Price ID not found",
			},
		})
	}))
	defer server.Close()

	client := &PaddleHTTPClient{
		apiKey:     "test-key",
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	_, err := client.CreateTransaction(context.Background(), "pri_invalid", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Paddle API error") {
		t.Errorf("expected Paddle API error, got: %v", err)
	}
}

func TestPaddleClient_CreateTransaction_EmptyURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"checkout": map[string]any{
					"url": "",
				},
			},
		})
	}))
	defer server.Close()

	client := &PaddleHTTPClient{
		apiKey:     "test-key",
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	_, err := client.CreateTransaction(context.Background(), "pri_test", nil)
	if err == nil {
		t.Fatal("expected error for empty URL, got nil")
	}
	if !strings.Contains(err.Error(), "empty checkout URL") {
		t.Errorf("expected 'empty checkout URL' error, got: %v", err)
	}
}

func TestPaddleClient_GetCustomerPortalURL_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/customers/ctm_123/portal-sessions" {
			t.Errorf("expected path /customers/ctm_123/portal-sessions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization header 'Bearer test-key', got %s", r.Header.Get("Authorization"))
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"urls": map[string]any{
					"general": map[string]any{
						"overview": "https://customer.paddle.com/overview",
					},
				},
			},
		})
	}))
	defer server.Close()

	client := &PaddleHTTPClient{
		apiKey:     "test-key",
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	url, err := client.GetCustomerPortalURL(context.Background(), "ctm_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "https://customer.paddle.com/overview" {
		t.Errorf("expected URL 'https://customer.paddle.com/overview', got %s", url)
	}
}

func TestPaddleClient_GetCustomerPortalURL_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"type":   "invalid_request",
				"detail": "Customer not found",
			},
		})
	}))
	defer server.Close()

	client := &PaddleHTTPClient{
		apiKey:     "test-key",
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	_, err := client.GetCustomerPortalURL(context.Background(), "ctm_invalid")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Paddle API error") {
		t.Errorf("expected Paddle API error, got: %v", err)
	}
}

func TestPaddleClient_GetCustomerPortalURL_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not-json"))
	}))
	defer server.Close()

	client := &PaddleHTTPClient{
		apiKey:     "test-key",
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	_, err := client.GetCustomerPortalURL(context.Background(), "ctm_123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to decode response") {
		t.Errorf("expected decode error, got: %v", err)
	}
}

func TestPaddleClient_GetCustomerPortalURL_EmptyOverview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"urls": map[string]any{
					"general": map[string]any{
						"overview": "",
					},
				},
			},
		})
	}))
	defer server.Close()

	client := &PaddleHTTPClient{
		apiKey:     "test-key",
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	_, err := client.GetCustomerPortalURL(context.Background(), "ctm_123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "empty portal overview URL") {
		t.Errorf("expected empty portal overview URL error, got: %v", err)
	}
}

func TestPaddleClient_GetSubscriptionPeriodEnd_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/subscriptions/sub_123" {
			t.Errorf("expected path /subscriptions/sub_123, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization header 'Bearer test-key', got %s", r.Header.Get("Authorization"))
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{
				"current_billing_period": map[string]any{
					"ends_at": "2026-07-01T00:00:00Z",
				},
			},
		})
	}))
	defer server.Close()

	client := &PaddleHTTPClient{
		apiKey:     "test-key",
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	endsAt, err := client.GetSubscriptionPeriodEnd(context.Background(), "sub_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if endsAt != "2026-07-01T00:00:00Z" {
		t.Errorf("expected ends_at '2026-07-01T00:00:00Z', got %s", endsAt)
	}
}

func TestPaddleClient_GetSubscriptionPeriodEnd_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("subscription not found"))
	}))
	defer server.Close()

	client := &PaddleHTTPClient{
		apiKey:     "test-key",
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	_, err := client.GetSubscriptionPeriodEnd(context.Background(), "sub_missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Paddle API error") {
		t.Errorf("expected Paddle API error, got: %v", err)
	}
}

func TestPaddleClient_GetSubscriptionPeriodEnd_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not-json"))
	}))
	defer server.Close()

	client := &PaddleHTTPClient{
		apiKey:     "test-key",
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	_, err := client.GetSubscriptionPeriodEnd(context.Background(), "sub_123")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to decode response") {
		t.Errorf("expected decode error, got: %v", err)
	}
}

func TestPaddleClient_GetSubscriptionPeriodEnd_EmptySubscriptionID(t *testing.T) {
	client := &PaddleHTTPClient{}

	_, err := client.GetSubscriptionPeriodEnd(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty subscription ID, got nil")
	}
	if !strings.Contains(err.Error(), "subscription ID is required") {
		t.Errorf("expected 'subscription ID is required' error, got: %v", err)
	}
}

func TestPaddleClient_CreateTransaction_NetworkError(t *testing.T) {
	client := &PaddleHTTPClient{
		apiKey:     "test-key",
		baseURL:    "http://localhost:1",
		httpClient: &http.Client{Timeout: 100 * time.Millisecond},
	}

	_, err := client.CreateTransaction(context.Background(), "pri_test", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to call Paddle API") {
		t.Errorf("expected network error, got: %v", err)
	}
}

func TestPaddleClient_CreateTransaction_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not-json"))
	}))
	defer server.Close()

	client := &PaddleHTTPClient{
		apiKey:     "test-key",
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	_, err := client.CreateTransaction(context.Background(), "pri_test", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to decode response") {
		t.Errorf("expected decode error, got: %v", err)
	}
}

func TestPaddleClient_CreateTransaction_Non2xxPlainText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	client := &PaddleHTTPClient{
		apiKey:     "test-key",
		baseURL:    server.URL,
		httpClient: server.Client(),
	}

	_, err := client.CreateTransaction(context.Background(), "pri_test", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Paddle API error") {
		t.Errorf("expected Paddle API error, got: %v", err)
	}
}
