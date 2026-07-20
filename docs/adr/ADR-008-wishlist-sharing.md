# ADR-008: Identity-Based Wishlist Sharing

## Status
Accepted

## Context
AudioFile already supports anonymous wishlist sharing via a public link (`GET /api/public/wishlist/{userID}` surfaced at `/wishlist?share={userID}`). Anyone with the link can view a wishlist read-only. This works for "send a link to a friend over text," but it has gaps:

- There is no way to share *to a specific app user* by identity.
- Recipients have no in-app inbox of wishlists shared with them ("Shared with me").
- There is no revocation tied to a known recipient — you can only hope a copied URL isn't further shared.
- The `profiles` table exists in the schema but is never populated (no signup trigger), so there is no searchable user handle to target a share at.

Users want to share a wishlist to another user of the app and see wishlists others have shared with them.

## Decision
Introduce identity-based, in-app wishlist sharing alongside (not replacing) public-link sharing:

1. **Single list per user.** Sharing means sharing the user's entire wishlist. Multiple named lists are a future feature.
2. **Share by username, no friends graph.** A user searches another user by username and shares directly. No friend requests, no follower model.
3. **Auto-appear.** A share lands immediately in the recipient's "Shared with me" view. No accept/decline step.
4. **New `wishlist_shares` table** with `(owner_id, viewer_id)`, unique pair, `CHECK (owner_id <> viewer_id)`, cascading on user deletion. One row = one granted share. Revocation = deleting the row.
5. **`username` on `profiles`**, auto-populated on signup by a `handle_new_user` trigger on `auth.users` (derived from the email local-part, suffixed on collision), user-editable later. This also puts the previously-dormant `profiles` table into use.
6. **`wishlist_shares` is owner-managed**: only the owner creates/deletes shares of their wishlist; viewers can read shares granted to them. The backend enforces this at the query layer; RLS policies mirror it for direct client access.

## Consequences
- A share is a first-class, revocable relationship between two known users, independent of the anonymous public link.
- Deleting the row revokes access immediately and atomically.
- The `profiles` table now carries live data; a new migration backfills no rows (trigger only applies to new signups) — existing dev accounts without profiles must be handled manually in dev (`supabase db reset`).
- `username` uniqueness is enforced at the DB; collisions during signup are resolved by the trigger with a numeric suffix.
- Sharing is whole-list only. Per-item or named-list sharing will require a schema change to introduce a `wishlist_groups`/lists entity — explicitly deferred.
- ~~No notifications on share arrival in this iteration; recipients discover shares when they open "Shared with me."~~ Superseded by ADR-010: share arrival now creates an in-app notification (bell + inbox) and a best-effort email, and public links can be claimed into "Shared with me."

## Alternatives considered
- **Full friends system (requests, accept/decline, friends list).** Rejected for now — higher UI and model cost; share-by-username gives 90% of the value at a fraction of the scope. Can be layered on later without schema changes to `wishlist_shares`.
- **Accept/decline on incoming shares.** Rejected for MVP — auto-appear is simpler and matches how the public link already behaves (no gate). Adding a `status` column later is additive.
- **Per-list / named wishlists.** Rejected — the current model is one list per user and changing that is a larger refactor unrelated to sharing.
- **Reuse the public-link mechanism only.** Rejected — it has no recipient identity, no inbox, and no per-recipient revocation, which are the explicit asks.

## References
- Spec: `specs/wishlist-sharing/plan.md`
- ADR-002 (user-scoped data) — sharing extends, not violates, user-scoping: queries remain owner/viewer scoped.
- ADR-007 (Supabase Postgres) — uses Supabase auth.users and RLS.
