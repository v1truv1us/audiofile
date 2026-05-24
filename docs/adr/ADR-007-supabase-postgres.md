# ADR-007: Supabase Postgres for Production

## Status
Accepted

## Context
ADR-005 chose SQLite for MVP simplicity. The app is growing beyond single-user, and hosting SQLite requires managing file-based DB on a server. Supabase provides managed Postgres with built-in auth, storage, and RLS.

## Decision
- Use Supabase Postgres as the primary database (local dev via `supabase start`, cloud for production)
- Keep the Go backend as API layer — it connects to Postgres via pgxpool
- Use Supabase Auth for passkey + email authentication
- Row Level Security (RLS) policies enforce user-scoped data access at the DB level
- FTS5 (SQLite) replaced by Postgres `tsvector` + GIN index

## Consequences
- No more local SQLite file — `supabase start` runs Postgres in Docker for dev
- Go backend uses `pgxpool` instead of `database/sql` + SQLite driver
- Schema uses Postgres-native types (UUID, TIMESTAMPTZ, JSONB, NUMERIC)
- RLS policies enforce data isolation without application-level checks
- Migration to plain Postgres (non-Supabase) is straightforward if needed
