package billing

import "context"

// PaddleClient abstracts Paddle API operations for testability.
// The real implementation uses HTTP calls to the Paddle API; this interface
// allows mocking without adding the dependency yet.
type PaddleClient interface {
	CreateTransaction(ctx context.Context, priceID string, customData map[string]string) (string, error)
	GetCustomerPortalURL(ctx context.Context, customerID string) (string, error)
	// GetSubscriptionPeriodEnd fetches the current billing period end time
	// (RFC3339) for a subscription by its ID. Returns empty string if unknown.
	GetSubscriptionPeriodEnd(ctx context.Context, subscriptionID string) (string, error)
}
