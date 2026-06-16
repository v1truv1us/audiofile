package profiles

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	pgxmock "github.com/pashagolub/pgxmock/v4"

	"github.com/v1truv1us/audiofile/backend/internal/auth"
)

func TestSearchRejectsShortQuery(t *testing.T) {
	h := NewHandler(nil)
	req := authedRequest(http.MethodGet, "/search?q=a", nil)
	res := httptest.NewRecorder()

	h.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
	if !strings.Contains(res.Body.String(), "query must be at least 2 characters") {
		t.Fatalf("expected validation message, got %q", res.Body.String())
	}
}

func TestSearchReturnsMatches(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id::text, username, display_name").
		WithArgs("mi", "caller-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "username", "display_name"}).
			AddRow("user-2", "miles", "Miles Davis"))

	h := NewHandler(mock)
	req := authedRequest(http.MethodGet, "/search?q=%20mi%20", nil)
	res := httptest.NewRecorder()

	h.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	for _, want := range []string{"user-2", "miles", "Miles Davis", "displayName"} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("expected body to contain %q, got %q", want, res.Body.String())
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSearchReturnsEmptyArrayForNoMatches(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id::text, username, display_name").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "username", "display_name"}))

	h := NewHandler(mock)
	req := authedRequest(http.MethodGet, "/search?q=zz", nil)
	res := httptest.NewRecorder()

	h.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	if strings.TrimSpace(res.Body.String()) != "[]" {
		t.Fatalf("expected empty array, got %q", res.Body.String())
	}
}

func TestSearchReturnsQueryErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id::text, username, display_name").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(assertErr("query failed"))

	h := NewHandler(mock)
	req := authedRequest(http.MethodGet, "/search?q=mi", nil)
	res := httptest.NewRecorder()

	h.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func authedRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, chi.NewRouteContext())
	ctx = context.WithValue(ctx, auth.UserIDKey, "caller-1")
	return req.WithContext(ctx)
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
