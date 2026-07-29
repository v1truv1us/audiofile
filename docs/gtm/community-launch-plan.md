# AudioFile Community Launch Plan & Calendar

A 30-day, low-spam, story-first plan to build awareness for **AudioFile** (vinyl/record collection + wishlist tracking with social sharing — freemium at audiofile.app).

**Strategic hook (decided 2026-07-22):** Lead with **speed** — the in-store "do I own this?" pain collectors voice loudest (verified in `docs/gtm/drafts/swipe-file.md`). Feature **social sharing** as the differentiator ("why AudioFile, not just a faster Discogs"). Full strategy/metrics/assets live in `docs/gtm/launch-plan.md`; this doc is the executable community calendar under that strategy.

**Dates:** Day 1 = Mon Jul 27 2026 → Day 30 = Fri Aug 21 2026 (adjust as needed; keep launch days Tue–Thu).

---

## Golden rules (read first)

1. **9:1 ratio** — nine genuine contributions for every one that mentions AudioFile. New accounts that only self-promo get auto-suppressed.
2. **Disclose you're the maker** whenever the app comes up. ("I build AudioFile, so grain of salt…")
3. **Story > pitch.** "After X months building a collection tracker, here's what I learned" beats "Try my app."
4. **Never coordinate upvotes** (HN/Reddit detect voting rings → ban risk). Share that you launched; let people find it.
5. **No cross-posting** the same link to 5 subs in one hour. Pick 2–3, tailor each.
6. **Draft-for-approval only.** Nothing posts itself. Copy → paste → post yourself.

---

## Pre-flight checklist (do before Day 1)

- [ ] Reddit account ≥ 7 days old, ~50 karma, bio set ("builder of AudioFile · vinyl nerd")
- [ ] HN account ≥ 7 days old; Product Hunt account created (7+ days old)
- [ ] Instagram + TikTok accounts; bio link → audiofile.app
- [ ] Landing page: 1-line headline, **30s demo video**, pricing visible, free tier no-CC
- [ ] App stable on mobile; signup works end-to-end
- [ ] Resolve production blockers first (production-launch-checklist.md:17-19 Paddle domain/webhook + sandbox→prod)

---

## Week 1–2 (Jul 27 – Aug 7): Lurk & contribute. Zero product mentions.

**Goal:** build account age, karma, and a swipe file of real pain phrases.

- **Daily (15 min):** 3–5 substantive comments across **r/vinyl**, **Steve Hoffman Forums** (Music Corner + Marketplace Discussions), and **one Discord** (Arr Vinyl / Vinyl Chat / Groove Gang).
- **Log phrases** people use to complain about cataloging (→ `drafts/swipe-file.md`). These become your copy later.
- **Comment themes** (give value, no link): help ID a pressing, share how you store/grade, answer a gear question, commiserate on Discogs redesign pain.

Example genuinely-helpful comment (no product):
> "I switched to grading by the Goldmine standard + a quick photo of the sleeve corner under daylight — cuts down the 'is this VG or VG+' debate a lot. The trick is being consistent about hairlines vs scuffs."

---

## Week 3 (Aug 10 – 14): Soft content. App mentioned only if asked.

### Mon Aug 10 — r/vinyl discussion post (NOT a pitch)
**Title:** How do you all keep track of your collection / wishlist without it becoming a second job?
**Body:** Genuine question. Share your own flow (spreadsheet? Discogs? notebook?), what bugs you about it, ask others. Mention AudioFile **only in a reply if someone asks** for alternatives, and disclose you're the maker.

### Wed Aug 12 — Instagram + TikTok demo #1 (script)
**30s script:**
- 0–3s (hook): "I scanned my whole shelf in two minutes." [screen recording]
- 3–15s (problem): "Discogs takes forever to load my collection on mobile, so I built something faster."
- 15–22s (demo): add an item in real time, show condition + photo.
- 22–30s (CTA): "Free at audiofile.app — link in bio."
Caption: `#vinylcommunity #vinylcollection #recordcollector`

### Fri Aug 14 — Steve Hoffman / Discogs forum
Join an active "Discogs collection is slow / redesign" thread as a peer. Commiserate specifically. **Only if** the thread invites alternatives: "I've been working on a smaller, faster tracker called AudioFile — not trying to replace Discogs's database, just the day-to-day collection bit. Happy to share if useful."

---

## Week 4 (Aug 17 – 21): Coordinated launch

> **Launch-slip guard:** Week 4 only proceeds if production blockers are cleared (production-launch-checklist.md:17-19 — Paddle domain approval, webhook, sandbox→prod). If they slip, slide the whole launch week forward; Weeks 1–3 community work still runs on the dates above.

### Mon Aug 17 — final prep
- Drafts loaded into each platform's composer, ready to paste.
- Brief 3–5 friends: "launching tomorrow, would value honest feedback."
- Schedule a thank-you tweet/thread.

### Tue Aug 18 — r/SideProject launch (full draft below)
Post ~9am. Reply to every comment within 2h.

### Wed Aug 19 — Product Hunt launch + IG/TikTok launch video
- **PH goes live 00:01 PT.** Maker comment within 5 min.
- IG/TikTok: launch video (reuse demo, add "we're live" CTA).

### Thu Aug 20 — Hacker News Show HN *(optional — first cut if time-tight)*
Post **8–11am ET**. Intro comment immediately. Reply to everything for 2h.

### Fri Aug 21 — follow-up
Thank-you replies, ship any top bug reported, post a "what I learned launching" thread.

---

## Drafted posts (copy → paste → tweak per community)

### 1) r/SideProject — launch (Tue Aug 18)
**Title:** After 6 months building a vinyl collection tracker, here's what I learned about cataloging pain
**Body:**
> **The problem I was trying to solve**
> My collection outgrew a spreadsheet and Discogs' mobile app takes minutes to load it. I kept losing track of what I owned vs. wanted, and sharing a wishlist with friends was impossible.
>
> **What I built**
> [AudioFile](https://audiofile.app) — fast collection + wishlist tracking with social sharing.
> - Add items in seconds with condition + photos
> - Track condition history over time
> - Share a wishlist; friends get notified, can claim items
> - Free tier; $5/mo premium
>
> **Tech stack (for the curious):** Astro + Svelte frontend, Go backend, Supabase, Paddle billing.
>
> **What surprised me:** how much collectors hate slow collection tools — it's the #1 thing testers mentioned.
>
> **What I'm looking for:** what's the one thing you wish a collection app did that nothing currently does?
>
> [link] — free, no credit card.

### 2) Product Hunt — Wed Aug 19
**Tagline (≤60 chars):** Catalog your vinyl fast. Share the hunt.
**Description:** AudioFile is a fast, mobile-friendly way to track your record collection and wishlist — add items in seconds with condition and photos, watch condition change over time, and share wishlists with friends who get notified when you want something. Built for people who find Discogs too slow on mobile. Free tier; $5/mo premium.
**Maker comment (post within 5 min):**
> Hi! I'm John, the maker. I built AudioFile because my own collection outgrew a spreadsheet and every existing tool was either slow on mobile or had no social layer. It's a solo project — v1, some rough edges, and I'm here all day to answer questions. What collection size would stress-test it for you? Happy to fix things live.

### 3) Hacker News — Show HN (Thu Aug 20, 8–11am ET)
**Title:** Show HN: AudioFile – fast, social collection tracking for vinyl collectors
**Intro comment (post immediately):**
> Hi HN! I built a hobby project for tracking vinyl collections. Existing tools (mainly Discogs) are painfully slow on mobile and have no social layer, so I wanted something fast and shareable.
>
> **How it works:** Astro + Svelte frontend, Go backend, Postgres (Supabase). Items have condition + photos; condition history is tracked over time; wishlists can be shared and friends get notified.
>
> **What was hard:** keeping the collection view fast as it scales (lazy loading + indexing), and getting Paddle billing + webhooks right as a solo dev.
>
> **Trade-offs:** I'm not trying to be a music database — Discogs already owns that. AudioFile is just the day-to-day "what do I own / want / its condition" layer.
>
> **Current state:** live at audiofile.app, free tier, no signup wall. Known rough edge: mobile nav on Safari. Honest feedback very welcome — what would make you actually use this?

### 4) r/vinyl — discussion (Wed Aug 10, soft)
**Title:** How do you catalog your collection? Spreadsheet vs Discogs vs something else?
**Body:**
> Curious what everyone actually uses day-to-day. I've been bouncing between a Google Sheet and Discogs, and both feel like a chore — Sheet doesn't scale, Discogs is slow on my phone when I'm in a store. What's your setup, and what's the one thing you'd change about it?
(Mention AudioFile only if asked, and disclose.)

### 5) Discord — launch message (mod-approved #self-promo channel only)
> Hey all — longtime lurker, first share. I built **AudioFile**, a fast collection + wishlist tracker (Discogs was too slow on my phone). Free, web-based: audiofile.app. Not here to spam — happy to do a quick demo in voice if anyone's curious, and genuinely want feedback from people who actually collect. 🙏

### 6) Instagram / TikTok — launch video (Wed Aug 19)
Same 30s script as Aug 12, swap CTA to: **"We just launched — AudioFile's live. Link in bio."**

---

## Posting log

Track every contribution to protect the 9:1 ratio. Maintained in `docs/gtm/drafts/POSTING-LOG.md`.

---

## Post-launch (Aug 22 onward)

- D+1: categorize all feedback (bugs / UX / features / won't-fix). Publish a short public roadmap.
- D+2: ship top 3 bugs; post "fixed X based on your feedback" where you launched.
- D+7: write a launch retro (numbers + lessons). It's your best content for the next push.
- Ongoing: 1 IG/TikTok demo/week + stay active in 2 communities. Add a 4th channel only after one returns signal.
