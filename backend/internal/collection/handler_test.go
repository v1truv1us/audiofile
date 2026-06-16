package collection

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	pgxmock "github.com/pashagolub/pgxmock/v4"

	"github.com/v1truv1us/audiofile/backend/internal/auth"
)

func TestRoutesRegistersCollectionEndpoints(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.ExpectQuery("SELECT ci.id").
		WithArgs(pgxmock.AnyArg(), 50, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "media_condition", "sleeve_condition", "purchase_price", "notes", "is_for_sale", "release_id", "title", "artist", "year", "label", "cover_url"}))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	h.Routes().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
}

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
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"userId":"u1","title":"Kind of Blue"}`))
	res := httptest.NewRecorder()

	h.create(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
	if !strings.Contains(res.Body.String(), "title and artist are required") {
		t.Fatalf("expected validation message, got %q", res.Body.String())
	}
}

func TestCreateInsertsReleaseAndCollectionItem(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO public.releases").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("release-1"))
	mock.ExpectQuery("INSERT INTO public.collection_items").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("item-1"))
	mock.ExpectCommit()

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"userId":"user-1","title":"Kind of Blue","artist":"Miles Davis","mediaCondition":"NM","sleeveCondition":"VG+","label":"Columbia"}`))
	res := httptest.NewRecorder()

	h.create(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusCreated, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "item-1") {
		t.Fatalf("expected created id, got %q", res.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateReturnsReleaseInsertErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO public.releases").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(assertErr("insert failed"))
	mock.ExpectRollback()

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"userId":"user-1","title":"Kind of Blue","artist":"Miles Davis"}`))
	res := httptest.NewRecorder()

	h.create(res, req)

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
	mock.ExpectQuery("SELECT ci.id").WithArgs(pgxmock.AnyArg(), 50, 0).WillReturnError(assertErr("query failed"))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	h.list(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestListReturnsScanErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()
	mock.ExpectQuery("SELECT ci.id").
		WithArgs(pgxmock.AnyArg(), 50, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "media_condition", "sleeve_condition", "purchase_price", "notes", "is_for_sale", "release_id", "title", "artist", "year", "label", "cover_url"}).
			AddRow("item-1", "NM", "bad-pointer", "bad-price", nil, false, "release-1", "Kind of Blue", "Miles Davis", nil, "Columbia", nil))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	h.list(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestPublicListReturnsCollectionItemsForSharedUser(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT ci.id").
		WithArgs("user-1", 50, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "media_condition", "sleeve_condition", "purchase_price", "notes", "is_for_sale", "release_id", "title", "artist", "year", "label", "cover_url"}).
			AddRow("item-1", "NM", nil, nil, nil, false, "release-1", "Kind of Blue", "Miles Davis", nil, "Columbia", nil))

	res := httptest.NewRecorder()
	NewHandler(mock).PublicRoutes().ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/user-1", nil))

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "Kind of Blue") {
		t.Fatalf("expected shared collection item, got %q", res.Body.String())
	}
}

func TestListReturnsCollectionItems(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	sleeve := "VG+"
	price := 25.50
	notes := "clean"
	year := 1959
	coverURL := "https://img.example/cover.jpg"
	mock.ExpectQuery("SELECT ci.id").
		WithArgs(pgxmock.AnyArg(), 50, 0).
		WillReturnRows(pgxmock.NewRows([]string{"id", "media_condition", "sleeve_condition", "purchase_price", "notes", "is_for_sale", "release_id", "title", "artist", "year", "label", "cover_url"}).
			AddRow("item-1", "NM", &sleeve, &price, &notes, false, "release-1", "Kind of Blue", "Miles Davis", &year, "Columbia", &coverURL))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/?sort=artist", nil)
	res := httptest.NewRecorder()

	h.list(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	for _, want := range []string{"item-1", "Kind of Blue", "Miles Davis", "NM", "25.5"} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("expected body to contain %q, got %q", want, res.Body.String())
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStatsReturnsCounts(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT COUNT").WithArgs(pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery("SELECT COUNT").WithArgs(pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT SUM").WithArgs(pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"sum"}).AddRow(42.0))
	mock.ExpectQuery("SELECT COUNT").WithArgs(pgxmock.AnyArg()).WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(3))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	res := httptest.NewRecorder()

	h.stats(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
	for _, want := range []string{"\"collectionCount\":2", "\"forSaleCount\":1", "\"wishlistCount\":3"} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("expected body to contain %q, got %q", want, res.Body.String())
		}
	}
}

func TestCreateReturnsCollectionInsertErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO public.releases").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("release-1"))
	mock.ExpectQuery("INSERT INTO public.collection_items").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(assertErr("insert item failed"))
	mock.ExpectRollback()

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"userId":"user-1","title":"Kind of Blue","artist":"Miles Davis","coverUrl":"https://img.example/cover.jpg"}`))
	res := httptest.NewRecorder()

	h.create(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestCreateReturnsCommitErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectBegin()
	mock.ExpectQuery("INSERT INTO public.releases").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("release-1"))
	mock.ExpectQuery("INSERT INTO public.collection_items").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("item-1"))
	mock.ExpectCommit().WillReturnError(assertErr("commit failed"))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"userId":"user-1","title":"Kind of Blue","artist":"Miles Davis","coverUrl":"https://img.example/cover.jpg"}`))
	res := httptest.NewRecorder()

	h.create(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestBackfillCoversUpdatesFoundCovers(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").WillReturnRows(pgxmock.NewRows([]string{"id", "title", "artist"}).AddRow("release-1", "Kind of Blue", "Miles Davis"))
	mock.ExpectExec("UPDATE public.releases").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	coverURL := "https://img.example/cover.jpg"
	h := NewHandler(mock)
	h.coverFinder = func(ctx context.Context, title, artist string) *string {
		if title != "Kind of Blue" || artist != "Miles Davis" {
			t.Fatalf("unexpected cover lookup %q %q", title, artist)
		}
		return &coverURL
	}
	req := httptest.NewRequest(http.MethodPost, "/backfill-covers", nil)
	res := httptest.NewRecorder()

	h.backfillCovers(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "\"checked\":1") || !strings.Contains(res.Body.String(), "\"updated\":1") {
		t.Fatalf("expected one backfilled cover, got %q", res.Body.String())
	}
}

func TestBackfillCoversSkipsMissingCovers(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").WillReturnRows(pgxmock.NewRows([]string{"id", "title", "artist"}).AddRow("release-1", "Unknown", "Artist"))

	h := NewHandler(mock)
	h.coverFinder = func(ctx context.Context, title, artist string) *string { return nil }
	req := httptest.NewRequest(http.MethodPost, "/backfill-covers", nil)
	res := httptest.NewRecorder()

	h.backfillCovers(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "\"checked\":1") || !strings.Contains(res.Body.String(), "\"updated\":0") {
		t.Fatalf("expected checked without update, got %q", res.Body.String())
	}
}

func TestBackfillCoversHandlesNoRows(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").WillReturnRows(pgxmock.NewRows([]string{"id", "title", "artist"}))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/backfill-covers", nil)
	res := httptest.NewRecorder()

	h.backfillCovers(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "\"checked\":0") || !strings.Contains(res.Body.String(), "\"updated\":0") {
		t.Fatalf("expected zero backfill counts, got %q", res.Body.String())
	}
}

func TestBackfillCoversReturnsUpdateErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").WillReturnRows(pgxmock.NewRows([]string{"id", "title", "artist"}).AddRow("release-1", "Kind of Blue", "Miles Davis"))
	mock.ExpectExec("UPDATE public.releases").WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnError(assertErr("update failed"))
	coverURL := "https://img.example/cover.jpg"
	h := NewHandler(mock)
	h.coverFinder = func(ctx context.Context, title, artist string) *string { return &coverURL }
	req := httptest.NewRequest(http.MethodPost, "/backfill-covers", nil)
	res := httptest.NewRecorder()

	h.backfillCovers(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestBackfillCoversReturnsQueryErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").WillReturnError(assertErr("query failed"))

	h := NewHandler(mock)
	req := httptest.NewRequest(http.MethodPost, "/backfill-covers", nil)
	res := httptest.NewRecorder()

	h.backfillCovers(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestUpdateChangesCollectionItem(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").
		WithArgs("item-1", "user-1").
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("item-1"))
	mock.ExpectExec("UPDATE public.collection_items").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), "item-1", "user-1").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	h := NewHandler(mock)
	req := collectionRequestWithParam(http.MethodPut, "/item-1", strings.NewReader(`{"mediaCondition":"NM","sleeveCondition":"VG+","purchasePrice":25.5,"notes":"clean","isForSale":true}`), "id", "item-1")
	res := httptest.NewRecorder()

	h.update(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "item-1") {
		t.Fatalf("expected updated id, got %q", res.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateReturnsNotFoundWhenItemIsNotOwned(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").WithArgs("item-1", "user-1").WillReturnError(pgx.ErrNoRows)

	h := NewHandler(mock)
	req := collectionRequestWithParam(http.MethodPut, "/item-1", strings.NewReader(`{"mediaCondition":"NM"}`), "id", "item-1")
	res := httptest.NewRecorder()

	h.update(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, res.Code)
	}
}

func TestUpdateRejectsInvalidJSON(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").WithArgs("item-1", "user-1").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("item-1"))

	h := NewHandler(mock)
	req := collectionRequestWithParam(http.MethodPut, "/item-1", strings.NewReader(`{`), "id", "item-1")
	res := httptest.NewRecorder()

	h.update(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestUpdateReturnsExecErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").WithArgs("item-1", "user-1").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("item-1"))
	mock.ExpectExec("UPDATE public.collection_items").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), "item-1", "user-1").
		WillReturnError(assertErr("update failed"))

	h := NewHandler(mock)
	req := collectionRequestWithParam(http.MethodPut, "/item-1", strings.NewReader(`{"mediaCondition":"NM"}`), "id", "item-1")
	res := httptest.NewRecorder()

	h.update(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestDeleteRemovesCollectionItem(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM public.collection_items").WithArgs("item-1", "user-1").WillReturnResult(pgxmock.NewResult("DELETE", 1))

	h := NewHandler(mock)
	req := collectionRequestWithParam(http.MethodDelete, "/item-1", nil, "id", "item-1")
	res := httptest.NewRecorder()

	h.delete(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, res.Code)
	}
}

func TestDeleteReturnsNotFound(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM public.collection_items").WithArgs("item-1", "user-1").WillReturnResult(pgxmock.NewResult("DELETE", 0))

	h := NewHandler(mock)
	req := collectionRequestWithParam(http.MethodDelete, "/item-1", nil, "id", "item-1")
	res := httptest.NewRecorder()

	h.delete(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, res.Code)
	}
}

func TestDeleteReturnsExecErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec("DELETE FROM public.collection_items").WithArgs("item-1", "user-1").WillReturnError(assertErr("delete failed"))

	h := NewHandler(mock)
	req := collectionRequestWithParam(http.MethodDelete, "/item-1", nil, "id", "item-1")
	res := httptest.NewRecorder()

	h.delete(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestPublicListRequiresUserID(t *testing.T) {
	h := NewHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	h.publicList(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestListConditionReturnsEntries(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").WithArgs("item-1", "user-1").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("item-1"))
	mock.ExpectQuery("SELECT id::text, media_condition, sleeve_condition").
		WithArgs("item-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "media_condition", "sleeve_condition", "warp_notes", "scratch_notes", "cleaning_notes", "playback_notes", "created_at"}).
			AddRow("condition-1", "NM", "VG+", "flat", "none", "ultrasonic", "quiet", "2026-01-01T00:00:00Z"))

	h := NewHandler(mock)
	req := collectionRequestWithParam(http.MethodGet, "/item-1/condition", nil, "id", "item-1")
	res := httptest.NewRecorder()

	h.listCondition(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusOK, res.Code, res.Body.String())
	}
	for _, want := range []string{"condition-1", "NM", "ultrasonic", "quiet"} {
		if !strings.Contains(res.Body.String(), want) {
			t.Fatalf("expected condition response to contain %q, got %q", want, res.Body.String())
		}
	}
}

func TestListConditionReturnsNotFoundWhenItemIsNotOwned(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").WithArgs("item-1", "user-1").WillReturnError(pgx.ErrNoRows)

	h := NewHandler(mock)
	req := collectionRequestWithParam(http.MethodGet, "/item-1/condition", nil, "id", "item-1")
	res := httptest.NewRecorder()

	h.listCondition(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, res.Code)
	}
}

func TestListConditionReturnsQueryErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").WithArgs("item-1", "user-1").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("item-1"))
	mock.ExpectQuery("SELECT id::text, media_condition, sleeve_condition").WithArgs("item-1").WillReturnError(assertErr("condition query failed"))

	h := NewHandler(mock)
	req := collectionRequestWithParam(http.MethodGet, "/item-1/condition", nil, "id", "item-1")
	res := httptest.NewRecorder()

	h.listCondition(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestListConditionReturnsScanErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").WithArgs("item-1", "user-1").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("item-1"))
	mock.ExpectQuery("SELECT id::text, media_condition, sleeve_condition").
		WithArgs("item-1").
		WillReturnRows(pgxmock.NewRows([]string{"id", "media_condition", "sleeve_condition", "warp_notes", "scratch_notes", "cleaning_notes", "playback_notes", "created_at"}).
			AddRow("condition-1", struct{}{}, "VG+", "flat", "none", "ultrasonic", "quiet", "2026-01-01T00:00:00Z"))

	h := NewHandler(mock)
	req := collectionRequestWithParam(http.MethodGet, "/item-1/condition", nil, "id", "item-1")
	res := httptest.NewRecorder()

	h.listCondition(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestCreateConditionInsertsEntry(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").WithArgs("item-1", "user-1").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("item-1"))
	mock.ExpectQuery("INSERT INTO public.condition_history").
		WithArgs("item-1", "NM", "VG+", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("condition-1"))

	h := NewHandler(mock)
	req := collectionRequestWithParam(http.MethodPost, "/item-1/condition", strings.NewReader(`{"mediaCondition":"NM","sleeveCondition":"VG+","warpNotes":"flat"}`), "id", "item-1")
	res := httptest.NewRecorder()

	h.createCondition(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusCreated, res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "condition-1") {
		t.Fatalf("expected condition id, got %q", res.Body.String())
	}
}

func TestCreateConditionDefaultsMissingConditions(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").WithArgs("item-1", "user-1").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("item-1"))
	mock.ExpectQuery("INSERT INTO public.condition_history").
		WithArgs("item-1", "VG", "VG", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("condition-1"))

	h := NewHandler(mock)
	req := collectionRequestWithParam(http.MethodPost, "/item-1/condition", strings.NewReader(`{}`), "id", "item-1")
	res := httptest.NewRecorder()

	h.createCondition(res, req)

	if res.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d with body %q", http.StatusCreated, res.Code, res.Body.String())
	}
}

func TestCreateConditionReturnsNotFoundWhenItemIsNotOwned(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").WithArgs("item-1", "user-1").WillReturnError(pgx.ErrNoRows)

	h := NewHandler(mock)
	req := collectionRequestWithParam(http.MethodPost, "/item-1/condition", strings.NewReader(`{}`), "id", "item-1")
	res := httptest.NewRecorder()

	h.createCondition(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, res.Code)
	}
}

func TestCreateConditionRejectsInvalidJSON(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").WithArgs("item-1", "user-1").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("item-1"))

	h := NewHandler(mock)
	req := collectionRequestWithParam(http.MethodPost, "/item-1/condition", strings.NewReader(`{`), "id", "item-1")
	res := httptest.NewRecorder()

	h.createCondition(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, res.Code)
	}
}

func TestCreateConditionReturnsInsertErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT id").WithArgs("item-1", "user-1").WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow("item-1"))
	mock.ExpectQuery("INSERT INTO public.condition_history").
		WithArgs("item-1", "NM", "VG", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(assertErr("condition insert failed"))

	h := NewHandler(mock)
	req := collectionRequestWithParam(http.MethodPost, "/item-1/condition", strings.NewReader(`{"mediaCondition":"NM"}`), "id", "item-1")
	res := httptest.NewRecorder()

	h.createCondition(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, res.Code)
	}
}

func TestFindCoverURLUsesCachedResult(t *testing.T) {
	resetCoverCache()
	coverURL := "https://cache.example/cover.jpg"
	key := strings.ToLower(strings.TrimSpace("Cache Hit Album") + "\x00" + strings.TrimSpace("Cache Artist"))
	coverCache.Lock()
	coverCache.items[key] = coverCacheEntry{coverURL: &coverURL, expires: time.Now().Add(time.Hour)}
	coverCache.Unlock()

	got := findCoverURL(context.Background(), "Cache Hit Album", "Cache Artist")
	if got == nil || *got != coverURL {
		t.Fatalf("expected cached cover URL, got %#v", got)
	}
}

func TestFindCoverURLReturnsNilForMusicBrainzStatus(t *testing.T) {
	resetCoverCache()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	oldClient, oldMB, oldCover := coverHTTPClient, musicBrainzCoverBaseURL, coverArtBaseURL
	coverHTTPClient, musicBrainzCoverBaseURL, coverArtBaseURL = server.Client(), server.URL, server.URL
	defer func() { coverHTTPClient, musicBrainzCoverBaseURL, coverArtBaseURL = oldClient, oldMB, oldCover }()

	if got := findCoverURL(context.Background(), "Status Failure Album", "Status Artist"); got != nil {
		t.Fatalf("expected nil cover, got %#v", got)
	}
}

func TestFindCoverURLReturnsNilWhenCoverArtIsMissing(t *testing.T) {
	resetCoverCache()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"releases":[{"id":"missing-cover"}]}`))
	}))
	defer server.Close()
	oldClient, oldMB, oldCover := coverHTTPClient, musicBrainzCoverBaseURL, coverArtBaseURL
	coverHTTPClient, musicBrainzCoverBaseURL, coverArtBaseURL = server.Client(), server.URL, server.URL
	defer func() { coverHTTPClient, musicBrainzCoverBaseURL, coverArtBaseURL = oldClient, oldMB, oldCover }()

	if got := findCoverURL(context.Background(), "Missing Cover Album", "Missing Artist"); got != nil {
		t.Fatalf("expected nil cover, got %#v", got)
	}
}

func collectionRequestWithParam(method, target string, body io.Reader, key, value string) *http.Request {
	req := httptest.NewRequest(method, target, body)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, auth.UserIDKey, "user-1")
	return req.WithContext(ctx)
}

func resetCoverCache() {
	coverCache.Lock()
	coverCache.items = map[string]coverCacheEntry{}
	coverCache.Unlock()
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

func TestFindCoverURLReturnsCoverWhenMusicBrainzAndCoverArtSucceed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"releases":[{"id":"mbid-1"}]}`))
	}))
	defer server.Close()
	oldClient, oldMB, oldCover := coverHTTPClient, musicBrainzCoverBaseURL, coverArtBaseURL
	coverHTTPClient, musicBrainzCoverBaseURL, coverArtBaseURL = server.Client(), server.URL, server.URL
	defer func() { coverHTTPClient, musicBrainzCoverBaseURL, coverArtBaseURL = oldClient, oldMB, oldCover }()

	got := findCoverURL(context.Background(), "Coverage Unique Album", "Coverage Artist")
	if got == nil || *got != server.URL+"/release/mbid-1/front-500" {
		t.Fatalf("expected cover URL, got %#v", got)
	}
}

func TestFindCoverURLReturnsNilForMissingRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"releases":[]}`))
	}))
	defer server.Close()
	oldClient, oldMB, oldCover := coverHTTPClient, musicBrainzCoverBaseURL, coverArtBaseURL
	coverHTTPClient, musicBrainzCoverBaseURL, coverArtBaseURL = server.Client(), server.URL, server.URL
	defer func() { coverHTTPClient, musicBrainzCoverBaseURL, coverArtBaseURL = oldClient, oldMB, oldCover }()

	if got := findCoverURL(context.Background(), "Nope", "Nobody"); got != nil {
		t.Fatalf("expected nil cover, got %#v", got)
	}
}

func TestOptionalStringTrimsBlankStrings(t *testing.T) {
	if optionalString("  ") != nil {
		t.Fatal("expected blank string to return nil")
	}
	got := optionalString(" Columbia ")
	if got == nil || *got != "Columbia" {
		t.Fatalf("expected trimmed string, got %#v", got)
	}
}

func TestDerefStr(t *testing.T) {
	if derefStr(nil) != "" {
		t.Fatal("expected nil pointer to dereference to empty string")
	}
	value := "VG+"
	if derefStr(&value) != value {
		t.Fatalf("expected %q", value)
	}
}
