package billing

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	pgxmock "github.com/pashagolub/pgxmock/v4"

	"github.com/v1truv1us/audiofile/backend/internal/auth"
)

// --- mock PaddleClient ---

type mockPaddle struct {
	createTransactionFn           func(ctx context.Context, priceID string, customData map[string]string) (string, error)
	getCustomerPortalURLFn        func(ctx context.Context, customerID string) (string, error)
	getSubscriptionPeriodEndFn    func(ctx context.Context, subscriptionID string) (string, error)
}

func (m *mockPaddle) CreateTransaction(ctx context.Context, priceID string, customData map[string]string) (string, error) {
	if m.createTransactionFn != nil {
		return m.createTransactionFn(ctx, priceID, customData)
	}
	return "https://buy.paddle.com/mock-checkout", nil
}

func (m *mockPaddle) GetCustomerPortalURL(ctx context.Context, customerID string) (string, error) {
	if m.getCustomerPortalURLFn != nil {
		return m.getCustomerPortalURLFn(ctx, customerID)
	}
	return "https://buy.paddle.com/mock-portal", nil
}

func (m *mockPaddle) GetSubscriptionPeriodEnd(ctx context.Context, subscriptionID string) (string, error) {
	if m.getSubscriptionPeriodEndFn != nil {
		return m.getSubscriptionPeriodEndFn(ctx, subscriptionID)
	}
	return "", nil
}

// --- helpers ---

func billingRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	ctx := context.WithValue(req.Context(), auth.UserIDKey, "user-1")
	return req.WithContext(ctx)
}

func adminBillingRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	rctx := chi.NewRouteContext()
	ctx := context.WithValue(req.Context(), auth.UserIDKey, "admin-1")
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	return req.WithContext(ctx)
}

func makePaddleSignature(payload []byte, secret string) string {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	signedPayload := ts + ":" + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	h1 := hex.EncodeToString(mac.Sum(nil))
	return "ts=" + ts + ";h1=" + h1
}

// --- UserStatus / IsPremium tests ---

func TestIsPremiumVIP(t *testing.T) {
	us := &UserStatus{IsVIP: true, Tier: "free", Status: "inactive"}
	if !us.IsPremium() {
		t.Fatal("expected VIP user to be premium")
	}
}

func TestIsPremiumActiveSubscription(t *testing.T) {
	us := &UserStatus{Tier: "premium", Status: "active"}
	if !us.IsPremium() {
		t.Fatal("expected active premium to be premium")
	}
}

func TestIsPremiumTrialingSubscription(t *testing.T) {
	us := &UserStatus{Tier: "premium", Status: "trialing"}
	if !us.IsPremium() {
		t.Fatal("expected trialing premium to be premium")
	}
}

func TestIsNotPremiumFree(t *testing.T) {
	us := &UserStatus{Tier: "free", Status: "inactive"}
	if us.IsPremium() {
		t.Fatal("expected free inactive to not be premium")
	}
}

func TestIsNotPremiumCanceled(t *testing.T) {
	us := &UserStatus{Tier: "premium", Status: "canceled"}
	if us.IsPremium() {
		t.Fatal("expected canceled premium to not be premium")
	}
}

func TestIsNotPremiumPastDue(t *testing.T) {
	us := &UserStatus{Tier: "premium", Status: "past_due"}
	if us.IsPremium() {
		t.Fatal("expected past_due premium to not be premium")
	}
}

// --- FetchStatus tests ---

func TestFetchStatusReturnsSubscriptionData(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	cpe := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"tier", "status", "current_period_end", "is_vip", "is_admin"}).
			AddRow("premium", "active", &cpe, false, false),
	)

	us, err := FetchStatus(context.Background(), mock, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if us.Tier != "premium" || us.Status != "active" || us.IsVIP || us.IsAdmin {
		t.Fatalf("unexpected status: %+v", us)
	}
	if !us.CurrentPeriodEnd.Equal(cpe) {
		t.Fatalf("expected current_period_end %v, got %v", cpe, us.CurrentPeriodEnd)
	}
}

func TestFetchStatusReturnsDefaultsForNoRows(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	// Empty result set triggers Scan error → treated as no-rows default path
	mock.ExpectQuery("SELECT").WithArgs("no-user").WillReturnRows(
		pgxmock.NewRows([]string{"tier", "status", "current_period_end", "is_vip", "is_admin"}),
	)

	us, err := FetchStatus(context.Background(), mock, "no-user")
	if err != nil {
		t.Fatal(err)
	}
	if us.Tier != "free" || us.Status != "inactive" {
		t.Fatalf("expected defaults, got tier=%s status=%s", us.Tier, us.Status)
	}
}

func TestFetchStatusReturnsDBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").WithArgs("user-1").WillReturnError(errors.New("connection refused"))

	_, err = FetchStatus(context.Background(), mock, "user-1")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFetchStatusHandlesNilCurrentPeriodEnd(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"tier", "status", "current_period_end", "is_vip", "is_admin"}).
			AddRow("free", "inactive", nil, false, false),
	)

	us, err := FetchStatus(context.Background(), mock, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if !us.CurrentPeriodEnd.IsZero() {
		t.Fatalf("expected zero time for nil current_period_end, got %v", us.CurrentPeriodEnd)
	}
}

// --- GuardLimit tests ---

func TestGuardLimitAllowsPremiumUser(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").WithArgs("vip-user").WillReturnRows(
		pgxmock.NewRows([]string{"tier", "status", "current_period_end", "is_vip", "is_admin"}).
			AddRow("free", "inactive", nil, true, false),
	)

	err = GuardLimit(context.Background(), mock, "vip-user", "collection")
	if err != nil {
		t.Fatalf("expected nil for VIP user, got %v", err)
	}
}

func TestGuardLimitBlocksCollectionOverLimit(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"tier", "status", "current_period_end", "is_vip", "is_admin"}).
			AddRow("free", "inactive", nil, false, false),
	)
	mock.ExpectQuery("SELECT COUNT").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"count"}).AddRow(50),
	)

	err = GuardLimit(context.Background(), mock, "user-1", "collection")
	if !errors.Is(err, ErrCollectionLimitExceeded) {
		t.Fatalf("expected ErrCollectionLimitExceeded, got %v", err)
	}
}

func TestGuardLimitAllowsCollectionUnderLimit(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"tier", "status", "current_period_end", "is_vip", "is_admin"}).
			AddRow("free", "inactive", nil, false, false),
	)
	mock.ExpectQuery("SELECT COUNT").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"count"}).AddRow(10),
	)

	err = GuardLimit(context.Background(), mock, "user-1", "collection")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestGuardLimitBlocksWishlistOverLimit(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"tier", "status", "current_period_end", "is_vip", "is_admin"}).
			AddRow("free", "inactive", nil, false, false),
	)
	mock.ExpectQuery("SELECT COUNT").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"count"}).AddRow(25),
	)

	err = GuardLimit(context.Background(), mock, "user-1", "wishlist")
	if !errors.Is(err, ErrWishlistLimitExceeded) {
		t.Fatalf("expected ErrWishlistLimitExceeded, got %v", err)
	}
}

func TestGuardLimitBlocksShareOverLimit(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"tier", "status", "current_period_end", "is_vip", "is_admin"}).
			AddRow("free", "inactive", nil, false, false),
	)
	mock.ExpectQuery("SELECT COUNT").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"count"}).AddRow(1),
	)

	err = GuardLimit(context.Background(), mock, "user-1", "share")
	if !errors.Is(err, ErrShareLimitExceeded) {
		t.Fatalf("expected ErrShareLimitExceeded, got %v", err)
	}
}

func TestGuardLimitReturnsFetchError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").WithArgs("user-1").WillReturnError(errors.New("db down"))

	err = GuardLimit(context.Background(), mock, "user-1", "collection")
	if err == nil {
		t.Fatal("expected error from fetch failure")
	}
}

func TestGuardLimitReturnsCountError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"tier", "status", "current_period_end", "is_vip", "is_admin"}).
			AddRow("free", "inactive", nil, false, false),
	)
	mock.ExpectQuery("SELECT COUNT").WithArgs("user-1").WillReturnError(errors.New("count failed"))

	err = GuardLimit(context.Background(), mock, "user-1", "collection")
	if err == nil {
		t.Fatal("expected error from count query failure")
	}
}

func TestGuardLimitUnknownActionPasses(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"tier", "status", "current_period_end", "is_vip", "is_admin"}).
			AddRow("free", "inactive", nil, false, false),
	)

	err = GuardLimit(context.Background(), mock, "user-1", "unknown_action")
	if err != nil {
		t.Fatalf("expected nil for unknown action, got %v", err)
	}
}

// --- Handler: checkout tests ---

func TestCheckoutRejectsInvalidJSON(t *testing.T) {
	h := NewHandler(nil, nil)
	req := billingRequest(http.MethodPost, "/checkout", strings.NewReader("{"))
	res := httptest.NewRecorder()
	h.checkout(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}
}

func TestCheckoutRejectsEmptyPriceID(t *testing.T) {
	h := NewHandler(nil, nil)
	req := billingRequest(http.MethodPost, "/checkout", strings.NewReader(`{"priceId":""}`))
	res := httptest.NewRecorder()
	h.checkout(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}
}

func TestCheckoutCreatesTransaction(t *testing.T) {
	paddle := &mockPaddle{}
	h := NewHandler(nil, paddle)
	req := billingRequest(http.MethodPost, "/checkout", strings.NewReader(`{"priceId":"pri_test"}`))
	res := httptest.NewRecorder()
	h.checkout(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "checkoutUrl") {
		t.Fatalf("expected checkoutUrl in response, got %s", res.Body.String())
	}
}

func TestCheckoutPassesCustomData(t *testing.T) {
	var capturedCustomData map[string]string
	paddle := &mockPaddle{
		createTransactionFn: func(ctx context.Context, priceID string, customData map[string]string) (string, error) {
			capturedCustomData = customData
			return "https://buy.paddle.com/checkout", nil
		},
	}
	h := NewHandler(nil, paddle)
	req := billingRequest(http.MethodPost, "/checkout", strings.NewReader(`{"priceId":"pri_test"}`))
	res := httptest.NewRecorder()
	h.checkout(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if capturedCustomData["user_id"] != "user-1" {
		t.Fatalf("expected custom_data user_id=user-1, got %v", capturedCustomData)
	}
}

func TestCheckoutReturnsErrorWhenPaddleFails(t *testing.T) {
	paddle := &mockPaddle{
		createTransactionFn: func(ctx context.Context, priceID string, customData map[string]string) (string, error) {
			return "", errors.New("paddle api error")
		},
	}
	h := NewHandler(nil, paddle)
	req := billingRequest(http.MethodPost, "/checkout", strings.NewReader(`{"priceId":"pri_test"}`))
	res := httptest.NewRecorder()
	h.checkout(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", res.Code)
	}
}

func TestCheckoutReturnsErrorWhenNoPaddleClient(t *testing.T) {
	h := NewHandler(nil, nil)
	req := billingRequest(http.MethodPost, "/checkout", strings.NewReader(`{"priceId":"pri_test"}`))
	res := httptest.NewRecorder()
	h.checkout(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", res.Code)
	}
}

// --- Handler: portal tests ---

func TestPortalReturnsNotFoundWhenNoSubscription(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT paddle_customer_id").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"paddle_customer_id"}),
	)

	h := NewHandler(mock, &mockPaddle{})
	req := billingRequest(http.MethodPost, "/portal", nil)
	res := httptest.NewRecorder()
	h.portal(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", res.Code)
	}
}

func TestPortalReturnsPortalURL(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT paddle_customer_id").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"paddle_customer_id"}).AddRow("ctm_123"),
	)

	h := NewHandler(mock, &mockPaddle{})
	req := billingRequest(http.MethodPost, "/portal", nil)
	res := httptest.NewRecorder()
	h.portal(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "portalUrl") {
		t.Fatalf("expected portalUrl, got %s", res.Body.String())
	}
}

func TestPortalReturnsErrorOnDBFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT paddle_customer_id").WithArgs("user-1").WillReturnError(errors.New("db error"))

	h := NewHandler(mock, &mockPaddle{})
	req := billingRequest(http.MethodPost, "/portal", nil)
	res := httptest.NewRecorder()
	h.portal(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", res.Code)
	}
}

func TestPortalReturnsNotFoundForEmptyCustomerID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT paddle_customer_id").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"paddle_customer_id"}).AddRow(""),
	)

	h := NewHandler(mock, &mockPaddle{})
	req := billingRequest(http.MethodPost, "/portal", nil)
	res := httptest.NewRecorder()
	h.portal(res, req)

	if res.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", res.Code)
	}
}

func TestPortalReturnsErrorWhenNoPaddleClient(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT paddle_customer_id").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"paddle_customer_id"}).AddRow("ctm_123"),
	)

	h := NewHandler(mock, nil)
	req := billingRequest(http.MethodPost, "/portal", nil)
	res := httptest.NewRecorder()
	h.portal(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", res.Code)
	}
}

func TestPortalReturnsErrorWhenPaddleFails(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT paddle_customer_id").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"paddle_customer_id"}).AddRow("ctm_123"),
	)

	paddle := &mockPaddle{
		getCustomerPortalURLFn: func(ctx context.Context, customerID string) (string, error) {
			return "", errors.New("portal error")
		},
	}
	h := NewHandler(mock, paddle)
	req := billingRequest(http.MethodPost, "/portal", nil)
	res := httptest.NewRecorder()
	h.portal(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", res.Code)
	}
}

// --- Handler: status tests ---

func TestStatusReturnsFullResponse(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	cpe := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	// FetchStatus query
	mock.ExpectQuery("SELECT").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"tier", "status", "current_period_end", "is_vip", "is_admin"}).
			AddRow("free", "inactive", &cpe, false, false),
	)
	// Usage counts
	mock.ExpectQuery("SELECT COUNT.*collection_items").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"count"}).AddRow(12),
	)
	mock.ExpectQuery("SELECT COUNT.*wishlist_items").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"count"}).AddRow(5),
	)
	mock.ExpectQuery("SELECT COUNT.*wishlist_shares").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"count"}).AddRow(0),
	)

	h := NewHandler(mock, nil)
	req := billingRequest(http.MethodGet, "/status", nil)
	res := httptest.NewRecorder()
	h.status(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	body := res.Body.String()
	for _, want := range []string{`"tier":"free"`, `"status":"inactive"`, `"isVip":false`, `"used":12`, `"limit":50`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected body to contain %q, got %s", want, body)
		}
	}
}

func TestStatusReturnsUnlimitedForPremium(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"tier", "status", "current_period_end", "is_vip", "is_admin"}).
			AddRow("premium", "active", nil, false, false),
	)
	mock.ExpectQuery("SELECT COUNT.*collection_items").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"count"}).AddRow(100),
	)
	mock.ExpectQuery("SELECT COUNT.*wishlist_items").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"count"}).AddRow(50),
	)
	mock.ExpectQuery("SELECT COUNT.*wishlist_shares").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"count"}).AddRow(5),
	)

	h := NewHandler(mock, nil)
	req := billingRequest(http.MethodGet, "/status", nil)
	res := httptest.NewRecorder()
	h.status(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if !strings.Contains(res.Body.String(), `"limit":-1`) {
		t.Fatalf("expected unlimited (-1) limits for premium, got %s", res.Body.String())
	}
}

func TestStatusReturnsErrorOnFetchFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").WithArgs("user-1").WillReturnError(errors.New("db error"))

	h := NewHandler(mock, nil)
	req := billingRequest(http.MethodGet, "/status", nil)
	res := httptest.NewRecorder()
	h.status(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", res.Code)
	}
}

// --- Handler: setVIP tests ---

func TestSetVIPRejectsNonAdmin(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT is_admin").WithArgs("admin-1").WillReturnRows(
		pgxmock.NewRows([]string{"is_admin"}).AddRow(false),
	)

	h := NewHandler(mock, nil)
	req := adminBillingRequest(http.MethodPost, "/vip", strings.NewReader(`{"userId":"target-user","isVip":true}`))
	res := httptest.NewRecorder()
	h.setVIP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", res.Code)
	}
}

func TestSetVIPOK(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT is_admin").WithArgs("admin-1").WillReturnRows(
		pgxmock.NewRows([]string{"is_admin"}).AddRow(true),
	)
	mock.ExpectExec("UPDATE public.profiles").WithArgs(true, "target-user").WillReturnResult(
		pgxmock.NewResult("UPDATE", 1),
	)

	h := NewHandler(mock, nil)
	req := adminBillingRequest(http.MethodPost, "/vip", strings.NewReader(`{"userId":"target-user","isVip":true}`))
	res := httptest.NewRecorder()
	h.setVIP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"status":"success"`) {
		t.Fatalf("expected success response, got %s", res.Body.String())
	}
}

func TestSetVIPRejectsInvalidJSON(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT is_admin").WithArgs("admin-1").WillReturnRows(
		pgxmock.NewRows([]string{"is_admin"}).AddRow(true),
	)

	h := NewHandler(mock, nil)
	req := adminBillingRequest(http.MethodPost, "/vip", strings.NewReader(`{`))
	res := httptest.NewRecorder()
	h.setVIP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}
}

func TestSetVIPRejectsEmptyUserID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT is_admin").WithArgs("admin-1").WillReturnRows(
		pgxmock.NewRows([]string{"is_admin"}).AddRow(true),
	)

	h := NewHandler(mock, nil)
	req := adminBillingRequest(http.MethodPost, "/vip", strings.NewReader(`{"userId":"","isVip":true}`))
	res := httptest.NewRecorder()
	h.setVIP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}
}

func TestSetVIPReturnsErrorOnAdminCheckFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT is_admin").WithArgs("admin-1").WillReturnError(errors.New("db error"))

	h := NewHandler(mock, nil)
	req := adminBillingRequest(http.MethodPost, "/vip", strings.NewReader(`{"userId":"u","isVip":true}`))
	res := httptest.NewRecorder()
	h.setVIP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", res.Code)
	}
}

func TestSetVIPReturnsErrorOnUpdateFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT is_admin").WithArgs("admin-1").WillReturnRows(
		pgxmock.NewRows([]string{"is_admin"}).AddRow(true),
	)
	mock.ExpectExec("UPDATE public.profiles").WithArgs(true, "target-user").WillReturnError(errors.New("update failed"))

	h := NewHandler(mock, nil)
	req := adminBillingRequest(http.MethodPost, "/vip", strings.NewReader(`{"userId":"target-user","isVip":true}`))
	res := httptest.NewRecorder()
	h.setVIP(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", res.Code)
	}
}

// --- Handler: webhook tests ---

func TestWebhookAcceptsValidSignature(t *testing.T) {
	// Use unknown event type so webhook returns early after signature check
	payload := `{"event_type":"unknown.event","data":{}}`
	sig := makePaddleSignature([]byte(payload), "")
	h := NewHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	req.Header.Set("paddle-signature", sig)
	res := httptest.NewRecorder()
	h.webhook(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
}

func TestWebhookRejectsMissingSignature(t *testing.T) {
	h := NewHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"event_type":"test"}`))
	res := httptest.NewRecorder()
	h.webhook(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}
}

func TestWebhookRejectsInvalidSignature(t *testing.T) {
	h := NewHandler(nil, nil)
	h.SetSignatureValidator(func(payload []byte, header string) error {
		return errors.New("invalid signature")
	})
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"event_type":"test"}`))
	req.Header.Set("paddle-signature", "bad")
	res := httptest.NewRecorder()
	h.webhook(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}
}

func TestWebhookHMACValidation(t *testing.T) {
	secret := "test-webhook-secret"
	os.Setenv("PADDLE_WEBHOOK_SECRET", secret)
	defer os.Unsetenv("PADDLE_WEBHOOK_SECRET")

	// Use unknown event type so we only test signature validation, not DB logic
	payload := `{"event_type":"unknown.event","data":{}}`
	sig := makePaddleSignature([]byte(payload), secret)

	h := NewHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	req.Header.Set("paddle-signature", sig)
	res := httptest.NewRecorder()
	h.webhook(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid HMAC, got %d: %s", res.Code, res.Body.String())
	}
}

func TestWebhookRejectsBadHMAC(t *testing.T) {
	secret := "test-webhook-secret"
	os.Setenv("PADDLE_WEBHOOK_SECRET", secret)
	defer os.Unsetenv("PADDLE_WEBHOOK_SECRET")

	payload := `{"event_type":"unknown.event","data":{}}`

	h := NewHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	req.Header.Set("paddle-signature", "ts=12345;h1=invalidsignature")
	res := httptest.NewRecorder()
	h.webhook(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 with bad HMAC, got %d: %s", res.Code, res.Body.String())
	}
}

func TestWebhookRejectsMalformedSignatureFormat(t *testing.T) {
	h := NewHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"event_type":"test"}`))
	req.Header.Set("paddle-signature", "not-valid-format")
	res := httptest.NewRecorder()
	h.webhook(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed signature, got %d: %s", res.Code, res.Body.String())
	}
}

func TestWebhookTransactionCompleted(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`INSERT INTO public.subscriptions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	h := NewHandler(mock, nil)
	payload := `{"event_type":"transaction.completed","data":{"id":"txn_1","customer_id":"ctm_1","status":"completed","custom_data":{"user_id":"user-1"},"current_billing_period":{"ends_at":"2026-07-01T00:00:00Z"},"items":[{"price":{"id":"pri_1"}}]}}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	req.Header.Set("paddle-signature", "ts=123;h1=abc")
	res := httptest.NewRecorder()
	h.webhook(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
}

func TestWebhookSubscriptionCreated(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`INSERT INTO public.subscriptions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	h := NewHandler(mock, nil)
	payload := `{"event_type":"subscription.created","data":{"id":"sub_1","customer_id":"ctm_1","status":"active","custom_data":{"user_id":"user-1"},"current_billing_period":{"ends_at":"2026-07-01T00:00:00Z"},"items":[{"price":{"id":"pri_1"}}]}}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	req.Header.Set("paddle-signature", "ts=123;h1=abc")
	res := httptest.NewRecorder()
	h.webhook(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
}

func TestWebhookSubscriptionUpdated(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`INSERT INTO public.subscriptions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	h := NewHandler(mock, nil)
	payload := `{"event_type":"subscription.updated","data":{"id":"sub_1","customer_id":"ctm_1","status":"past_due","custom_data":{"user_id":"user-1"},"items":[{"price":{"id":"pri_1"}}]}}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	req.Header.Set("paddle-signature", "ts=123;h1=abc")
	res := httptest.NewRecorder()
	h.webhook(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
}

func TestWebhookSubscriptionCanceled(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`INSERT INTO public.subscriptions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	h := NewHandler(mock, nil)
	payload := `{"event_type":"subscription.canceled","data":{"id":"sub_1","customer_id":"ctm_1","status":"canceled","custom_data":{"user_id":"user-1"},"items":[{"price":{"id":"pri_1"}}]}}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	req.Header.Set("paddle-signature", "ts=123;h1=abc")
	res := httptest.NewRecorder()
	h.webhook(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
}

func TestWebhookSubscriptionPaused(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`INSERT INTO public.subscriptions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	h := NewHandler(mock, nil)
	payload := `{"event_type":"subscription.paused","data":{"id":"sub_1","customer_id":"ctm_1","status":"paused","custom_data":{"user_id":"user-1"},"items":[{"price":{"id":"pri_1"}}]}}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	req.Header.Set("paddle-signature", "ts=123;h1=abc")
	res := httptest.NewRecorder()
	h.webhook(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
}

func TestWebhookUnknownEventType(t *testing.T) {
	h := NewHandler(nil, nil)
	payload := `{"event_type":"invoice.created","data":{"id":"inv_1"}}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	req.Header.Set("paddle-signature", "ts=123;h1=abc")
	res := httptest.NewRecorder()
	h.webhook(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200 for unknown event, got %d: %s", res.Code, res.Body.String())
	}
}

func TestWebhookMissingUserID(t *testing.T) {
	h := NewHandler(nil, nil)
	payload := `{"event_type":"subscription.created","data":{"id":"sub_1","customer_id":"ctm_1","status":"active","custom_data":{},"items":[{"price":{"id":"pri_1"}}]}}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	req.Header.Set("paddle-signature", "ts=123;h1=abc")
	res := httptest.NewRecorder()
	h.webhook(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200 for missing user_id, got %d: %s", res.Code, res.Body.String())
	}
}

func TestWebhookDBFailure(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectExec(`INSERT INTO public.subscriptions`).
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnError(errors.New("db error"))

	h := NewHandler(mock, nil)
	payload := `{"event_type":"subscription.created","data":{"id":"sub_1","customer_id":"ctm_1","status":"active","custom_data":{"user_id":"user-1"},"items":[{"price":{"id":"pri_1"}}]}}`
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(payload))
	req.Header.Set("paddle-signature", "ts=123;h1=abc")
	res := httptest.NewRecorder()
	h.webhook(res, req)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", res.Code, res.Body.String())
	}
}

func TestWebhookInvalidJSON(t *testing.T) {
	h := NewHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{invalid`))
	req.Header.Set("paddle-signature", "ts=123;h1=abc")
	res := httptest.NewRecorder()
	h.webhook(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.Code)
	}
}

// --- Routes registration test ---

func TestRoutesRegistersEndpoints(t *testing.T) {
	h := NewHandler(nil, nil)
	r := h.Routes()
	if r == nil {
		t.Fatal("expected non-nil router")
	}
}

func TestAdminRoutesRegistersEndpoints(t *testing.T) {
	h := NewHandler(nil, nil)
	r := h.AdminRoutes()
	if r == nil {
		t.Fatal("expected non-nil router")
	}
}

// --- mapPaddleStatus tests ---

func TestMapPaddleStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"active", "active"},
		{"trialing", "trialing"},
		{"past_due", "past_due"},
		{"canceled", "canceled"},
		{"paused", "paused"},
		{"unpaid", "unpaid"},
		{"unknown_status", "inactive"},
		{"", "inactive"},
	}
	for _, tt := range tests {
		got := mapPaddleStatus(tt.input)
		if got != tt.expected {
			t.Errorf("mapPaddleStatus(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// --- Webhook export test ---

func TestWebhookExportDelegatesToInternal(t *testing.T) {
	h := NewHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{invalid`))
	req.Header.Set("paddle-signature", "ts=123;h1=abc")
	res := httptest.NewRecorder()
	h.Webhook(res, req)

	// Should behave same as internal webhook — invalid JSON returns 400
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from exported Webhook, got %d", res.Code)
	}
}

// --- NoOpPaddleClient test ---

func TestNoOpPaddleClientReturnsErrors(t *testing.T) {
	c := &NoOpPaddleClient{}
	_, err := c.CreateTransaction(context.Background(), "pri_1", nil)
	if err == nil {
		t.Fatal("expected error from NoOpPaddleClient.CreateTransaction")
	}
	_, err = c.GetCustomerPortalURL(context.Background(), "ctm_1")
	if err == nil {
		t.Fatal("expected error from NoOpPaddleClient.GetCustomerPortalURL")
	}
}
