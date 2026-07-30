# Paddle Live Dashboard Settings — click-through guide

Everything here is a manual step in the **live** Paddle dashboard (`vendors.paddle.com`). Reference: AudioFile, Premium **$5/mo**, Paddle as Merchant of Record, price `pri_01kykdwht33gz89pj1t9898yny` (`tax_mode: external`).

Work top to bottom; each maps to a go-live checklist item.

## 1. Default payment link — Checkout → Checkout settings → Default payment link
- Set to **`https://audiofile.app/account`** (the billing page — it includes Paddle.js via `BillingSettings`). This is the fallback checkout page for transactions that don't carry an explicit checkout URL.

## 2. Payment methods — Checkout → Checkout settings → Payment methods
- **Card**: on (always on).
- **Apple Pay**: **on**.
- **Google Pay**: **on** (mobile-first audience).
- **PayPal**: optional — enable if you want it; adds reach but a little friction.

## 3. Sales tax settings — Checkout → Sales tax settings
- Choose **"Prices exclusive of tax"** — this matches your price's `tax_mode: external`, so Paddle calculates and adds tax at checkout by customer location.
- Alternative: if you want the advertised **$5 to be the all-in price**, switch to "inclusive of tax" **and** update the live price's `tax_mode` to `internal` (via API or dashboard).

## 4. Balance currency — Business account → Currencies
- **USD** (matches the $5/mo price and, presumably, your bank account).

## 5. Retain / dunning — Paddle → Retain
- **Enable dunning**: on.
- **Failed-payment retries**: leave the default retry schedule (Retain auto-retries).
- **Payment reminders**: on (emails customers around a failed charge).
- **Self-service payment update**: on (customers update their card in the customer portal — `pwCustomer` is already wired in the frontend).

## 6. Payout details — Business account → Payouts → Payout settings
- **Method**: bank transfer — add your bank details.
- **Minimum threshold**: $100 (lower it if you want more frequent payouts; note payouts run monthly and only once verification is complete).

## 7. Taxable categories — Catalog → Taxable categories
- Keep **Standard Digital Goods** (default; your product already uses `tax_category: standard`).
- Optional: request **SaaS** if you want software-subscription-specific tax handling — not required to launch.

## 8. Revoke the migration API key — Developer tools → Authentication
- Revoke `pdl_live_apikey_01kykdjhbrgz…` (the migration key — it's in this chat history and has served its purpose). Keep only the **runtime** key in production.

## After these + individual verification
Once these are set and your individual verification is approved:
1. Coolify env → live values (runtime `PADDLE_API_KEY`, `PADDLE_CLIENT_TOKEN=live_93311…`, `PADDLE_PREMIUM_MONTHLY_PRICE_ID=pri_01kykdwht33…`, `PADDLE_WEBHOOK_SECRET=<live ntfset secret>`, `PADDLE_ENVIRONMENT=production`).
2. Set `BILLING_ENABLED=true` (or remove it) in Coolify and redeploy.
3. Take a real payment to confirm end-to-end.
