# ADR-009: Monetization, Paywalling, and VIP Exemption

## Status
Accepted

## Context
AudioFile is transitioning from a completely free application to a freemium monetization model. To support this, we require:
1. **Subscription Tiers**: A **Free Tier** with item storage limits and a **Premium Tier** with unlimited storage and sharing.
2. **VIP Exemption**: An administrative override mechanism to exempt specific users (e.g. friends, testers, VIP collectors) from subscription requirements or paywall limits.
3. **Billing Integration**: Paddle integration for subscriptions, including secure webhook signature validation, Paddle.js overlay checkout, and a self-service customer portal.
4. **Paywall Enforcements**: Guarding collection additions, wishlist additions, and wishlist sharing actions.
5. **Downgrade Resilience**: If a user's premium subscription expires or is canceled, we must not delete any existing data. Instead, we disable adding new items or creating new shares until they are below the Free Tier limits or resubscribe.

### Key Security & Schema Challenges
* **Profiles Table Publicity**: In ADR-008, the `public.profiles` table was made globally readable to support username searches (`Profiles are readable` SELECT USING `true`). Sensitive billing identifiers (Paddle Customer ID, Subscription ID, price ID, and payment status) must never be readable by other users.
* **Self-Elevation Risk**: If administrative flags like `is_vip` and `is_admin` are placed on the `profiles` table, a standard user could potentially elevate their privileges by issuing direct updates via the client API (since users have UPDATE policy access to their own profile rows).

---

## Decision

We will implement the monetization architecture through five core areas: DB Migrations, Go API Contracts, Go Backend logic, Frontend Svelte/Astro components, and Edge Case management.

### 1. Database Schema Design & Security Trigger

We will separate billing metadata from the public profile table into a dedicated `public.subscriptions` table. This table will have strict RLS, readable only by the owner and writeable only by system-level database roles. 

To protect against self-elevation of administrative flags `is_vip` and `is_admin` (which are added to `public.profiles` to allow UI badge decoration), we will implement a PostgreSQL `BEFORE UPDATE` trigger that intercepts client-initiated profile changes and prevents standard users from altering their VIP or Admin statuses.

#### Migrations: `supabase/migrations/00004_monetization_and_billing.sql` and `00005_paddle_migration.sql`

The monetization schema was introduced in `00004_monetization_and_billing.sql` and later adapted for Paddle in `00005_paddle_migration.sql`. The relevant structure is:

```sql
-- Migration 00004/00005: Monetization, Billing, and VIP Exemption

-- 1. Extend public.profiles with administrative and VIP flags
ALTER TABLE public.profiles ADD COLUMN IF NOT EXISTS is_vip BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE public.profiles ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT false;

-- 2. Prevent self-elevation of is_vip and is_admin by users
CREATE OR REPLACE FUNCTION public.prevent_profile_elevation()
RETURNS TRIGGER AS $$
BEGIN
    -- If the update is made by the user themselves, prevent modifying is_vip or is_admin
    IF auth.uid() = NEW.id THEN
        IF NEW.is_vip IS DISTINCT FROM OLD.is_vip THEN
            NEW.is_vip := OLD.is_vip;
        END IF;
        IF NEW.is_admin IS DISTINCT FROM OLD.is_admin THEN
            NEW.is_admin := OLD.is_admin;
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

DROP TRIGGER IF EXISTS enforce_profile_elevation ON public.profiles;
CREATE TRIGGER enforce_profile_elevation
    BEFORE UPDATE ON public.profiles
    FOR EACH ROW EXECUTE FUNCTION public.prevent_profile_elevation();

-- 3. Create public.subscriptions table (isolated from globally-readable profiles)
CREATE TABLE IF NOT EXISTS public.subscriptions (
    id                     TEXT PRIMARY KEY, -- Paddle Subscription ID
    user_id                UUID NOT NULL UNIQUE REFERENCES auth.users(id) ON DELETE CASCADE,
    paddle_customer_id     TEXT UNIQUE,
    price_id               TEXT,
    tier                   TEXT NOT NULL DEFAULT 'free' CHECK (tier IN ('free', 'premium')),
    status                 TEXT NOT NULL DEFAULT 'inactive' CHECK (status IN (
        'active', 'trialing', 'past_due', 'canceled', 'unpaid', 'incomplete', 'incomplete_expired', 'inactive', 'paused'
    )),
    current_period_end     TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 4. Create indexes for high-performance querying
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON public.subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_paddle_customer_id ON public.subscriptions(paddle_customer_id);

-- 5. Enable Row-Level Security
ALTER TABLE public.subscriptions ENABLE ROW LEVEL SECURITY;

-- 6. RLS Policies
-- Subscriptions are only readable by their owner
DROP POLICY IF EXISTS "Users can view own subscription" ON public.subscriptions;
CREATE POLICY "Users can view own subscription" ON public.subscriptions
    FOR SELECT USING (auth.uid() = user_id);

-- 7. Trigger for updated_at tracking
DROP TRIGGER IF EXISTS set_subscriptions_updated_at ON public.subscriptions;
CREATE TRIGGER set_subscriptions_updated_at BEFORE UPDATE ON public.subscriptions
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();
```

---

### 2. Go API Contracts & Endpoint Definitions

A new endpoint suite `/api/billing` and admin suite `/api/admin` will be registered in the Go backend routing configuration:

#### A. Billing Configuration (`GET /api/billing/config`)
Returns the Paddle price ID, environment, and client token needed to initialize Paddle.js on the frontend.
* **Authentication**: Required.
* **Success Response (200 OK)**:
  ```json
  {
    "premiumMonthlyPriceId": "pri_01hxyz...",
    "environment": "sandbox",
    "clientToken": "test_abc123..."
  }
  ```
* **Error Responses**:
  * `401 Unauthorized`: Missing or invalid bearer token.
  * `500 Internal Server Error`: Failed to load billing configuration.

`POST /api/billing/checkout` remains implemented for server-side transaction creation, but the current overlay checkout flow does not use it.

#### B. Customer Portal Session (`POST /api/billing/portal`)
Creates a Paddle Customer Portal session so users can manage payment methods, download invoices, or cancel/upgrade plans.
* **Authentication**: Required.
* **Request Body**: None (inferred from JWT authenticated context).
* **Success Response (200 OK)**:
  ```json
  {
    "portalUrl": "https://checkout.paddle.com/customer-portal/cm_..."
  }
  ```
* **Error Responses**:
  * `401 Unauthorized`: Missing or invalid bearer token.
  * `404 Not Found`: User does not have an active Paddle customer profile.
  * `500 Internal Server Error`: Failed to create portal session.

#### C. Billing & Usage Status (`GET /api/billing/status`)
Retrieves user status, VIP details, and current limits/usages.
* **Authentication**: Required.
* **Response Body (200 OK)**:
  ```json
  {
    "userId": "9b1deb4d-3b7d-4bad-9bdd-2b0d7b3dcb6d",
    "tier": "free",
    "status": "inactive",
    "currentPeriodEnd": "0001-01-01T00:00:00Z",
    "isVip": false,
    "isAdmin": false,
    "limits": {
      "collection": {
        "used": 12,
        "limit": 50,
        "isExceeded": false
      },
      "wishlist": {
        "used": 5,
        "limit": 25,
        "isExceeded": false
      },
      "shares": {
        "used": 0,
        "limit": 1,
        "isExceeded": false
      }
    }
  }
  ```

#### D. VIP Admin Management (`POST /api/admin/billing/vip`)
Allows administrators to manually override a user's VIP status.
* **Authentication**: Required (and verifying user's `is_admin` field).
* **Request Body**:
  ```json
  {
    "userId": "d7448df7-e921-4f32-840f-7b78a9c8b73a",
    "isVip": true
  }
  ```
* **Success Response (200 OK)**:
  ```json
  {
    "status": "success",
    "userId": "d7448df7-e921-4f32-840f-7b78a9c8b73a",
    "isVip": true
  }
  ```
* **Error Responses**:
  * `401 Unauthorized`: Unauthenticated.
  * `403 Forbidden`: Authenticated, but not an admin.

#### E. Paddle Webhook Ingest Route (`POST /api/billing/webhook`)
Handles real-time asynchronous callbacks from Paddle.
* **Authentication**: None (Paddle signature header verified).
* **Headers**: `Paddle-Signature: pds_live_...`
* **Response**: `200 OK` (with small confirmation payload) on successful ingestion.

---

### 3. Backend Service Architecture & Paywall Middleware

To support proper testing and enforce limits securely, we define boundaries between Paddle API clients, database query structures, and handler layers.

#### Limits Definitions (`internal/billing/limits.go`)
Limits are represented as constants and tied directly to check functions.
```go
package billing

const (
	FreeCollectionLimit = 50
	FreeWishlistLimit   = 25
	FreeShareLimit      = 1
)
```

#### Testable Paddle API Interface (`internal/billing/paddle_client.go`)
By isolating external API dependencies within a mockable Go interface, we can test 100% of the checkout, portal, and webhook code routes, keeping test coverage $\ge 90\%$.
```go
package billing

import (
	"context"
	"time"
)

type PaddleClient interface {
	CreateTransaction(ctx context.Context, priceID, successURL, cancelURL, userID string) (string, error)
	GetCustomerPortalURL(ctx context.Context, customerID string) (string, error)
	GetSubscriptionPeriodEnd(ctx context.Context, subscriptionID string) (time.Time, error)
}
```

#### Active Status Evaluation
Instead of trusting self-reported token claims or JWT payloads, the backend evaluates active status directly in database handlers during mutations. We define a fast lookup model:

```go
package billing

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type dbPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type UserStatus struct {
	UserID           string    `json:"userId"`
	Tier             string    `json:"tier"`
	Status           string    `json:"status"`
	CurrentPeriodEnd time.Time `json:"currentPeriodEnd"`
	IsVIP            bool      `json:"isVip"`
	IsAdmin          bool      `json:"isAdmin"`
}

// IsPremium validates if the user bypasses limits through active subscriptions or VIP overrides.
func (u *UserStatus) IsPremium() bool {
	if u.IsVIP {
		return true
	}
	return u.Tier == "premium" && (u.Status == "active" || u.Status == "trialing")
}

// FetchStatus queries the exact, real-time database state.
func FetchStatus(ctx context.Context, pool dbPool, userID string) (*UserStatus, error) {
	var tier, status string
	var currentPeriodEnd *time.Time
	var isVip, isAdmin bool

	err := pool.QueryRow(ctx, `
		SELECT 
			COALESCE(s.tier, 'free'),
			COALESCE(s.status, 'inactive'),
			s.current_period_end,
			p.is_vip,
			p.is_admin
		FROM public.profiles p
		LEFT JOIN public.subscriptions s ON s.user_id = p.id
		WHERE p.id = $1`, userID).Scan(&tier, &status, &currentPeriodEnd, &isVip, &isAdmin)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &UserStatus{UserID: userID, Tier: "free", Status: "inactive"}, nil
		}
		return nil, err
	}

	us := &UserStatus{
		UserID:  userID,
		Tier:    tier,
		Status:  status,
		IsVIP:   isVip,
		IsAdmin: isAdmin,
	}
	if currentPeriodEnd != nil {
		us.CurrentPeriodEnd = *currentPeriodEnd
	}
	return us, nil
}
```

#### Limit Validation Middleware/Utility (`internal/billing/guard.go`)
This routine validates limits before adding collection records, wishlists, or shares:
```go
package billing

import (
	"context"
	"errors"
)

var (
	ErrCollectionLimitExceeded = errors.New("free tier collection limit (50 items) reached")
	ErrWishlistLimitExceeded   = errors.New("free tier wishlist limit (25 items) reached")
	ErrShareLimitExceeded      = errors.New("free tier wishlist sharing limit (1 share) reached")
)

func GuardLimit(ctx context.Context, pool dbPool, userID string, action string) error {
	status, err := FetchStatus(ctx, pool, userID)
	if err != nil {
		return err
	}

	// Bypass limits entirely if VIP or active Premium
	if status.IsPremium() {
		return nil
	}

	switch action {
	case "collection":
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM public.collection_items WHERE user_id = $1", userID).Scan(&count)
		if err != nil {
			return err
		}
		if count >= FreeCollectionLimit {
			return ErrCollectionLimitExceeded
		}
	case "wishlist":
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM public.wishlist_items WHERE user_id = $1", userID).Scan(&count)
		if err != nil {
			return err
		}
		if count >= FreeWishlistLimit {
			return ErrWishlistLimitExceeded
		}
	case "share":
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM public.wishlist_shares WHERE owner_id = $1", userID).Scan(&count)
		if err != nil {
			return err
		}
		if count >= FreeShareLimit {
			return ErrShareLimitExceeded
		}
	}

	return nil
}
```

---

### 4. Frontend Views & Svelte Mockups

The Svelte-based frontend will implement subscription dashboards, billing management buttons, non-intrusive upgrade notices, and paywall intercept modals.

#### A. Billing Portal & Settings Component (`frontend/src/components/BillingSettings.svelte`)
Loads Paddle.js, fetches `/api/billing/config`, and opens the Paddle.js overlay checkout.
```svelte
<script lang="ts">
	import { apiFetch } from '../lib/api';
	import { fetchBillingConfig, loadPaddleScript, initPaddle, openPaddleCheckout } from '../lib/paddle';
	
	let status = $state<any>(null);
	let loading = $state(true);
	let processing = $state(false);

	async function loadBillingStatus() {
		try {
			const res = await apiFetch('/api/billing/status');
			status = await res.json();
		} catch (err) {
			console.error('Failed to load billing details', err);
		} finally {
			loading = false;
		}
	}

	async function handleCheckout() {
		processing = true;
		try {
			const config = await fetchBillingConfig();
			await loadPaddleScript();
			const paddle = await initPaddle(config.clientToken, config.environment);
			await openPaddleCheckout({
				priceId: config.premiumMonthlyPriceId,
				userId: status.userId,
				successUrl: window.location.origin + '/account?checkout=success#billing',
				paddle,
				onComplete: () => loadBillingStatus()
			});
		} catch (err) {
			alert('Failed to initiate checkout. Please try again.');
		} finally {
			processing = false;
		}
	}

	async function handlePortal() {
		processing = true;
		try {
			const res = await apiFetch('/api/billing/portal', { method: 'POST' });
			const data = await res.json();
			if (data.portalUrl) {
				window.location.href = data.portalUrl;
			}
		} catch (err) {
			alert('Failed to open billing portal.');
		} finally {
			processing = false;
		}
	}

	$effect(() => {
		loadBillingStatus();
	});
</script>

<div class="border border-gold-muted/30 bg-white p-6 rounded-lg max-w-lg">
	<h3 class="font-serif text-2xl text-espresso mb-4">Membership Plan</h3>

	{#if loading}
		<div class="text-gold-dark text-xs animate-pulse">Loading membership details...</div>
	{:else if status}
		<div class="space-y-4">
			<div class="flex justify-between items-center pb-3 border-b border-gold-muted/10">
				<div>
					<span class="text-xs text-gold-dark uppercase tracking-wider block">Current Tier</span>
					<span class="text-lg font-bold text-espresso uppercase">
						{status.tier}
						{#if status.isVip} <span class="text-gold font-serif text-sm font-normal lowercase">(vip exemption)</span>{/if}
					</span>
				</div>
				<span class="px-2.5 py-1 text-xs rounded-full uppercase tracking-wider {status.status === 'active' || status.isVip ? 'bg-emerald-100 text-emerald-800' : 'bg-gold-muted/20 text-gold-dark'}">
					{status.isVip ? 'Lifetime Exemption' : status.status}
				</span>
			</div>

			<!-- Limits Visual Meter -->
			<div class="space-y-3 py-2">
				<h4 class="text-xs uppercase text-gold-dark tracking-wider font-semibold">Usage Limits</h4>
				
				<!-- Collection Limit -->
				<div>
					<div class="flex justify-between text-xs mb-1">
						<span class="text-espresso">Collection Space</span>
						<span class="font-semibold">{status.limits.collection.used} / {status.limits.collection.limit} releases</span>
					</div>
					<div class="w-full bg-gold-muted/20 h-2.5 rounded-full overflow-hidden">
						<div class="bg-gold h-full transition-all duration-300" style="width: {Math.min((status.limits.collection.used / status.limits.collection.limit) * 100, 100)}%"></div>
					</div>
				</div>

				<!-- Wishlist Share Limit -->
				<div>
					<div class="flex justify-between text-xs mb-1">
						<span class="text-espresso">Wishlist Shares</span>
						<span class="font-semibold">{status.limits.shares.used} / {status.limits.shares.limit} active links</span>
					</div>
					<div class="w-full bg-gold-muted/20 h-2.5 rounded-full overflow-hidden">
						<div class="bg-espresso h-full transition-all duration-300" style="width: {Math.min((status.limits.shares.used / status.limits.shares.limit) * 100, 100)}%"></div>
					</div>
				</div>
			</div>

			<div class="pt-4 flex gap-4">
				{#if status.tier === 'free' && !status.isVip}
					<button 
						disabled={processing}
						onclick={handleCheckout} 
						class="w-full bg-espresso hover:bg-espresso-dark text-gold font-semibold py-3 px-4 rounded transition-colors text-xs uppercase tracking-wider disabled:opacity-50">
						{processing ? 'Loading...' : 'Upgrade to Premium — $5/mo'}
					</button>
				{:else if !status.isVip}
					<button 
						disabled={processing}
						onclick={handlePortal} 
						class="w-full border border-espresso text-espresso hover:bg-gold-muted/10 font-semibold py-3 px-4 rounded transition-colors text-xs uppercase tracking-wider disabled:opacity-50">
						{processing ? 'Loading...' : 'Manage Subscription'}
					</button>
				{/if}
			</div>
		</div>
	{/if}
</div>
```

#### B. Dashboard Top Alert Banner (`frontend/src/components/UpgradeBanner.svelte`)
Displays a warning message if a user's subscription downgrades or is close to limits.
```svelte
<script lang="ts">
	interface Props {
		limits: {
			collection: { used: number; limit: number; isExceeded: boolean };
			wishlist: { used: number; limit: number; isExceeded: boolean };
			shares: { used: number; limit: number; isExceeded: boolean };
		};
		tier: string;
	}
	let { limits, tier }: Props = $props();

	let exceeded = $derived(
		limits.collection.isExceeded || limits.wishlist.isExceeded || limits.shares.isExceeded
	);
</script>

{#if exceeded}
	<div class="bg-amber-50 border-l-4 border-amber-600 p-4 mb-6 rounded-r-lg">
		<div class="flex items-start gap-3">
			<div class="flex-shrink-0 text-amber-600">
				<svg class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
					<path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd" />
				</svg>
			</div>
			<div class="flex-1">
				<h3 class="text-sm font-semibold text-amber-800">Account limits exceeded</h3>
				<p class="text-xs text-amber-700 mt-1">
					Your account is currently over its limit due to a tier change or plan expiration. Your records are preserved safely, but you will not be able to add new items or share wishlists until you upgrade your subscription.
				</p>
				<a href="/account#billing" class="inline-block mt-2.5 text-xs font-bold text-amber-900 hover:underline">
					Upgrade Now &rarr;
				</a>
			</div>
		</div>
	</div>
{/if}
```

#### C. Intercept Overlay Notification (`frontend/src/components/PaywallModal.svelte`)
Triggered dynamically when a mutation returns a limit-exceeded error code. It fetches `/api/billing/config` and opens the Paddle.js overlay checkout.
```svelte
<script lang="ts">
	import { fetchBillingConfig, loadPaddleScript, initPaddle, openPaddleCheckout } from '../lib/paddle';

	interface Props {
		isOpen: boolean;
		actionType: 'collection' | 'wishlist' | 'share';
		onClose: () => void;
		onComplete?: () => void;
	}
	let { isOpen, actionType, onClose, onComplete }: Props = $props();

	let details = $derived({
		collection: {
			title: 'Record Storage Limit Reached',
			desc: 'Free accounts can store up to 50 albums. Expand to unlimited space to catalog your entire crate.',
		},
		wishlist: {
			title: 'Wishlist Limit Reached',
			desc: 'Keep tracking your holy grails. Upgrade to Premium to track more than 25 records on your wishlist.',
		},
		share: {
			title: 'Wishlist Share Limit Reached',
			desc: 'Direct wishlist sharing to fellow collectors is capped at 1 share for Free accounts. Upgrade for unlimited sharing.',
		}
	}[actionType]);

	async function handleUpgrade() {
		try {
			const config = await fetchBillingConfig();
			await loadPaddleScript();
			const paddle = await initPaddle(config.clientToken, config.environment);
			await openPaddleCheckout({
				priceId: config.premiumMonthlyPriceId,
				userId: currentUserId,
				successUrl: window.location.origin + '/account?checkout=success#billing',
				paddle,
				onComplete: () => { if (onComplete) onComplete(); }
			});
		} catch (err) {
			alert('Failed to initiate checkout. Please try again.');
		}
	}
</script>

{#if isOpen}
	<div class="fixed inset-0 bg-espresso/80 backdrop-blur-sm flex items-center justify-center z-50 p-4">
		<div class="bg-white border-2 border-gold rounded-xl max-w-md w-full p-6 text-center shadow-2xl">
			<div class="w-16 h-16 bg-gold/10 text-gold rounded-full flex items-center justify-center mx-auto mb-4">
				<svg class="w-8 h-8" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
				</svg>
			</div>
			<h2 class="font-serif text-2xl text-espresso mb-2">{details.title}</h2>
			<p class="text-sm text-gold-dark mb-6 leading-relaxed">{details.desc}</p>
			
			<div class="space-y-3">
				<button
					onclick={handleUpgrade}
					class="block w-full bg-espresso hover:bg-espresso-dark text-gold font-semibold py-3 px-4 rounded text-xs uppercase tracking-wider">
					Unlock Unlimited with Premium
				</button>
				<button onclick={onClose} class="text-xs text-gold-dark hover:text-espresso font-semibold tracking-wider uppercase pt-2">
					Maybe Later
				</button>
			</div>
		</div>
	</div>
{/if}
```

---

### 5. Robust Edge Cases & System Resiliency

To prevent revenue loss, transaction synchronization failures, or customer frustration, the system addresses the following complex scenarios:

#### A. Webhook Failure, Out-of-Order Delivery, & Verification
* **Signature Guarding**: Every webhook callback verifies Paddle's `Paddle-Signature` header. Replays are immediately dropped.
* **Webhook Idempotency**: Actionable events are recorded in `public.paddle_webhook_events` (introduced in `supabase/migrations/00006_webhook_idempotency.sql`) keyed by `event_id`. If an event ID has already been processed, the handler returns 200 without reprocessing.
* **Database Upserts (Idempotence)**: To prevent duplicate processing (e.g. from retries), webhook updates use SQL upsert statements (`INSERT ... ON CONFLICT (id) DO UPDATE`).
* **Sandbox Unsigned Fallback**: Unsigned webhooks are accepted only when `PADDLE_ENVIRONMENT=sandbox` and `PADDLE_WEBHOOK_SECRET` is empty. Production requires signature verification.
* **Concurrency Handling**: If a `transaction.completed` arrives before a `subscription.created`, database queries check for pre-existing checkout state mappings rather than relying on exact delivery sequences.
* **Paddle Webhook Delivery Delays**: If webhook delivery lags, the client-side checkout callback page `/account?checkout=success#billing` displays a friendly polling screen while waiting up to 5 seconds for the database record to update:
  ```ts
  // Polling loop for active membership status change
  async function pollSubscriptionStatus(retries = 5, delay = 1000) {
      for (let i = 0; i < retries; i++) {
          const res = await apiFetch('/api/billing/status');
          const status = await res.json();
          if (status.tier === 'premium') return true;
          await new Promise(resolve => setTimeout(resolve, delay));
      }
      return false;
  }
  ```

#### B. Subscription Cancellation and Graceful Downgrades
* **Soft Downgrades**: As defined in **AD-4**, if a user cancels their subscription, existing items over the limit are never truncated or deleted. This preserves customer goodwill.
* **Canceled Status Tracking**: The subscription table transitions status to `'canceled'`. The Go backend sees that their active status check (`IsPremium()`) evaluates to false, subsequently blocking write additions without modifying old data.

#### C. Past Due, Unpaid, and Paused Status Handling
* **Grace Period**: When an automatic recurring charge fails, Paddle places the subscription in `past_due` and later `paused`.
* **User Intercept**: During `past_due`, we allow a short grace period where the user remains Premium but sees a subtle notification banner: `"Payment overdue. Please update billing method to retain unlimited access"`. If payment fails after all retries, Paddle marks it `unpaid` or `canceled`, and the status degrades to inactive.

---

## Consequences

* **Security**: Billing metadata is securely isolated in `subscriptions`, preventing leakages via globally readable profile queries.
* **Integrity**: Standard users cannot self-elevate to VIP or Admin via client-side endpoints, thanks to the PL/pgSQL database trigger guard.
* **Testing**: High test coverage remains preserved because Paddle operations are routed behind mockable interfaces.
* **Customer Experience**: Soft downgrades ensure that users don't lose records when subscriptions lapse, making resubscription smooth.
