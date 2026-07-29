package billing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pgxmock "github.com/pashagolub/pgxmock/v4"
)

func TestBillingEnabledDefaultsTrue(t *testing.T) {
	t.Setenv("BILLING_ENABLED", "")
	if !billingEnabled() {
		t.Fatal("expected billingEnabled true when unset")
	}
}

func TestBillingEnabledFalseOptOut(t *testing.T) {
	t.Setenv("BILLING_ENABLED", "false")
	if billingEnabled() {
		t.Fatal("expected billingEnabled false")
	}
}

func TestGuardLimitDisabledShortCircuits(t *testing.T) {
	t.Setenv("BILLING_ENABLED", "false")
	// Billing disabled -> returns nil without touching the pool.
	if err := GuardLimit(context.Background(), nil, "user-1", "collection"); err != nil {
		t.Fatalf("expected nil when billing disabled, got %v", err)
	}
}

func TestConfigExposesBillingEnabledTrue(t *testing.T) {
	t.Setenv("BILLING_ENABLED", "")
	h := NewHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	res := httptest.NewRecorder()
	h.config(res, req)
	if !strings.Contains(res.Body.String(), `"billingEnabled":true`) {
		t.Fatalf("expected billingEnabled true, got %s", res.Body.String())
	}
}

func TestConfigExposesBillingEnabledFalse(t *testing.T) {
	t.Setenv("BILLING_ENABLED", "false")
	h := NewHandler(nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/config", nil)
	res := httptest.NewRecorder()
	h.config(res, req)
	if !strings.Contains(res.Body.String(), `"billingEnabled":false`) {
		t.Fatalf("expected billingEnabled false, got %s", res.Body.String())
	}
}

func TestCheckoutRejectsWhenBillingDisabled(t *testing.T) {
	t.Setenv("BILLING_ENABLED", "false")
	h := NewHandler(nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/checkout", strings.NewReader(`{"priceId":"pri_x"}`))
	res := httptest.NewRecorder()
	h.checkout(res, req)
	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when billing disabled, got %d", res.Code)
	}
}

func TestStatusUnlimitedWhenBillingDisabled(t *testing.T) {
	t.Setenv("BILLING_ENABLED", "false")
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	mock.ExpectQuery("SELECT").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"tier", "status", "current_period_end", "is_vip", "is_admin"}).
			AddRow("free", "inactive", nil, false, false),
	)
	mock.ExpectQuery("SELECT COUNT.*collection_items").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"count"}).AddRow(999),
	)
	mock.ExpectQuery("SELECT COUNT.*wishlist_items").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"count"}).AddRow(999),
	)
	mock.ExpectQuery("SELECT COUNT.*wishlist_shares").WithArgs("user-1").WillReturnRows(
		pgxmock.NewRows([]string{"count"}).AddRow(999),
	)

	h := NewHandler(mock, nil)
	req := billingRequest(http.MethodGet, "/status", nil)
	res := httptest.NewRecorder()
	h.status(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), `"limit":-1`) {
		t.Fatalf("expected unlimited (-1) when billing disabled, got %s", res.Body.String())
	}
}
