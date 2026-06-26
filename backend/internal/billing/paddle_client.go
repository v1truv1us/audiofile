package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// PaddleHTTPClient implements PaddleClient using the Paddle Billing API.
type PaddleHTTPClient struct {
	apiKey      string
	baseURL     string
	httpClient  *http.Client
	successURL  string
	cancelURL   string
	portalBase  string
}

// NewPaddleClient creates a Paddle API client.
// environment should be "production" or "sandbox".
func NewPaddleClient(apiKey, environment, appBaseURL string) *PaddleHTTPClient {
	baseURL := "https://sandbox-api.paddle.com"
	portalBase := "https://sandbox-buy.paddle.com"
	if environment == "production" {
		baseURL = "https://api.paddle.com"
		portalBase = "https://buy.paddle.com"
	}

	return &PaddleHTTPClient{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		successURL: appBaseURL + "/settings/billing?checkout=success",
		cancelURL:  appBaseURL + "/settings/billing?checkout=canceled",
		portalBase: portalBase,
	}
}

// CreateTransaction creates a Paddle transaction and returns the checkout URL.
func (c *PaddleHTTPClient) CreateTransaction(ctx context.Context, priceID string, customData map[string]string) (string, error) {
	payload := map[string]any{
		"items": []map[string]any{
			{
				"price_id": priceID,
				"quantity": 1,
			},
		},
		"custom_data": customData,
		"checkout": map[string]string{
			"url": c.successURL,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/transactions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call Paddle API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Paddle API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data struct {
			Checkout struct {
				URL string `json:"url"`
			} `json:"checkout"`
		} `json:"data"`
		Error *struct {
			Type    string `json:"type"`
			Message string `json:"detail"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("Paddle API error: %s - %s", result.Error.Type, result.Error.Message)
	}

	if result.Data.Checkout.URL == "" {
		return "", fmt.Errorf("Paddle API returned empty checkout URL")
	}

	return result.Data.Checkout.URL, nil
}

// GetCustomerPortalURL returns the Paddle customer portal URL.
// Paddle's customer portal is a hosted page where customers can manage subscriptions.
func (c *PaddleHTTPClient) GetCustomerPortalURL(ctx context.Context, customerID string) (string, error) {
	if customerID == "" {
		return "", fmt.Errorf("customer ID is required")
	}

	// Paddle's customer portal URL pattern
	// Customers access their portal at: https://buy.paddle.com/portal/customers/{customer_id}
	// We can optionally append a return_url query parameter
	portalURL := fmt.Sprintf("%s/portal/customers/%s", c.portalBase, customerID)

	return portalURL, nil
}

// GetSubscriptionPeriodEnd fetches the current billing period end for a subscription.
func (c *PaddleHTTPClient) GetSubscriptionPeriodEnd(ctx context.Context, subscriptionID string) (string, error) {
	if subscriptionID == "" {
		return "", fmt.Errorf("subscription ID is required")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/subscriptions/"+subscriptionID, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call Paddle API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Paddle API error (status %d)", resp.StatusCode)
	}

	var result struct {
		Data struct {
			CurrentBillingPeriod struct {
				EndsAt string `json:"ends_at"`
			} `json:"current_billing_period"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Data.CurrentBillingPeriod.EndsAt, nil
}
