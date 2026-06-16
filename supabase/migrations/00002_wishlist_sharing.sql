-- Identity-based wishlist sharing

ALTER TABLE public.profiles ADD COLUMN IF NOT EXISTS username TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_profiles_username
    ON public.profiles(lower(username))
    WHERE username IS NOT NULL;

DROP TRIGGER IF EXISTS on_auth_user_created ON auth.users;
DROP FUNCTION IF EXISTS public.handle_new_user();
CREATE OR REPLACE FUNCTION public.handle_new_user()
RETURNS TRIGGER
SECURITY DEFINER
SET search_path = public
AS $$
DECLARE
    base_username TEXT;
    candidate_username TEXT;
    suffix INTEGER := 1;
BEGIN
    base_username := regexp_replace(lower(split_part(NEW.email, '@', 1)), '[^a-z0-9_]', '', 'g');
    IF base_username = '' THEN
        base_username := 'user';
    END IF;
    candidate_username := base_username;

    LOOP
        BEGIN
            INSERT INTO public.profiles (id, username)
            VALUES (NEW.id, candidate_username)
            ON CONFLICT (id) DO UPDATE SET updated_at = now();
            RETURN NEW;
        EXCEPTION WHEN unique_violation THEN
            suffix := suffix + 1;
            candidate_username := base_username || '-' || suffix::text;
        END;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER on_auth_user_created
    AFTER INSERT ON auth.users
    FOR EACH ROW EXECUTE FUNCTION public.handle_new_user();

DROP POLICY IF EXISTS "Profiles are readable" ON public.profiles;
CREATE POLICY "Profiles are readable" ON public.profiles
    FOR SELECT USING (true);

CREATE TABLE IF NOT EXISTS public.wishlist_shares (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id   UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    viewer_id  UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    message    TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (owner_id, viewer_id),
    CHECK (owner_id <> viewer_id)
);

CREATE INDEX IF NOT EXISTS idx_wishlist_shares_owner ON public.wishlist_shares(owner_id);
CREATE INDEX IF NOT EXISTS idx_wishlist_shares_viewer ON public.wishlist_shares(viewer_id);

ALTER TABLE public.wishlist_shares ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS "Owners can view own wishlist shares" ON public.wishlist_shares;
CREATE POLICY "Owners can view own wishlist shares" ON public.wishlist_shares
    FOR SELECT USING (auth.uid() = owner_id);

DROP POLICY IF EXISTS "Viewers can view granted wishlist shares" ON public.wishlist_shares;
CREATE POLICY "Viewers can view granted wishlist shares" ON public.wishlist_shares
    FOR SELECT USING (auth.uid() = viewer_id);

DROP POLICY IF EXISTS "Owners can create wishlist shares" ON public.wishlist_shares;
CREATE POLICY "Owners can create wishlist shares" ON public.wishlist_shares
    FOR INSERT WITH CHECK (auth.uid() = owner_id);

DROP POLICY IF EXISTS "Owners can delete wishlist shares" ON public.wishlist_shares;
CREATE POLICY "Owners can delete wishlist shares" ON public.wishlist_shares
    FOR DELETE USING (auth.uid() = owner_id);
