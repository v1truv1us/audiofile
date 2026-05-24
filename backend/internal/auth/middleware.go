package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
)

type contextKey string

const UserIDKey contextKey = "user_id"

// Middleware validates the Supabase JWT by calling the Supabase Auth API.
// This avoids needing the raw JWT secret — just requires SUPABASE_URL.
func Middleware(supabaseURL string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				writeUnauthorized(w, "missing authorization header")
				return
			}

			tokenStr := strings.TrimPrefix(header, "Bearer ")
			if tokenStr == header {
				writeUnauthorized(w, "invalid authorization format")
				return
			}

			// Verify token with Supabase Auth API
			userID, err := verifyToken(r.Context(), supabaseURL, tokenStr)
			if err != nil {
				writeUnauthorized(w, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type supabaseUser struct {
	ID string `json:"id"`
}

func verifyToken(ctx context.Context, supabaseURL, token string) (string, error) {
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, supabaseURL+"/auth/v1/user", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("apikey", os.Getenv("PUBLIC_SUPABASE_ANON_KEY"))

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", ErrInvalidToken
	}

	var user supabaseUser
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return "", err
	}

	if user.ID == "" {
		return "", ErrInvalidToken
	}

	return user.ID, nil
}

var ErrInvalidToken = &authError{"invalid token"}

type authError struct {
	msg string
}

func (e *authError) Error() string { return e.msg }

// UserID extracts the authenticated user_id from request context.
func UserID(ctx context.Context) string {
	if v, ok := ctx.Value(UserIDKey).(string); ok {
		return v
	}
	return ""
}

func writeUnauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
