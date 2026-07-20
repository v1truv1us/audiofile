package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func expectActorProfile(mock pgxmock.PgxPoolIface, actorID string) {
	mock.ExpectQuery("SELECT username, display_name").
		WithArgs(actorID).
		WillReturnRows(pgxmock.NewRows([]string{"username", "display_name"}).AddRow("alice", "Alice A"))
}

func expectNotificationInsert(mock pgxmock.PgxPoolIface, userID, notifType, actorID string) {
	mock.ExpectExec("INSERT INTO public.notifications").
		WithArgs(userID, notifType, actorID, pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
}

func TestNotifyWishlistSharedInsertsNotification(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	expectActorProfile(mock, "owner-1")
	expectNotificationInsert(mock, "viewer-1", TypeWishlistShared, "owner-1")

	n := NewNotifier(mock, nil, nil)
	if err := n.NotifyWishlistShared(context.Background(), "viewer-1", "owner-1", "check this out"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNotifyWishlistSharedPropagatesInsertError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	expectActorProfile(mock, "owner-1")
	mock.ExpectExec("INSERT INTO public.notifications").
		WithArgs("viewer-1", TypeWishlistShared, "owner-1", pgxmock.AnyArg()).
		WillReturnError(errors.New("insert failed"))

	n := NewNotifier(mock, nil, nil)
	if err := n.NotifyWishlistShared(context.Background(), "viewer-1", "owner-1", ""); err == nil {
		t.Fatal("expected insert error to propagate")
	}
}

func TestNotifyWishlistSharedPropagatesProfileLookupError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT username, display_name").
		WithArgs("owner-1").
		WillReturnError(errors.New("db down"))

	n := NewNotifier(mock, nil, nil)
	if err := n.NotifyWishlistShared(context.Background(), "viewer-1", "owner-1", ""); err == nil {
		t.Fatal("expected profile lookup error to propagate")
	}
}

func TestNotifyWishlistSharedSendsEmail(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	expectActorProfile(mock, "owner-1")
	expectNotificationInsert(mock, "viewer-1", TypeWishlistShared, "owner-1")

	adminServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("apikey") != "service-key" || r.Header.Get("Authorization") != "Bearer service-key" {
			t.Errorf("missing service role headers")
		}
		if !strings.Contains(r.URL.Path, "/auth/v1/admin/users/viewer-1") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Write([]byte(`{"email":"viewer@example.com"}`))
	}))
	defer adminServer.Close()

	var resendBody []byte
	resendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer resend-key" {
			t.Errorf("missing resend auth header")
		}
		resendBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"email-1"}`))
	}))
	defer resendServer.Close()

	sender := NewResendSender("resend-key", "AudioFile <notifications@audiofile.app>")
	sender.baseURL = resendServer.URL
	lookup := NewSupabaseAdminLookup(adminServer.URL, "service-key")

	n := NewNotifier(mock, sender, lookup)
	if err := n.NotifyWishlistShared(context.Background(), "viewer-1", "owner-1", "hi"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	var payload resendEmailRequest
	if err := json.Unmarshal(resendBody, &payload); err != nil {
		t.Fatalf("failed to parse resend payload: %v", err)
	}
	if payload.To != "viewer@example.com" {
		t.Fatalf("expected recipient email, got %q", payload.To)
	}
	if payload.Subject == "" || payload.HTML == "" {
		t.Fatalf("expected subject and html, got %+v", payload)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNotifyWishlistSharedSwallowsEmailFailures(t *testing.T) {
	cases := map[string]struct {
		adminStatus  int
		resendStatus int
	}{
		"email lookup fails":  {adminStatus: 404, resendStatus: 200},
		"email send fails":    {adminStatus: 200, resendStatus: 500},
		"both servers broken": {adminStatus: 500, resendStatus: 500},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			if err != nil {
				t.Fatal(err)
			}
			defer mock.Close()

			expectActorProfile(mock, "owner-1")
			expectNotificationInsert(mock, "viewer-1", TypeWishlistShared, "owner-1")

			adminServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.adminStatus)
				w.Write([]byte(`{"email":"viewer@example.com"}`))
			}))
			defer adminServer.Close()

			resendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.resendStatus)
			}))
			defer resendServer.Close()

			sender := NewResendSender("resend-key", "from@example.com")
			sender.baseURL = resendServer.URL
			lookup := NewSupabaseAdminLookup(adminServer.URL, "service-key")

			n := NewNotifier(mock, sender, lookup)
			if err := n.NotifyWishlistShared(context.Background(), "viewer-1", "owner-1", ""); err != nil {
				t.Fatalf("expected best-effort nil error, got %v", err)
			}
		})
	}
}

func TestNotifyWishlistSharedSkipsEmailWhenNotConfigured(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	expectActorProfile(mock, "owner-1")
	expectNotificationInsert(mock, "viewer-1", TypeWishlistShared, "owner-1")

	n := NewNotifier(mock, NoOpSender{}, NoOpLookup{})
	if err := n.NotifyWishlistShared(context.Background(), "viewer-1", "owner-1", ""); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNotifyWishlistClaimedInsertsNotificationWithoutEmail(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	expectActorProfile(mock, "viewer-1")
	expectNotificationInsert(mock, "owner-1", TypeWishlistClaimed, "viewer-1")

	sender := &recordingSender{}
	n := NewNotifier(mock, sender, NoOpLookup{})
	if err := n.NotifyWishlistClaimed(context.Background(), "owner-1", "viewer-1"); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if sender.calls != 0 {
		t.Fatalf("expected no email for claim notifications, got %d calls", sender.calls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNotifyWishlistClaimedPropagatesInsertError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	expectActorProfile(mock, "viewer-1")
	mock.ExpectExec("INSERT INTO public.notifications").
		WithArgs("owner-1", TypeWishlistClaimed, "viewer-1", pgxmock.AnyArg()).
		WillReturnError(errors.New("insert failed"))

	n := NewNotifier(mock, nil, nil)
	if err := n.NotifyWishlistClaimed(context.Background(), "owner-1", "viewer-1"); err == nil {
		t.Fatal("expected insert error to propagate")
	}
}

type recordingSender struct {
	calls int
}

func (s *recordingSender) Send(ctx context.Context, to, subject, htmlBody string) error {
	s.calls++
	return nil
}
