package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type SupabaseAdminLookup struct {
	baseURL        string
	serviceRoleKey string
	client         *http.Client
}

func NewSupabaseAdminLookup(baseURL, serviceRoleKey string) *SupabaseAdminLookup {
	return &SupabaseAdminLookup{
		baseURL:        baseURL,
		serviceRoleKey: serviceRoleKey,
		client:         &http.Client{Timeout: 5 * time.Second},
	}
}

func (l *SupabaseAdminLookup) EmailForUser(ctx context.Context, userID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.baseURL+"/auth/v1/admin/users/"+userID, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("apikey", l.serviceRoleKey)
	req.Header.Set("Authorization", "Bearer "+l.serviceRoleKey)

	resp, err := l.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("supabase admin: unexpected status %d", resp.StatusCode)
	}

	var user struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", err
	}
	if user.Email == "" {
		return "", fmt.Errorf("supabase admin: no email for user %s", userID)
	}
	return user.Email, nil
}

type NoOpLookup struct{}

func (NoOpLookup) EmailForUser(ctx context.Context, userID string) (string, error) {
	return "", nil
}
