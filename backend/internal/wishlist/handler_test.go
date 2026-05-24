package wishlist

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pgxmock "github.com/pashagolub/pgxmock/v4"
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
	if !strings.Contains(res.Body.String(), "userId, title, and artist are required") {
		t.Fatalf("expected validation message, got %q", res.Body.String())
	}
}

func TestCreateInsertsWishlistItem(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

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
	if !strings.Contains(res.Body.String(), "id, userId, title, and artist are required") {
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

func TestListReturnsWishlistItems(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	price := 20.0
	notes := "first press"
	mock.ExpectQuery("SELECT w.id").
		WithArgs(pgxmock.AnyArg()).
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

func TestDeleteRequiresUserID(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(http.MethodDelete, "/abc", nil)
	res := httptest.NewRecorder()

	h.delete(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
	if !strings.Contains(res.Body.String(), "id and userId are required") {
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

func TestPurchaseRequiresUserID(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(http.MethodPost, "/abc/purchase", strings.NewReader(`{}`))
	res := httptest.NewRecorder()

	h.purchase(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
	if !strings.Contains(res.Body.String(), "id and userId are required") {
		t.Fatalf("expected validation message, got %q", res.Body.String())
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

var _ = errors.New

func TestOptionalStringTrimsBlankStrings(t *testing.T) {
	if optionalString("  ") != nil {
		t.Fatal("expected blank string to return nil")
	}
	got := optionalString(" Blue Note ")
	if got == nil || *got != "Blue Note" {
		t.Fatalf("expected trimmed string, got %#v", got)
	}
}
