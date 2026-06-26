package billing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewPaddleClient_Production(t *testing.T) {
	client := NewPaddleClient("test-key", "production", "https://audiofile.app")

	if client.apiKey != "test-key" {
		t.Errorf("expected apiKey 'test-key', got %s", client.apiKey)
	}
	if client.baseURL != "https://api.paddle.com" {
		t.Errorf("expected baseURL 'https://api.paddle.com', got %s", client.baseURL)
	}
	if client.portalBase != "https://buy.paddle.com" {
		t.Errorf("expected portalBase 'https://buy.paddle.com', got %s", client.portalBase)
	}
	if client.successURL != "https://audiofile.app/settings/billing?checkout=success" {
		t.Errorf("unexpected successURL: %s", client.successURL)
	}
	if client.cancelURL != "https://audiofile.app/settings/billing?checkout=canceled" {
		t.Errorf("unexpected cancelURL: %s", client.cancelURL)
	}
}

func TestNewPaddleClient_Sandbox(t *testing.T) {
	client := NewPaddleClient("test-key", "sandbox", "https://staging.audiofile.app")

	if client.baseURL != "https://sandbox-api.paddle.com" {
		t.Errorf("expected baseURL 'https://sandbox-api.paddle.com', got %s", client.baseURL)
	}
	if client.portalBase != "https://sandbox-buy.paddle.com" {
		t.Errorf("expected portalBase 'https://sandbox-buy.paddle.com', got %s", client.portalBase)
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
	client := &PaddleHTTPClient{
		portalBase: "https://buy.paddle.com",
	}

	url, err := client.GetCustomerPortalURL(context.Background(), "ctm_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "https://buy.paddle.com/portal/customers/ctm_123"
	if url != expected {
		t.Errorf("expected URL %s, got %s", expected, url)
	}
}

func TestPaddleClient_GetCustomerPortalURL_EmptyID(t *testing.T) {
	client := &PaddleHTTPClient{}

	_, err := client.GetCustomerPortalURL(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty customer ID, got nil")
	}
	if !strings.Contains(err.Error(), "customer ID is required") {
		t.Errorf("expected 'customer ID is required' error, got: %v", err)
	}
}
