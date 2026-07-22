# Production Launch Checklist — Premium Billing + Share Notifications

Everything below is verified against production as of 2026-07-20. Code is merged on `main`; items marked [ ] block the launch.

## Already done

- [x] Code merged to `main` (Paddle billing, notifications, claim links)
- [x] DB migrations applied to production: `00001`–`00007` (incl. `paddle_webhook_events`, `notifications`) and recorded in `supabase_migrations.schema_migrations`
- [x] VIP exemptions set: `john.ferguson@v1truv1us.dev` (VIP + admin), `porgito2011@gmail.com` (VIP), `snobord4life@gmail.com` (VIP), `mcp-test@audiofile.app` (VIP, pre-existing)
- [x] Verified: prod Paddle API key valid (`GET /api/billing/test` → reachable)
- [x] Verified: webhook signature enforcement active (unsigned POST → 400)
- [x] Verified: `/api/billing/status` returns correct tier/limits per user
- [x] Verified: sandbox price `pri_01kvd8cef535j23f40kz63qdyf` active ($5/mo)

## Blocking — Paddle dashboard (owner action required)

- [ ] **Approve checkout domain**: Paddle Dashboard → Checkout → Domain approval → add `audiofile.app`. Without this, transaction creation fails with `transaction_checkout_url_domain_is_not_approved` (verified 2026-07-20).
- [ ] **Webhook destination**: Developer Tools → Notifications → URL `https://audiofile.app/api/billing/webhook`, events: `transaction.completed`, `subscription.created`, `subscription.updated`, `subscription.canceled`, `subscription.paused`. Confirm the secret matches `PADDLE_WEBHOOK_SECRET` in Coolify.
- [ ] Decide sandbox vs production: prod currently runs `PADDLE_ENVIRONMENT=sandbox`. Going live means switching all Paddle vars to `pdl_live_*` / `live_*` values and repeating domain approval in the production Paddle account.

## Blocking — Coolify env vars

Set via Coolify API on 2026-07-21 (all six Paddle vars + `APP_BASE_URL`):

- [x] `PADDLE_API_KEY`, `PADDLE_WEBHOOK_SECRET`, `PADDLE_ENVIRONMENT` (sandbox), `PADDLE_CLIENT_TOKEN`, `PADDLE_PREMIUM_MONTHLY_PRICE_ID`, `APP_BASE_URL` — verified live: `/api/billing/config` returns the real client token + price ID
- [ ] `SUPABASE_SERVICE_ROLE_KEY` — recipient email lookup for share emails (server-only)
- [ ] `RESEND_API_KEY`, `RESEND_FROM_EMAIL` — share notification emails (optional but recommended)

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
