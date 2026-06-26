package wishlist

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	pgxmock "github.com/pashagolub/pgxmock/v4"

	"github.com/v1truv1us/audiofile/backend/internal/auth"
)

func TestCreateRejectsInvalidJSON(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{"))
	res := httptest.NewRecorder()

	h.create(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestCreateRequiresUserTitleAndArtist(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"userId":"u1","title":"A"}`))
	res := httptest.NewRecorder()

	h.create(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
	if !strings.Contains(res.Body.String(), "title and artist are required") {
		t.Fatalf("expected validation message, got %q", res.Body.String())
	}
}

func TestCreateInsertsWishlistItem(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	expectGuardPass(mock, "wishlist")
	mock.ExpectQuery("INSERT INTO public.releases").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("release-1"))
	mock.ExpectQuery("INSERT INTO public.wishlist_items").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("wish-1"))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"userId":"user-1","title":"Kind of Blue","artist":"Miles Davis","priority":2,"label":"Columbia"}`))
	res := httptest.NewRecorder()

	h.create(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusCreated, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "wish-1") {
		t.Fatalf("expected created id, got %q", res.Body.String())
	}
}

func TestCreateManualWishlistItemWithoutRelease(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	expectGuardPass(mock, "wishlist")
	mock.ExpectQuery("INSERT INTO public.wishlist_items").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("wish-1"))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"userId":"user-1","title":"Kind of Blue","artist":"Miles Davis","priority":99}`))
	res := httptest.NewRecorder()

	h.create(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusCreated, res.Code, res.Body.String())
	}
}

func TestUpdateRequiresIDUserTitleAndArtist(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(http.MethodPut, "/missing", strings.NewReader(`{"userId":"u1","title":"A"}`))
	res := httptest.NewRecorder()

	h.update(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
	if !strings.Contains(res.Body.String(), "id, title, and artist are required") {
		t.Fatalf("expected validation message, got %q", res.Body.String())
	}
}

func TestUpdateChangesWishlistItem(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT release_id").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"release_id"}).AddRow("release-1"))
	mock.ExpectExec("UPDATE public.releases").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))
	mock.ExpectExec("UPDATE public.wishlist_items").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPut, "/wish-1", strings.NewReader(`{"userId":"user-1","title":"Kind of Blue","artist":"Miles Davis","priority":2,"label":"Columbia"}`))
	res := httptest.NewRecorder()

	h.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
}

func TestUpdateCreatesReleaseWhenAddingMetadata(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT release_id").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"release_id"}).AddRow(sql.NullString{}))
	mock.ExpectQuery("INSERT INTO public.releases").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("release-1"))
	mock.ExpectExec("UPDATE public.wishlist_items").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPut, "/wish-1", strings.NewReader(`{"userId":"user-1","title":"Kind of Blue","artist":"Miles Davis","priority":2,"label":"Columbia"}`))
	res := httptest.NewRecorder()

	h.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
}

func TestUpdateReturnsWishlistUpdateErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.ExpectQuery("SELECT release_id").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"release_id"}).AddRow(sql.NullString{}))
	mock.ExpectExec("UPDATE public.wishlist_items").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(assertErr("update failed"))
	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPut, "/wish-1", strings.NewReader(`{"userId":"user-1","title":"Kind of Blue","artist":"Miles Davis"}`))
	res := httptest.NewRecorder()
	h.Routes().ServeHTTP(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestListReturnsQueryErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.ExpectQuery("SELECT w.id").WithArgs(pgxmock.AnyArg()).WillReturnError(assertErr("query failed"))
	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()
	h.list(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestPublicListReturnsWishlistItemsForSharedUser(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT w.id").
		WithArgs("user-1", 50).
		WillReturnRows(pgxmock.NewRows([]string{"id", "priority", "target_price", "pressing_notes", "title", "artist", "label"}).
			AddRow("wish-1", 2, nil, nil, "Kind of Blue", "Miles Davis", "Columbia"))

	res := httptest.NewRecorder()
	NewHandler(mock).PublicRoutes().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/user-1", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "Kind of Blue") {
		t.Fatalf("expected shared wishlist item, got %q", res.Body.String())
	}
}

func TestListReturnsWishlistItems(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	price := 20.0
	notes := "first press"
	mock.ExpectQuery("SELECT w.id").
		WithArgs(pgxmock.AnyArg(), 50).
		WillReturnRows(pgxmock.NewRows([]string{"id", "priority", "target_price", "pressing_notes", "title", "artist", "label"}).
			AddRow("wish-1", 2, &price, &notes, "Kind of Blue", "Miles Davis", "Columbia"))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	h.list(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	for _, want := range []string{"wish-1", "Kind of Blue", "Miles Davis", "first press", "20"} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("expected body to contain %q, got %q", want, res.Body.String())
		}
	}
}

func TestDeleteRequiresID(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	res := httptest.NewRecorder()

	h.delete(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
	if !strings.Contains(res.Body.String(), "id is required") {
		t.Fatalf("expected validation message, got %q", res.Body.String())
	}
}

func TestUpdateReturnsNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT release_id").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(sql.ErrNoRows)

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPut, "/wish-1", strings.NewReader(`{"userId":"user-1","title":"Kind of Blue","artist":"Miles Davis"}`))
	res := httptest.NewRecorder()

	h.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, res.Code)
	}
}

func TestDeleteRemovesWishlistItem(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM public.wishlist_items").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodDelete, "/abc?userId=user-1", nil)
	res := httptest.NewRecorder()

	h.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusNoContent, res.Code, res.Body.String())
	}
}

func TestDeleteReturnsExecErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.ExpectExec("DELETE FROM public.wishlist_items").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(assertErr("delete failed"))
	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodDelete, "/abc?userId=user-1", nil)
	res := httptest.NewRecorder()
	h.Routes().ServeHTTP(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestDeleteReturnsNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM public.wishlist_items").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodDelete, "/abc?userId=user-1", nil)
	res := httptest.NewRecorder()

	h.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, res.Code)
	}
}

func TestPurchaseReturnsBeginErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.ExpectBegin().WillReturnError(assertErr("begin failed"))
	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/wish-1/purchase", strings.NewReader(`{"userId":"user-1"}`))
	res := httptest.NewRecorder()
	h.Routes().ServeHTTP(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestPurchaseReturnsNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT w.release_id").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(sql.ErrNoRows)
	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/wish-1/purchase", strings.NewReader(`{"userId":"user-1"}`))
	res := httptest.NewRecorder()
	h.Routes().ServeHTTP(res, req)
	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, res.Code)
	}
}

func TestPurchaseCreatesReleaseForManualWishlistItem(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT w.release_id").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"release_id", "title", "artist", "year", "label", "cover_url", "pressing_notes"}).
			AddRow(sql.NullString{}, "Kind of Blue", "Miles Davis", nil, nil, nil, nil))
	mock.ExpectQuery("INSERT INTO public.releases").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("release-1"))
	mock.ExpectQuery("INSERT INTO public.collection_items").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("item-1"))
	mock.ExpectExec("DELETE FROM public.wishlist_items").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectCommit()

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/wish-1/purchase", strings.NewReader(`{"userId":"user-1"}`))
	res := httptest.NewRecorder()

	h.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusCreated, res.Code, res.Body.String())
	}
}

func TestPurchaseReturnsCollectionInsertErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT w.release_id").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"release_id", "title", "artist", "year", "label", "cover_url", "pressing_notes"}).AddRow(sql.NullString{String: "release-1", Valid: true}, "Kind of Blue", "Miles Davis", nil, nil, nil, nil))
	mock.ExpectQuery("INSERT INTO public.collection_items").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(assertErr("insert failed"))
	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/wish-1/purchase", strings.NewReader(`{"userId":"user-1"}`))
	res := httptest.NewRecorder()
	h.Routes().ServeHTTP(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestPurchaseReturnsDeleteErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT w.release_id").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"release_id", "title", "artist", "year", "label", "cover_url", "pressing_notes"}).AddRow(sql.NullString{String: "release-1", Valid: true}, "Kind of Blue", "Miles Davis", nil, nil, nil, nil))
	mock.ExpectQuery("INSERT INTO public.collection_items").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("item-1"))
	mock.ExpectExec("DELETE FROM public.wishlist_items").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(assertErr("delete failed"))
	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/wish-1/purchase", strings.NewReader(`{"userId":"user-1"}`))
	res := httptest.NewRecorder()
	h.Routes().ServeHTTP(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestPurchaseReturnsCommitErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT w.release_id").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"release_id", "title", "artist", "year", "label", "cover_url", "pressing_notes"}).AddRow(sql.NullString{String: "release-1", Valid: true}, "Kind of Blue", "Miles Davis", nil, nil, nil, nil))
	mock.ExpectQuery("INSERT INTO public.collection_items").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("item-1"))
	mock.ExpectExec("DELETE FROM public.wishlist_items").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectCommit().WillReturnError(assertErr("commit failed"))
	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/wish-1/purchase", strings.NewReader(`{"userId":"user-1"}`))
	res := httptest.NewRecorder()
	h.Routes().ServeHTTP(res, req)
	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestPurchaseMovesWishlistItemToCollection(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	notes := "first press"
	label := "Columbia"
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT w.release_id").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"release_id", "title", "artist", "year", "label", "cover_url", "pressing_notes"}).
			AddRow(sql.NullString{String: "release-1", Valid: true}, "Kind of Blue", "Miles Davis", nil, &label, nil, &notes))
	mock.ExpectQuery("INSERT INTO public.collection_items").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("item-1"))
	mock.ExpectExec("DELETE FROM public.wishlist_items").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("DELETE", 1))
	mock.ExpectCommit()

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/wish-1/purchase", strings.NewReader(`{"userId":"user-1","mediaCondition":"NM","sleeveCondition":"VG+"}`))
	res := httptest.NewRecorder()

	h.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusCreated, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "item-1") {
		t.Fatalf("expected collection id, got %q", res.Body.String())
	}
}

func TestPurchaseRejectsInvalidJSON(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/abc/purchase", strings.NewReader("{"))
	res := httptest.NewRecorder()

	h.purchase(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestPurchaseRequiresID(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/purchase", strings.NewReader(`{}`))
	res := httptest.NewRecorder()

	h.purchase(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
	if !strings.Contains(res.Body.String(), "id is required") {
		t.Fatalf("expected validation message, got %q", res.Body.String())
	}
}

func TestCreateShareReturnsViewerID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	expectGuardPass(mock, "share")
	mock.ExpectQuery("SELECT id::text").
		WithArgs("miles").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("viewer-1"))
	mock.ExpectExec("INSERT INTO public.wishlist_shares").
		WithArgs("owner-1", "viewer-1", "hi").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	h := NewHandler(mock)
	req := wishlistRequest(http.MethodPost, "/shares", strings.NewReader(`{"username":"miles","message":"hi"}`))
	res := httptest.NewRecorder()

	h.createShare(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusCreated, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "viewer-1") {
		t.Fatalf("expected viewer id, got %q", res.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateShareReturnsNotFoundForUnknownUser(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	expectGuardPass(mock, "share")
	mock.ExpectQuery("SELECT id::text").
		WithArgs("missing").
		WillReturnError(pgx.ErrNoRows)

	h := NewHandler(mock)
	req := wishlistRequest(http.MethodPost, "/shares", strings.NewReader(`{"username":"missing"}`))
	res := httptest.NewRecorder()

	h.createShare(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, res.Code)
	}
}

func TestCreateShareRejectsSelfShare(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	expectGuardPass(mock, "share")
	mock.ExpectQuery("SELECT id::text").
		WithArgs("owner").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("owner-1"))

	h := NewHandler(mock)
	req := wishlistRequest(http.MethodPost, "/shares", strings.NewReader(`{"username":"owner"}`))
	res := httptest.NewRecorder()

	h.createShare(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestCreateShareReturnsDuplicateConflict(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	expectGuardPass(mock, "share")
	mock.ExpectQuery("SELECT id::text").
		WithArgs("miles").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("viewer-1"))
	mock.ExpectExec("INSERT INTO public.wishlist_shares").
		WithArgs("owner-1", "viewer-1", "").
		WillReturnError(&pgconn.PgError{Code: "23505"})

	h := NewHandler(mock)
	req := wishlistRequest(http.MethodPost, "/shares", strings.NewReader(`{"username":"miles"}`))
	res := httptest.NewRecorder()

	h.createShare(res, req)

	if res.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, res.Code)
	}
}

func TestListSharesReturnsOutgoingShares(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	message := "birthday"
	createdAt := time.Date(2026, 6, 16, 11, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT ws.viewer_id::text").
		WithArgs("owner-1").
		WillReturnRows(pgxmock.NewRows([]string{"viewer_id", "username", "display_name", "message", "created_at"}).
			AddRow("viewer-1", "miles", "Miles Davis", &message, createdAt))

	h := NewHandler(mock)
	req := wishlistRequest(http.MethodGet, "/shares", nil)
	res := httptest.NewRecorder()

	h.listShares(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	for _, want := range []string{"viewer-1", "miles", "Miles Davis", "birthday", "2026-06-16T11:00:00Z"} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("expected body to contain %q, got %q", want, res.Body.String())
		}
	}
}

func TestDeleteShareRemovesOwnedShare(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM public.wishlist_shares").
		WithArgs("owner-1", "viewer-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 1))

	h := NewHandler(mock)
	req := wishlistRequestWithParam(http.MethodDelete, "/shares/viewer-1", nil, "viewerID", "viewer-1")
	res := httptest.NewRecorder()

	h.deleteShare(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusNoContent, res.Code, res.Body.String())
	}
}

func TestDeleteShareReturnsNotFoundForMissingShare(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM public.wishlist_shares").
		WithArgs("owner-1", "viewer-1").
		WillReturnResult(pgxmock.NewResult("DELETE", 0))

	h := NewHandler(mock)
	req := wishlistRequestWithParam(http.MethodDelete, "/shares/viewer-1", nil, "viewerID", "viewer-1")
	res := httptest.NewRecorder()

	h.deleteShare(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, res.Code)
	}
}

func TestSharedWithMeReturnsAuthorizedWishlist(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	price := 20.0
	notes := "first press"
	mock.ExpectQuery("SELECT ws.owner_id::text").
		WithArgs("owner-1").
		WillReturnRows(pgxmock.NewRows([]string{"owner_id", "username", "display_name"}).
			AddRow("shared-owner-1", "miles", "Miles Davis"))
	mock.ExpectQuery("SELECT w.id").
		WithArgs("shared-owner-1", 50).
		WillReturnRows(pgxmock.NewRows([]string{"id", "priority", "target_price", "pressing_notes", "title", "artist", "label"}).
			AddRow("wish-1", 2, &price, &notes, "Kind of Blue", "Miles Davis", "Columbia"))

	h := NewHandler(mock)
	req := wishlistRequest(http.MethodGet, "/shared-with-me", nil)
	res := httptest.NewRecorder()

	h.sharedWithMe(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	for _, want := range []string{"shared-owner-1", "miles", "Kind of Blue", "20", "first press"} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("expected body to contain %q, got %q", want, res.Body.String())
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSharedWithMeReturnsEmptyArrayWhenNoShareExists(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT ws.owner_id::text").
		WithArgs("owner-1").
		WillReturnRows(pgxmock.NewRows([]string{"owner_id", "username", "display_name"}))

	h := NewHandler(mock)
	req := wishlistRequest(http.MethodGet, "/shared-with-me", nil)
	res := httptest.NewRecorder()

	h.sharedWithMe(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	if strings.TrimSpace(res.Body.String()) != "[]" {
		t.Fatalf("expected empty array, got %q", res.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func wishlistRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, chi.NewRouteContext())
	ctx = context.WithValue(ctx, auth.UserIDKey, "owner-1")
	return req.WithContext(ctx)
}

func wishlistRequestWithParam(method, target string, body io.Reader, key, value string) *http.Request {
	req := httptest.NewRequest(method, target, body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, auth.UserIDKey, "owner-1")
	return req.WithContext(ctx)
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

var _ = errors.New

func TestCreateShareRejectsInvalidJSON(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	h := NewHandler(mock)
	req := wishlistRequest(http.MethodPost, "/shares", strings.NewReader("{"))
	res := httptest.NewRecorder()

	h.createShare(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestCreateShareRequiresUsername(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	h := NewHandler(mock)
	req := wishlistRequest(http.MethodPost, "/shares", strings.NewReader(`{"username":"  "}`))
	res := httptest.NewRecorder()

	h.createShare(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestCreateShareReturnsServerErrorOnProfileLookupFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	expectGuardPass(mock, "share")
	mock.ExpectQuery("SELECT id::text").
		WithArgs("miles").
		WillReturnError(assertErr("db down"))

	h := NewHandler(mock)
	req := wishlistRequest(http.MethodPost, "/shares", strings.NewReader(`{"username":"miles"}`))
	res := httptest.NewRecorder()

	h.createShare(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateShareReturnsServerErrorOnInsertFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	expectGuardPass(mock, "share")
	mock.ExpectQuery("SELECT id::text").
		WithArgs("miles").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("viewer-1"))
	mock.ExpectExec("INSERT INTO public.wishlist_shares").
		WithArgs("owner-1", "viewer-1", "").
		WillReturnError(assertErr("insert failed"))

	h := NewHandler(mock)
	req := wishlistRequest(http.MethodPost, "/shares", strings.NewReader(`{"username":"miles"}`))
	res := httptest.NewRecorder()

	h.createShare(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListSharesReturnsServerErrorOnQueryFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT ws.viewer_id::text").
		WithArgs("owner-1").
		WillReturnError(assertErr("query failed"))

	h := NewHandler(mock)
	req := wishlistRequest(http.MethodGet, "/shares", nil)
	res := httptest.NewRecorder()

	h.listShares(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListSharesReturnsServerErrorOnScanFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Column count mismatch forces a scan error.
	mock.ExpectQuery("SELECT ws.viewer_id::text").
		WithArgs("owner-1").
		WillReturnRows(pgxmock.NewRows([]string{"viewer_id", "bogus"}).
			AddRow("viewer-1", "oops"))

	h := NewHandler(mock)
	req := wishlistRequest(http.MethodGet, "/shares", nil)
	res := httptest.NewRecorder()

	h.listShares(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestDeleteShareReturnsServerErrorOnExecFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM public.wishlist_shares").
		WithArgs("owner-1", "viewer-1").
		WillReturnError(assertErr("delete failed"))

	h := NewHandler(mock)
	req := wishlistRequestWithParam(http.MethodDelete, "/shares/viewer-1", nil, "viewerID", "viewer-1")
	res := httptest.NewRecorder()

	h.deleteShare(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSharedWithMeReturnsServerErrorOnQueryFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT ws.owner_id::text").
		WithArgs("owner-1").
		WillReturnError(assertErr("query failed"))

	h := NewHandler(mock)
	req := wishlistRequest(http.MethodGet, "/shared-with-me", nil)
	res := httptest.NewRecorder()

	h.sharedWithMe(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSharedWithMeReturnsServerErrorOnScanFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Column count mismatch forces a scan error.
	mock.ExpectQuery("SELECT ws.owner_id::text").
		WithArgs("owner-1").
		WillReturnRows(pgxmock.NewRows([]string{"owner_id", "bogus"}).
			AddRow("shared-owner-1", "oops"))

	h := NewHandler(mock)
	req := wishlistRequest(http.MethodGet, "/shared-with-me", nil)
	res := httptest.NewRecorder()

	h.sharedWithMe(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestSharedWithMeReturnsServerErrorOnItemFetchFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT ws.owner_id::text").
		WithArgs("owner-1").
		WillReturnRows(pgxmock.NewRows([]string{"owner_id", "username", "display_name"}).
			AddRow("shared-owner-1", "miles", "Miles Davis"))
	mock.ExpectQuery("SELECT w.id").
		WithArgs("shared-owner-1", 50).
			WillReturnError(assertErr("items query failed"))

	h := NewHandler(mock)
	req := wishlistRequest(http.MethodGet, "/shared-with-me", nil)
	res := httptest.NewRecorder()

	h.sharedWithMe(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestOptionalStringTrimsBlankStrings(t *testing.T) {
	if optionalString("  ") != nil {
		t.Fatal("expected blank string to return nil")
	}
	got := optionalString(" Blue Note ")
	if got == nil || *got != "Blue Note" {
		t.Fatalf("expected trimmed string, got %#v", got)
	}
}

// expectGuardPass mocks the billing GuardLimit queries for a free-tier user under the limit.
func expectGuardPass(mock pgxmock.PgxPoolIface, action string) {
	// FetchStatus query
	mock.ExpectQuery("SELECT.*FROM public.profiles p.*LEFT JOIN public.subscriptions s").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"tier", "status", "current_period_end", "is_vip", "is_admin"}).
			AddRow("free", "inactive", nil, false, false))

	// Count query based on action
	switch action {
	case "collection":
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM public.collection_items").
			WithArgs(pgxmock.AnyArg()).
			WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	case "wishlist":
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM public.wishlist_items").
			WithArgs(pgxmock.AnyArg()).
			WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	case "share":
		mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM public.wishlist_shares").
			WithArgs(pgxmock.AnyArg()).
			WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(0))
	}
}

func TestCreateReturnsForbiddenWhenWishlistLimitExceeded(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// FetchStatus returns free tier
	mock.ExpectQuery("SELECT.*FROM public.profiles p.*LEFT JOIN public.subscriptions s").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"tier", "status", "current_period_end", "is_vip", "is_admin"}).
			AddRow("free", "inactive", nil, false, false))
	// Count returns at limit
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM public.wishlist_items").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(25))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"userId":"user-1","title":"Kind of Blue","artist":"Miles Davis"}`))
	res := httptest.NewRecorder()

	h.create(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusForbidden, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "wishlist limit") {
		t.Fatalf("expected wishlist limit error, got %q", res.Body.String())
	}
}

func TestCreateShareReturnsForbiddenWhenShareLimitExceeded(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// FetchStatus returns free tier
	mock.ExpectQuery("SELECT.*FROM public.profiles p.*LEFT JOIN public.subscriptions s").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"tier", "status", "current_period_end", "is_vip", "is_admin"}).
			AddRow("free", "inactive", nil, false, false))
	// Count returns at limit
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM public.wishlist_shares").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

	h := NewHandler(mock)
	req := wishlistRequest(http.MethodPost, "/shares", strings.NewReader(`{"username":"miles"}`))
	res := httptest.NewRecorder()

	h.createShare(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusForbidden, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "sharing limit") {
		t.Fatalf("expected sharing limit error, got %q", res.Body.String())
	}
}
