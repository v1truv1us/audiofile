-- Backfill profile rows for auth.users created before migration 00002.
--
-- The handle_new_user() trigger (added in 00002) only fires on NEW signups,
-- so accounts created earlier have no row in public.profiles and therefore
-- no username — they are invisible to /api/profiles/search and cannot be
-- shared to. This migration creates a profile row (with a derived username)
-- for every auth.users row that is still missing one.
--
-- Idempotent: safe to re-run. The WHERE NOT EXISTS guard skips users who
-- already have a profile row, and ON CONFLICT (id) DO UPDATE handles races.
-- Username derivation mirrors handle_new_user() exactly (email local-part,
-- lowercased, non-[a-z0-9_] stripped, numeric suffix on collision).

DO $$
DECLARE
    u RECORD;
    base_username TEXT;
    candidate TEXT;
    suffix INTEGER;
BEGIN
    FOR u IN
        SELECT id, email
        FROM auth.users au
        WHERE NOT EXISTS (
            SELECT 1 FROM public.profiles p WHERE p.id = au.id
        )
    LOOP
        base_username := regexp_replace(lower(split_part(u.email, '@', 1)), '[^a-z0-9_]', '', 'g');
        IF base_username = '' THEN
            base_username := 'user';
        END IF;

        candidate := base_username;
        suffix := 1;
        LOOP
            BEGIN
                INSERT INTO public.profiles (id, username)
                VALUES (u.id, candidate)
                ON CONFLICT (id) DO UPDATE SET updated_at = now();
                EXIT;
            EXCEPTION WHEN unique_violation THEN
                suffix := suffix + 1;
                candidate := base_username || '-' || suffix::text;
            END;
        END LOOP;
    END LOOP;
END $$;
