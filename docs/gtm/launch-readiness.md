# Launch Readiness — start here when you're back

Snapshot: **2026-07-24.** All creative + community assets are staged. A few owner-only actions gate the paid launch. Everything else is ready to execute.

## TL;DR
- ✅ Launch copy, community plan, manager agent, teaser videos, **policy/pricing pages**, **webhook IP allowlist**, **Paddle live migration** (catalog + client token + webhook destination), and a **billing kill-switch** — all done.
- 🚀 **Launch publicly now:** set `BILLING_ENABLED=false` in Coolify + deploy the frontend → full-access beta, no paywall. Billing flips on later, one env var.
- ❌ **Take real payments later:** gated on a few owner actions + Paddle verification (Step 1b).
- 🗓️ Anchor **Day 1 = your first Monday back** (shift the calendar window if you return later — structure stays: 3 weeks community → 1 week launch).
- 🤖 Switch to the **`community-manager`** agent (restart opencode first) to keep drafting/logging.

---

## Step 1a — Launch publicly NOW — ✅ SHIPPED (2026-07-24)
- [x] **Coolify env: `BILLING_ENABLED=false`** set (via Coolify API).
- [x] **Deployed the frontend** — `/terms`, `/privacy`, `/refund`, `/pricing` + footer links live and verified.
- [x] **Domain provisionally approved** by Paddle (payouts pending Paddle re-crawl of the now-live policy pages).
- [x] **Live webhook handler verified end-to-end** (signed → 200, unsigned → 400; Cloudflare passes Paddle-style requests).
- [x] **paddle-live MCP** configured (opencode) + live catalog confirmed (Premium Monthly product + $5/mo price, active; webhook destination active).
- [ ] **Make `support@audiofile.app` real**: Cloudflare → Email → Email Routing → add `support@audiofile.app` → forward to your inbox, then click the verification email. (It's now in the footer + policies; Paddle + customers will email it.)

## Step 1b — Take real payments later (after Paddle verification)
Live migration done: product `pro_01kykdwhpf30n80wqn50b3xwe3`, price `pri_01kykdwht33gz89pj1t9898yny`, webhook dest `ntfset_01kykdwj0j7eqfyzmrsg45y4cd` (signing secret captured). `backend/.env` is on live; **Coolify stays on sandbox** until these clear:
- [ ] **Paddle LIVE: approve checkout domain** `audiofile.app` (Checkout → Website approval). Live doesn't auto-approve; checkout won't load without it.
- [ ] **Paddle LIVE: default payment link** → live checkout page (Checkout → Checkout settings; real domain, not localhost).
- [ ] **Revoke the migration API key** (`pdl_live_apikey_01kykdjhbrgz…`); keep only the runtime key in prod.
- [ ] **Paddle verification:** switch to `vendors.paddle.com` → submit **individual** verification (photo ID + proof of address; no company docs required) + take a real payment.
- [ ] **Coolify env → live:** `PADDLE_API_KEY` (runtime), `PADDLE_CLIENT_TOKEN=live_93311bcc434ed474dcb83358415`, `PADDLE_PREMIUM_MONTHLY_PRICE_ID=pri_01kykdwht33gz89pj1t9898yny`, `PADDLE_WEBHOOK_SECRET=<captured pdl_ntfset_…>`, `PADDLE_ENVIRONMENT=production`, plus `SUPABASE_SERVICE_ROLE_KEY`, `RESEND_API_KEY`, `RESEND_FROM_EMAIL`.
- [ ] **Flip the kill-switch:** set `BILLING_ENABLED=true` (or remove it) + redeploy → paywalls + checkout live.

> Already done this session: RLS migration `00008`; Resend domain `mail.audiofile.app` verified + sending; webhook IP allowlist (`webhook_ip.go`, fetches `api.paddle.com/ips`); `pwCustomer` wired for Retain; policy/pricing pages + footer links; billing kill-switch (`BILLING_ENABLED`). Supabase SMTP left on the built-in mailer per your call.

---

## Step 2 — community track (run during/after blockers)
Switch to **`community-manager`**; it researches threads, drafts, and logs to `drafts/POSTING-LOG.md`. You post manually.
- **Weeks 1–2 (lurk):** post 3–5 genuine comments/day from `drafts/week-1-comments.md`. No product mentions.
- **Week 3 (soft):** `drafts/week-3-content.md` — r/vinyl discussion post + first IG/TikTok demo.

---

## Step 3 — launch week (anytime after Step 1a; billing-off beta launch is fine)
Day-by-day in `community-launch-plan.md` → Week 4. Copy is pre-drafted there + in `drafts/launch-emails-and-bip.md`. Drop in the teaser videos: **9:16** for IG/TikTok, **16:9** for PH/HN/landing.

Optimal windows: **Product Hunt** Tue–Thu 00:01 PT (maker comment ≤5 min); **Show HN** Tue–Thu 8–11am ET (intro comment immediately, reply for 2h); **r/SideProject** first, then tailor per community. Never coordinate upvotes.

---

## Asset index

| File | What it is |
|---|---|
| `docs/gtm/launch-plan.md` | Strategy & metrics (source of truth) |
| `docs/gtm/community-launch-plan.md` | 30-day calendar + drafted launch posts |
| `docs/gtm/drafts/swipe-file.md` | Real collector pain phrases (copy bank) |
| `docs/gtm/drafts/week-1-comments.md` | Genuine, no-product Week 1 comments |
| `docs/gtm/drafts/week-3-content.md` | r/vinyl post + first IG/TikTok demo |
| `docs/gtm/drafts/demo-video-storyboard.md` | Full app-demo production package |
| `docs/gtm/drafts/launch-emails-and-bip.md` | 3-email sequence + build-in-public thread |
| `docs/gtm/drafts/POSTING-LOG.md` | 9:1 ratio tracker + channel rules |
| `docs/gtm/drafts/raw/audiofile-teaser-9x16.mp4` | Teaser — vertical (social) |
| `docs/gtm/drafts/raw/audiofile-teaser-16x9.mp4` | Teaser — landscape (web/PH/HN) |
| `.opencode/agent/community-manager.md` | The manager agent |

## Preview the videos
```
open docs/gtm/drafts/raw/audiofile-teaser-9x16.mp4
open docs/gtm/drafts/raw/audiofile-teaser-16x9.mp4
```

## Still optional / nice-to-have
- **Music bed** for the teasers (silent now) — drop a royalty-free mp3 in `drafts/raw/` and I'll mux it.
- **The real app-demo video** — record the 3 screen flows + store b-roll per the storyboard; I'll cut the full version with captions.
- **Captions/SRT** for the app-demo once VO is recorded.
