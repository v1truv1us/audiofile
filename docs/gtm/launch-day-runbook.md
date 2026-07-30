# Launch Day Runbook — AudioFile

One page to run launch day. Copy posts verbatim; times are PT unless noted. Lead angle: **speed (in-store "do I own this?") → social sharing**. Sources: `community-launch-plan.md` (calendar), `launch-emails-and-bip.md` (emails/thread), `demo-video-storyboard.md` (video).

**Decide before anything:** billing **off** (beta, full access — current state) or **on** (live, after Step 1b cutover). This runbook works for a billing-off beta launch; if live, the paywall/checkout is real.

---

## T-1 (evening before)
- [ ] `BILLING_ENABLED` set correctly in Coolify (false for beta, true if live). Confirm `GET /api/billing/config` returns expected `billingEnabled`.
- [ ] Drafts loaded into each platform's composer, ready to paste (PH, HN, Reddit, X/LinkedIn).
- [ ] Teaser video exported: `drafts/raw/audiofile-teaser-9x16.mp4` (social) + `-16x9.mp4` (web).
- [ ] Brief 3–5 friends: "launching tomorrow, honest feedback welcome" (NOT "please upvote" — that's a ban risk).
- [ ] Email #2 (launch) staged in your sender.
- [ ] Alarms: 00:01 PT (PH), 08:00 ET (Show HN).

## 00:01 PT — Product Hunt (main spike)
1. Publish the PH page (or your hunter does).
2. Post your **maker comment within 5 min**:
> Hi! I'm John, the maker. I built AudioFile because my collection outgrew a spreadsheet and every tool was slow on mobile — I kept buying records I already owned. It's a solo project — fast collection + wishlist tracking with shareable wishlists. v1, some rough edges, here all day. What collection size would stress-test it for you?
3. PH page details — Tagline: **Catalog your vinyl fast. Share the hunt.** Description: fast, mobile-friendly collection + wishlist tracking; share wishlists, get notified; free tier, $5/mo Premium.

## Morning (US) — r/SideProject (most welcoming)
Post ~9am PT. **Title:** After 6 months building a vinyl collection tracker, here's what I learned about cataloging pain. **Body:** the full draft in `community-launch-plan.md` → "1) r/SideProject". Reply to every comment within 2h.

## 8–11am ET — Hacker News Show HN *(optional, first cut if busy)*
Post. **Title:** Show HN: AudioFile – fast, social collection tracking for vinyl collectors. Drop the **intro comment immediately** (how it works, what was hard, trade-offs, stack, honest limits — full draft in `community-launch-plan.md` → "3) Hacker News"). Reply to everything for 2h. **Never ask for or coordinate upvotes.**

## Midday — social
- **X/LinkedIn:** post the build-in-public thread (`launch-emails-and-bip.md` → "Build-in-public thread"). Pin it.
- **IG/TikTok:** post the teaser (`audiofile-teaser-9x16.mp4`). Caption: *"Built this because I kept buying records I already owned 😅 AudioFile = your whole collection, fast, on your phone — plus shareable wishlists. Free, link in bio. #vinylcommunity #recordcollector"*
- **Email #2 (launch)** to existing users (in `launch-emails-and-bip.md`).

## All day
- Reply to **every** comment/mention within ~2h (PH, HN, Reddit, X). Thank people.
- Track hourly: PH rank, upvotes, signups, shares created, premium conversions (if live). Log in a scratch note.
- If a real bug surfaces: fix fast, then post "Update: fixed X based on your feedback."

## End of day
- Thank-you reply/tweet with raw numbers + one key insight.
- Mark the day in `POSTING-LOG.md`.

## D+1 / D+2 / D+7
- **D+1:** categorize feedback (bugs / UX / features / won't-fix); post a short public roadmap.
- **D+2:** ship top 3 bugs; post "fixed X, Y, Z."
- **D+7:** write a launch retro (numbers + lessons) — it's your best content for the next push.

## Rollback
- Checkout broken (if live): set `BILLING_ENABLED=false` in Coolify → redeploy → app back to full-access beta instantly.
- Bad traffic spike: the app is static frontend + Go API on Coolify; scale up the container if needed.

## Hard rules (don't get banned)
- 9:1 genuine contributions per product mention (before/after launch).
- Disclose you're the maker. Never coordinate upvotes. Don't cross-post the same link to 5 subs in an hour. Story > pitch.
