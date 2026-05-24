-- CrateKeeper schema — Postgres (Supabase)
-- Replaces SQLite schema from ADR-005

-- Supabase provides auth.users; we reference it instead of rolling our own.
-- display_name and email live in auth.users; this table is for app-specific profile data.
CREATE TABLE IF NOT EXISTS public.profiles (
    id          UUID PRIMARY KEY REFERENCES auth.users(id) ON DELETE CASCADE,
    display_name TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Global release metadata (ADR-003)
CREATE TABLE IF NOT EXISTS public.releases (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    discogs_id  TEXT UNIQUE,
    title       TEXT NOT NULL,
    artist      TEXT NOT NULL,
    year        INTEGER,
    label       TEXT,
    catalog_no  TEXT,
    format      TEXT,       -- LP, EP, 45, etc.
    country     TEXT,
    genres      JSONB,      -- JSON array
    cover_url   TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- User-owned collection items (ADR-002)
CREATE TABLE IF NOT EXISTS public.collection_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    release_id      UUID NOT NULL REFERENCES public.releases(id),
    media_condition TEXT NOT NULL DEFAULT 'VG' CHECK (media_condition IN ('M','NM','VG+','VG','G+','G','F','P')),
    sleeve_condition TEXT NOT NULL DEFAULT 'VG' CHECK (sleeve_condition IN ('M','NM','VG+','VG','G+','G','F','P')),
    purchase_price  NUMERIC(10,2),
    purchase_date   DATE,
    notes           TEXT,
    is_for_sale     BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Condition history log
CREATE TABLE IF NOT EXISTS public.condition_history (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    collection_item_id  UUID NOT NULL REFERENCES public.collection_items(id) ON DELETE CASCADE,
    media_condition     TEXT NOT NULL,
    sleeve_condition    TEXT NOT NULL,
    warp_notes          TEXT,
    scratch_notes       TEXT,
    cleaning_notes      TEXT,
    playback_notes      TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Wishlist
CREATE TABLE IF NOT EXISTS public.wishlist_items (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    release_id      UUID REFERENCES public.releases(id),
    -- Manual entry if no Discogs match
    manual_title    TEXT,
    manual_artist   TEXT,
    priority        INTEGER NOT NULL DEFAULT 5 CHECK (priority BETWEEN 1 AND 10),
    target_price    NUMERIC(10,2),
    pressing_notes  TEXT,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Photos attached to collection items
CREATE TABLE IF NOT EXISTS public.item_photos (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    collection_item_id  UUID NOT NULL REFERENCES public.collection_items(id) ON DELETE CASCADE,
    storage_path        TEXT NOT NULL,
    thumbnail_path      TEXT,
    caption             TEXT,
    sort_order          INTEGER NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Full-text search (Postgres version of FTS5)
ALTER TABLE public.releases ADD COLUMN IF NOT EXISTS fts tsvector
    GENERATED ALWAYS AS (to_tsvector('english', coalesce(title,'') || ' ' || coalesce(artist,'') || ' ' || coalesce(label,'') || ' ' || coalesce(catalog_no,''))) STORED;

CREATE INDEX IF NOT EXISTS idx_releases_fts ON public.releases USING gin(fts);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_collection_items_user ON public.collection_items(user_id);
CREATE INDEX IF NOT EXISTS idx_collection_items_release ON public.collection_items(release_id);
CREATE INDEX IF NOT EXISTS idx_wishlist_items_user ON public.wishlist_items(user_id);
CREATE INDEX IF NOT EXISTS idx_item_photos_item ON public.item_photos(collection_item_id);

-- updated_at trigger
CREATE OR REPLACE FUNCTION public.update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER set_profiles_updated_at BEFORE UPDATE ON public.profiles
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();
CREATE TRIGGER set_releases_updated_at BEFORE UPDATE ON public.releases
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();
CREATE TRIGGER set_collection_items_updated_at BEFORE UPDATE ON public.collection_items
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();
CREATE TRIGGER set_wishlist_items_updated_at BEFORE UPDATE ON public.wishlist_items
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();

-- RLS policies (Supabase standard)
ALTER TABLE public.profiles ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.collection_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.wishlist_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.condition_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.item_photos ENABLE ROW LEVEL SECURITY;

-- Users can only see/edit their own data
CREATE POLICY "Users can view own profile" ON public.profiles FOR SELECT USING (auth.uid() = id);
CREATE POLICY "Users can update own profile" ON public.profiles FOR UPDATE USING (auth.uid() = id);

CREATE POLICY "Users can view own collection" ON public.collection_items FOR SELECT USING (auth.uid() = user_id);
CREATE POLICY "Users can insert own collection" ON public.collection_items FOR INSERT WITH CHECK (auth.uid() = user_id);
CREATE POLICY "Users can update own collection" ON public.collection_items FOR UPDATE USING (auth.uid() = user_id);
CREATE POLICY "Users can delete own collection" ON public.collection_items FOR DELETE USING (auth.uid() = user_id);

CREATE POLICY "Users can view own wishlist" ON public.wishlist_items FOR SELECT USING (auth.uid() = user_id);
CREATE POLICY "Users can insert own wishlist" ON public.wishlist_items FOR INSERT WITH CHECK (auth.uid() = user_id);
CREATE POLICY "Users can update own wishlist" ON public.wishlist_items FOR UPDATE USING (auth.uid() = user_id);
CREATE POLICY "Users can delete own wishlist" ON public.wishlist_items FOR DELETE USING (auth.uid() = user_id);

CREATE POLICY "Users can view own condition history" ON public.condition_history FOR SELECT USING (
    auth.uid() = (SELECT user_id FROM public.collection_items WHERE id = collection_item_id)
);
CREATE POLICY "Users can insert own condition history" ON public.condition_history FOR INSERT WITH CHECK (
    auth.uid() = (SELECT user_id FROM public.collection_items WHERE id = collection_item_id)
);

CREATE POLICY "Users can view own photos" ON public.item_photos FOR SELECT USING (
    auth.uid() = (SELECT user_id FROM public.collection_items WHERE id = collection_item_id)
);
CREATE POLICY "Users can insert own photos" ON public.item_photos FOR INSERT WITH CHECK (
    auth.uid() = (SELECT user_id FROM public.collection_items WHERE id = collection_item_id)
);
CREATE POLICY "Users can delete own photos" ON public.item_photos FOR DELETE USING (
    auth.uid() = (SELECT user_id FROM public.collection_items WHERE id = collection_item_id)
);

-- Releases are publicly readable (global metadata, ADR-003)
ALTER TABLE public.releases ENABLE ROW LEVEL SECURITY;
CREATE POLICY "Releases are publicly readable" ON public.releases FOR SELECT USING (true);

-- Seed data for dev
INSERT INTO public.releases (id, title, artist, year, label) VALUES
    ('a0000001-0000-0000-0000-000000000001', 'Kind of Blue', 'Miles Davis', 1959, 'Blue Note'),
    ('a0000001-0000-0000-0000-000000000002', 'A Love Supreme', 'John Coltrane', 1965, 'Impulse!'),
    ('a0000001-0000-0000-0000-000000000003', 'Exodus', 'Bob Marley', 1977, 'Island'),
    ('a0000001-0000-0000-0000-000000000004', 'Purple Rain', 'Prince', 1984, 'Warner'),
    ('a0000001-0000-0000-0000-000000000005', 'Rumours', 'Fleetwood Mac', 1977, 'Warner'),
    ('a0000001-0000-0000-0000-000000000006', 'Blue', 'Joni Mitchell', 1971, 'Reprise'),
    ('a0000001-0000-0000-0000-000000000007', 'In the Wee Small Hours', 'Frank Sinatra', 1955, 'Capitol'),
    ('a0000001-0000-0000-0000-000000000008', 'Sketches of Spain', 'Miles Davis', 1960, 'Columbia'),
    ('a0000001-0000-0000-0000-000000000009', 'Getz / Gilberto', 'Stan Getz & João Gilberto', 1964, 'Verve');
