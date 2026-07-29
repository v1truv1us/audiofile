-- Migration 00008: Enable RLS on paddle_webhook_events
--
-- paddle_webhook_events is an audit/idempotency log of incoming Paddle webhooks.
-- It is written exclusively by the backend service via its privileged Postgres
-- connection (the Supabase service role bypasses RLS — see ADR-010), so backend
-- inserts are unaffected by RLS. The table must not be readable or writable
-- through the public PostgREST API (anon/authenticated roles).
--
-- Enabling RLS with no policies denies all client API access by default, which
-- resolves the Supabase Security advisor warning:
--   "Table public.paddle_webhook_events is public, but RLS has not been enabled."

ALTER TABLE public.paddle_webhook_events
    ENABLE ROW LEVEL SECURITY;
