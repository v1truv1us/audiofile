package billing

import (
	"context"
	"errors"
)

// NoOpPaddleClient is a placeholder PaddleClient that returns errors for all operations.
// Use this until the real Paddle API client is integrated.
type NoOpPaddleClient struct{}

var errPaddleNotConfigured = errors.New("paddle client not configured (no-op)")

func (n *NoOpPaddleClient) CreateTransaction(_ context.Context, _ string, _ map[string]string) (string, error) {
	return "", errPaddleNotConfigured
}

func (n *NoOpPaddleClient) GetCustomerPortalURL(_ context.Context, _ string) (string, error) {
	return "", errPaddleNotConfigured
}

func (n *NoOpPaddleClient) GetSubscriptionPeriodEnd(_ context.Context, _ string) (string, error) {
	return "", errPaddleNotConfigured
}
