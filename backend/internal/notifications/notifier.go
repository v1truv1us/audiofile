package notifications

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"

	"github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
)

const (
	TypeWishlistShared  = "wishlist_shared"
	TypeWishlistClaimed = "wishlist_claimed"
)

type EmailSender interface {
	Send(ctx context.Context, to, subject, htmlBody string) error
}

type EmailLookup interface {
	EmailForUser(ctx context.Context, userID string) (string, error)
}

type Notifier struct {
	pool   dbPool
	email  EmailSender
	lookup EmailLookup
}

func NewNotifier(pool dbPool, email EmailSender, lookup EmailLookup) *Notifier {
	return &Notifier{pool: pool, email: email, lookup: lookup}
}

func (n *Notifier) NotifyWishlistShared(ctx context.Context, recipientID, actorID, message string) error {
	username, displayName, err := n.actorProfile(ctx, actorID)
	if err != nil {
		return err
	}

	data, err := json.Marshal(map[string]string{
		"ownerUsername":    username,
		"ownerDisplayName": displayName,
		"message":          message,
	})
	if err != nil {
		return err
	}

	if err := n.insert(ctx, recipientID, TypeWishlistShared, actorID, data); err != nil {
		return err
	}

	if n.email == nil || n.lookup == nil {
		return nil
	}
	to, err := n.lookup.EmailForUser(ctx, recipientID)
	if err != nil {
		n.reportBestEffort(err)
		return nil
	}
	name := displayName
	if name == "" {
		name = username
	}
	subject := name + " shared their wishlist with you"
	body := "<p>" + name + " shared their vinyl wishlist with you on AudioFile.</p>"
	if message != "" {
		body += "<p>\"" + message + "\"</p>"
	}
	body += "<p><a href=\"https://audiofile.app/shared\">View shared wishlists</a></p>"
	if err := n.email.Send(ctx, to, subject, body); err != nil {
		n.reportBestEffort(err)
	}
	return nil
}

func (n *Notifier) NotifyWishlistClaimed(ctx context.Context, ownerID, actorID string) error {
	username, displayName, err := n.actorProfile(ctx, actorID)
	if err != nil {
		return err
	}

	data, err := json.Marshal(map[string]string{
		"viewerUsername":    username,
		"viewerDisplayName": displayName,
	})
	if err != nil {
		return err
	}

	return n.insert(ctx, ownerID, TypeWishlistClaimed, actorID, data)
}

func (n *Notifier) actorProfile(ctx context.Context, actorID string) (string, string, error) {
	var username, displayName sql.NullString
	err := n.pool.QueryRow(ctx, `
		SELECT username, display_name
		FROM public.profiles
		WHERE id = $1`, actorID).Scan(&username, &displayName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", "", nil
		}
		return "", "", err
	}
	return username.String, displayName.String, nil
}

func (n *Notifier) insert(ctx context.Context, userID, notifType, actorID string, data json.RawMessage) error {
	_, err := n.pool.Exec(ctx, `
		INSERT INTO public.notifications (user_id, type, actor_id, data)
		VALUES ($1, $2, $3, $4)`, userID, notifType, actorID, data)
	return err
}

func (n *Notifier) reportBestEffort(err error) {
	log.Printf("notifications: best-effort email failed: %v", err)
	sentry.CaptureException(err)
}
