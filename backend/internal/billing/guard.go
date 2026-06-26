package billing

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var (
	ErrCollectionLimitExceeded = errors.New("free tier collection limit (50 items) reached")
	ErrWishlistLimitExceeded   = errors.New("free tier wishlist limit (25 items) reached")
	ErrShareLimitExceeded      = errors.New("free tier wishlist sharing limit (1 share) reached")
)

type dbPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// UserStatus represents the current subscription and privilege state of a user.
type UserStatus struct {
	UserID           string    `json:"userId"`
	Tier             string    `json:"tier"`
	Status           string    `json:"status"`
	CurrentPeriodEnd time.Time `json:"currentPeriodEnd"`
	IsVIP            bool      `json:"isVip"`
	IsAdmin          bool      `json:"isAdmin"`
}

// IsPremium returns true if the user bypasses limits via VIP override or active subscription.
func (u *UserStatus) IsPremium() bool {
	if u.IsVIP {
		return true
	}
	return u.Tier == "premium" && (u.Status == "active" || u.Status == "trialing")
}

// FetchStatus queries the real-time subscription and profile state from the database.
func FetchStatus(ctx context.Context, pool dbPool, userID string) (*UserStatus, error) {
	var tier, status string
	var currentPeriodEnd *time.Time
	var isVip, isAdmin bool

	err := pool.QueryRow(ctx, `
		SELECT 
			COALESCE(s.tier, 'free'),
			COALESCE(s.status, 'inactive'),
			s.current_period_end,
			p.is_vip,
			p.is_admin
		FROM public.profiles p
		LEFT JOIN public.subscriptions s ON s.user_id = p.id
		WHERE p.id = $1`, userID).Scan(&tier, &status, &currentPeriodEnd, &isVip, &isAdmin)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &UserStatus{UserID: userID, Tier: "free", Status: "inactive"}, nil
		}
		return nil, err
	}

	us := &UserStatus{
		UserID:  userID,
		Tier:    tier,
		Status:  status,
		IsVIP:   isVip,
		IsAdmin: isAdmin,
	}
	if currentPeriodEnd != nil {
		us.CurrentPeriodEnd = *currentPeriodEnd
	}
	return us, nil
}

// GuardLimit checks whether a user has exceeded their free-tier limit for the given action.
// Returns nil if the user is premium or under the limit; otherwise returns the appropriate sentinel error.
func GuardLimit(ctx context.Context, pool dbPool, userID string, action string) error {
	status, err := FetchStatus(ctx, pool, userID)
	if err != nil {
		return err
	}

	if status.IsPremium() {
		return nil
	}

	switch action {
	case "collection":
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM public.collection_items WHERE user_id = $1", userID).Scan(&count)
		if err != nil {
			return err
		}
		if count >= FreeCollectionLimit {
			return ErrCollectionLimitExceeded
		}
	case "wishlist":
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM public.wishlist_items WHERE user_id = $1", userID).Scan(&count)
		if err != nil {
			return err
		}
		if count >= FreeWishlistLimit {
			return ErrWishlistLimitExceeded
		}
	case "share":
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM public.wishlist_shares WHERE owner_id = $1", userID).Scan(&count)
		if err != nil {
			return err
		}
		if count >= FreeShareLimit {
			return ErrShareLimitExceeded
		}
	}

	return nil
}
