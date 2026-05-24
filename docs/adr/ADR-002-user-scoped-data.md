# ADR-002: User-Scoped Collection Data

## Status
Accepted

## Context
All collection items, wishlist entries, and condition history must be private to the owning user. We need a data model that enforces this at the database level, not just the application layer.

## Decision
All user-specific tables (collection_items, wishlist_items, condition_history, item_photos) include a user_id foreign key referencing users.id. All API queries include a WHERE user_id = ? clause. Row-level isolation is enforced at both schema and query level.

## Consequences
- User data is always scoped at query time
- No risk of cross-user data leakage
- Slightly more verbose queries (always include user_id predicate)
