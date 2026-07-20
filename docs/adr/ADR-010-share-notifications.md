# ADR-010: Share Notifications and Public-Link Claiming

## Status
Accepted

## Context
ADR-008 shipped identity-based wishlist sharing with an explicit gap: recipients got no signal when a wishlist was shared with them — they had to open "Shared with me" to discover it. Separately, anyone holding an old public share link (`/wishlist?share={userID}`) had no way to pin that wishlist into their own "Shared with me" inbox; the link was view-only and ephemeral.

## Decision
1. **In-app notifications, poll-based.** New `public.notifications` table (`user_id`, `type`, `actor_id`, `data jsonb`, `read_at`, `created_at`). Backend `notifications` package exposes `GET /api/notifications`, `GET /api/notifications/unread-count`, `POST /api/notifications/{id}/read`, `POST /api/notifications/read-all`. The frontend renders a bell in the nav that polls `unread-count` every 30s and an inbox page at `/notifications`. RLS allows users to read/mark-read their own rows; inserts happen only through the backend.
2. **Best-effort email via Resend.** When a share is created, the backend also emails the recipient. The recipient's address is fetched from the Supabase admin API (`/auth/v1/admin/users/{id}`) using a server-only `SUPABASE_SERVICE_ROLE_KEY`. Email failure (or missing `RESEND_API_KEY`/`SUPABASE_SERVICE_ROLE_KEY`) never fails the share request — it is logged and reported to Sentry; the in-app notification is the durable record.
3. **Claim endpoint for public links.** `POST /api/wishlist/shares/claim {ownerId}` lets a logged-in user viewing a public share link add that wishlist to their own inbox by inserting the same `wishlist_shares` row an owner-initiated share would. Duplicates return 200 `already_added` (idempotent); self-claims are rejected. The shared view shows an "Add to my shared wishlists" button to logged-in users only.
4. **Owner is notified in-app on claim** (`wishlist_claimed`), but **never by email** — a public link can be claimed by anyone, so emailing the owner per claim is an abuse vector. The `UNIQUE(owner_id, viewer_id)` constraint caps in-app rows at one per pair.
5. **Claims bypass the billing share guard.** The `"share"` limit counts rows the owner created; charging an owner's quota for a stranger's claim would let anyone exhaust a free-tier owner's quota. No viewer-side limit is added in this iteration.

## Consequences
- Sharing now has a feedback loop: recipient gets a badge/inbox row (and usually an email); owner learns when someone saves their public link.
- The service-role key bypasses RLS; it is used only inside the server-side email lookup, never sent to the client (no `PUBLIC_` prefix).
- Email adds synchronous ~300-600ms to share creation; both HTTP clients use 5s timeouts. If this becomes a problem, the send can move behind the same `EmailSender` interface to a background worker.
- Notification types are extensible via the `type` column + `data jsonb`; future events (price alerts, wishlist matches) can reuse the inbox.

## Alternatives considered
- **Supabase Realtime push.** Rejected — connection management and infra cost for a marginal latency win over a 30s poll.
- **Supabase Edge Function / send-email hook.** Rejected — splits notification logic across two runtimes; the Go backend already owns the share write path.
- **Generic SMTP.** Rejected — deliverability and credential ops burden vs. Resend's single API key.
- **Failing the share when email fails.** Rejected — email providers flake; the in-app row is the source of truth.

## References
- ADR-008 (wishlist sharing) — this ADR closes its deferred notification gap.
- Spec: `specs/wishlist-sharing/plan.md`.
