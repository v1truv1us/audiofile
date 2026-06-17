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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

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

func TestGetMeReturnsProfile(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id::text, username, display_name").
		WithArgs("caller-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "username", "display_name"}).
			AddRow("caller-1", "miles", "Miles Davis"))

	h := NewHandler(mock)
	req := authedRequest(http.MethodGet, "/me", nil)
	res := httptest.NewRecorder()

	h.getMe(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	for _, want := range []string{"caller-1", "miles", "Miles Davis"} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("expected body to contain %q, got %q", want, res.Body.String())
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetMeReturnsEmptyForMissingProfile(t *testing.T) {
	// Caller authenticated but has no profile row (pre-trigger account).
	// Handler returns an empty profile (id only) rather than erroring.
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id::text, username, display_name").
		WithArgs("caller-1").
		WillReturnError(pgx.ErrNoRows)

	h := NewHandler(mock)
	req := authedRequest(http.MethodGet, "/me", nil)
	res := httptest.NewRecorder()

	h.getMe(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	if !strings.Contains(res.Body.String(), "caller-1") {
		t.Fatalf("expected body to contain caller id, got %q", res.Body.String())
	}
}

func TestGetMeReturnsServerErrorOnQueryFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id::text, username, display_name").
		WithArgs("caller-1").
		WillReturnError(assertErr("db down"))

	h := NewHandler(mock)
	req := authedRequest(http.MethodGet, "/me", nil)
	res := httptest.NewRecorder()

	h.getMe(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateMeRejectsInvalidJSON(t *testing.T) {
	h := NewHandler(nil)
	req := authedRequest(http.MethodPut, "/me", strings.NewReader("{"))
	res := httptest.NewRecorder()

	h.updateMe(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestValidateUsernameEnforcesRules(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
		want    string
	}{
		{name: "valid", input: "miles", wantErr: false, want: "miles"},
		{name: "valid with underscore", input: "miles_davis", wantErr: false, want: "miles_davis"},
		{name: "valid with digits", input: "miles123", wantErr: false, want: "miles123"},
		{name: "uppercased is normalized", input: "  Miles ", wantErr: false, want: "miles"},
		{name: "too short", input: "ab", wantErr: true},
		{name: "too long", input: strings.Repeat("a", 21), wantErr: true},
		{name: "spaces rejected", input: "miles davis", wantErr: true},
		{name: "dot rejected", input: "miles.davis", wantErr: true},
		{name: "plus rejected", input: "miles+1", wantErr: true},
		{name: "at rejected", input: "miles@", wantErr: true},
		{name: "empty", input: "   ", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateUsername(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %q", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestUpdateMeRejectsBadUsername(t *testing.T) {
	h := NewHandler(nil)
	req := authedRequest(http.MethodPut, "/me", strings.NewReader(`{"username":"ab"}`))
	res := httptest.NewRecorder()

	h.updateMe(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestUpdateMeRejectsLongDisplayName(t *testing.T) {
	h := NewHandler(nil)
	long := strings.Repeat("x", 51)
	req := authedRequest(http.MethodPut, "/me", strings.NewReader(`{"username":"miles","displayName":"`+long+`"}`))
	res := httptest.NewRecorder()

	h.updateMe(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestUpdateMeUpdatesProfile(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("UPDATE public.profiles").
		WithArgs("miles", "Miles Davis", "caller-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "username", "display_name"}).
			AddRow("caller-1", "miles", "Miles Davis"))

	h := NewHandler(mock)
	req := authedRequest(http.MethodPut, "/me", strings.NewReader(`{"username":"miles","displayName":"Miles Davis"}`))
	res := httptest.NewRecorder()

	h.updateMe(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	for _, want := range []string{"caller-1", "miles", "Miles Davis"} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("expected body to contain %q, got %q", want, res.Body.String())
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateMeReturnsConflictOnDuplicateUsername(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("UPDATE public.profiles").
		WithArgs("taken", pgxmock.AnyArg(), "caller-1").
		WillReturnError(&pgconn.PgError{Code: "23505"})

	h := NewHandler(mock)
	req := authedRequest(http.MethodPut, "/me", strings.NewReader(`{"username":"taken","displayName":""}`))
	res := httptest.NewRecorder()

	h.updateMe(res, req)

	if res.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, res.Code)
	}
}

func TestUpdateMeCreatesProfileWhenMissing(t *testing.T) {
	// Pre-trigger account has no profile row; UPDATE affects 0 rows (pgx.ErrNoRows
	// from RETURNING), so the handler falls back to INSERT ... ON CONFLICT.
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("UPDATE public.profiles").
		WithArgs("miles", pgxmock.AnyArg(), "caller-1").
		WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("INSERT INTO public.profiles").
		WithArgs("caller-1", "miles", pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id", "username", "display_name"}).
			AddRow("caller-1", "miles", ""))

	h := NewHandler(mock)
	req := authedRequest(http.MethodPut, "/me", strings.NewReader(`{"username":"miles","displayName":""}`))
	res := httptest.NewRecorder()

	h.updateMe(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateMeReturnsServerErrorOnUpdateFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("UPDATE public.profiles").
		WithArgs("miles", pgxmock.AnyArg(), "caller-1").
		WillReturnError(assertErr("update failed"))

	h := NewHandler(mock)
	req := authedRequest(http.MethodPut, "/me", strings.NewReader(`{"username":"miles","displayName":""}`))
	res := httptest.NewRecorder()

	h.updateMe(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateMeReturnsConflictOnFallbackInsertDuplicate(t *testing.T) {
	// Caller has no profile row; UPDATE returns ErrNoRows, handler falls back
	// to INSERT, which itself hits a 23505 (username taken by someone else).
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("UPDATE public.profiles").
		WithArgs("taken", pgxmock.AnyArg(), "caller-1").
		WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("INSERT INTO public.profiles").
		WithArgs("caller-1", "taken", pgxmock.AnyArg()).
		WillReturnError(&pgconn.PgError{Code: "23505"})

	h := NewHandler(mock)
	req := authedRequest(http.MethodPut, "/me", strings.NewReader(`{"username":"taken","displayName":""}`))
	res := httptest.NewRecorder()

	h.updateMe(res, req)

	if res.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, res.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateMeReturnsServerErrorOnFallbackInsertFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("UPDATE public.profiles").
		WithArgs("miles", pgxmock.AnyArg(), "caller-1").
		WillReturnError(pgx.ErrNoRows)
	mock.ExpectQuery("INSERT INTO public.profiles").
		WithArgs("caller-1", "miles", pgxmock.AnyArg()).
		WillReturnError(assertErr("insert failed"))

	h := NewHandler(mock)
	req := authedRequest(http.MethodPut, "/me", strings.NewReader(`{"username":"miles","displayName":""}`))
	res := httptest.NewRecorder()

	h.updateMe(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
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
