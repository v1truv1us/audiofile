# Production Launch Checklist — Premium Billing + Share Notifications

Everything below is verified against production as of 2026-07-20. Code is merged on `main`; items marked [ ] block the launch.

## Already done

- [x] Code merged to `main` (Paddle billing, notifications, claim links)
- [x] DB migrations applied to production: `00001`–`00008` (incl. `paddle_webhook_events`, `notifications`) and recorded in `supabase_migrations.schema_migrations`
- [x] VIP exemptions set: `john.ferguson@v1truv1us.dev` (VIP + admin), `porgito2011@gmail.com` (VIP), `snobord4life@gmail.com` (VIP), `mcp-test@audiofile.app` (VIP, pre-existing)
- [x] Verified: prod Paddle API key valid (`GET /api/billing/test` → reachable)
- [x] Verified: webhook signature enforcement active (unsigned POST → 400)
- [x] Verified: `/api/billing/status` returns correct tier/limits per user
- [x] Verified: sandbox price `pri_01kvd8cef535j23f40kz63qdyf` active ($5/mo)

## Paddle — sandbox → live migration (2026-07-24)

Live catalog + webhook destination created in the LIVE account via API. Sandbox → live ID map:
- Product: `pro_01kvd8bpqs11zv3mvm88z4c2ew` → **`pro_01kykdwhpf30n80wqn50b3xwe3`**
- Price:   `pri_01kvd8cef535j23f40kz63qdyf` → **`pri_01kykdwht33gz89pj1t9898yny`** ($5/mo)
- Webhook destination: **`ntfset_01kykdwj0j7eqfyzmrsg45y4cd`** → `https://audiofile.app/api/billing/webhook` (signing secret captured = `PADDLE_WEBHOOK_SECRET`)

Code is env-driven; `backend/.env` switched to live for local/staging verification. **Coolify (production) remains on sandbox until the items below clear + Part 3.**

### Blocking — Paddle LIVE dashboard (owner action required)

- [ ] **Approve checkout domain (LIVE)**: Paddle live dashboard → Checkout → Website approval → add `audiofile.app`. Live does NOT auto-approve; without this, live checkout fails to load.
- [ ] **Default payment link (LIVE)**: Checkout → Checkout settings → Default payment link → your live checkout page (real approved domain, not localhost).
- [ ] **Revoke the migration API key** (`pdl_live_apikey_01kykdjhbrgz…`) — served its purpose; keep only the runtime key in production.
- [ ] Local/staging verification: backend with live env → `GET /api/billing/config` returns the `live_…` token + `pri_01kykdwht33…`; run a checkout once the domain is approved.
- [ ] **Part 3**: switch to `vendors.paddle.com` for verification + a real payment, then update Coolify env + deploy.

## Blocking — Coolify env vars

Set via Coolify API on 2026-07-21 (all six Paddle vars + `APP_BASE_URL`):

- [x] `PADDLE_API_KEY`, `PADDLE_WEBHOOK_SECRET`, `PADDLE_ENVIRONMENT` (sandbox), `PADDLE_CLIENT_TOKEN`, `PADDLE_PREMIUM_MONTHLY_PRICE_ID`, `APP_BASE_URL` — verified live: `/api/billing/config` returns the real client token + price ID
- [ ] `SUPABASE_SERVICE_ROLE_KEY` — recipient email lookup for share emails (server-only)
- [ ] `RESEND_API_KEY`, `RESEND_FROM_EMAIL` — share notification emails (optional but recommended)

## Database

- [x] Applied security migration `supabase/migrations/00008_paddle_webhook_events_rls.sql` to production (2026-07-22 via `supabase db push`). Enables RLS on `paddle_webhook_events` to clear the Security advisor warning. Recorded in `supabase_migrations.schema_migrations`; `00001`–`00008` now synced.

## Blocking — Deploy

- [x] GitHub Actions `Test` + `Deploy to Coolify` green on `main` (2026-07-21, after frontend build fix); app restarted via Coolify API to pick up the new env vars

## Post-deploy smoke (in order)

1. `curl https://audiofile.app/api/health` → 200
2. `curl -X POST https://audiofile.app/api/billing/webhook -d '{}'` → 400 `invalid signature` (verification still enforced)
3. Signed in: `GET /api/billing/config` → non-empty `premiumMonthlyPriceId` + `clientToken`
4. Sandbox checkout end-to-end with test card `4242 4242 4242 4242` on a non-VIP test account → webhook fires → `SELECT tier, status FROM subscriptions WHERE user_id = ...` shows `premium/active`
5. Share a wishlist to another account → bell badge appears ≤30s, row in `/notifications`, email in Resend dashboard
6. Open `/wishlist?share=<owner-id>` signed in → "Add to my shared wishlists" → appears in `/shared`; re-click → already added
7. VIP check: VIP accounts can exceed free limits without paywall

## Rollback

Redeploy previous image from Coolify. Migrations are additive and safe to leave in place.
