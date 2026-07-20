package notifications

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestListReturnsNotifications(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	createdAt := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	readAt := time.Date(2026, 7, 19, 13, 0, 0, 0, time.UTC)
	rows := pgxmock.NewRows([]string{"id", "type", "actor_id", "username", "display_name", "data", "read_at", "created_at"}).
		AddRow("n-1", "wishlist_shared", "actor-1", "alice", "Alice A", []byte(`{"ownerUsername":"alice"}`), &readAt, createdAt).
		AddRow("n-2", "wishlist_claimed", "actor-2", "bob", nil, []byte(`{}`), nil, createdAt)
	mock.ExpectQuery("SELECT n.id::text, n.type, n.actor_id::text").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(rows)

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	h.list(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	body := res.Body.String()
	if !strings.Contains(body, `"id":"n-1"`) || !strings.Contains(body, `"id":"n-2"`) {
		t.Fatalf("expected both notifications, got %q", body)
	}
	if !strings.Contains(body, `"readAt"`) {
		t.Fatalf("expected readAt on read notification, got %q", body)
	}
	if !strings.Contains(body, `"username":"alice"`) {
		t.Fatalf("expected actor username, got %q", body)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListReturnsEmptyArrayWhenNone(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT n.id::text, n.type, n.actor_id::text").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "type", "actor_id", "username", "display_name", "data", "read_at", "created_at"}))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	h.list(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if strings.TrimSpace(res.Body.String()) != "[]" {
		t.Fatalf("expected empty array, got %q", res.Body.String())
	}
}

func TestListReturnsServerErrorOnQueryFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT n.id::text").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(errors.New("db down"))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	h.list(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestUnreadCountReturnsCount(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT COUNT").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(3))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/unread-count", nil)
	res := httptest.NewRecorder()

	h.unreadCount(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if !strings.Contains(res.Body.String(), `"count":3`) {
		t.Fatalf("expected count 3, got %q", res.Body.String())
	}
}

func TestUnreadCountReturnsServerErrorOnFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT COUNT").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(errors.New("db down"))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/unread-count", nil)
	res := httptest.NewRecorder()

	h.unreadCount(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestMarkReadUpdatesNotification(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("UPDATE public.notifications").
		WithArgs("n-1", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/n-1/read", nil)
	res := httptest.NewRecorder()

	serveWithChi(h.Routes(), res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusNoContent, res.Code, res.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMarkReadReturnsNotFoundForMissingNotification(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("UPDATE public.notifications").
		WithArgs("n-missing", pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 0))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/n-missing/read", nil)
	res := httptest.NewRecorder()

	serveWithChi(h.Routes(), res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, res.Code)
	}
}

func TestMarkReadReturnsServerErrorOnFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("UPDATE public.notifications").
		WithArgs("n-1", pgxmock.AnyArg()).
		WillReturnError(errors.New("db down"))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/n-1/read", nil)
	res := httptest.NewRecorder()

	serveWithChi(h.Routes(), res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestMarkAllReadUpdatesNotifications(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("UPDATE public.notifications").
		WithArgs(pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 5))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/read-all", nil)
	res := httptest.NewRecorder()

	serveWithChi(h.Routes(), res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.Code)
	}
}

func TestMarkAllReadReturnsServerErrorOnFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("UPDATE public.notifications").
		WithArgs(pgxmock.AnyArg()).
		WillReturnError(errors.New("db down"))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/read-all", nil)
	res := httptest.NewRecorder()

	serveWithChi(h.Routes(), res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func serveWithChi(router http.Handler, res *httptest.ResponseRecorder, req *http.Request) {
	router.ServeHTTP(res, req)
}
