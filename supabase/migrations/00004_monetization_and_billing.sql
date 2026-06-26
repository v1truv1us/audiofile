-- Migration 00004: Monetization, Billing, and VIP Exemption

-- 1. Extend public.profiles with administrative and VIP flags
ALTER TABLE public.profiles ADD COLUMN IF NOT EXISTS is_vip BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE public.profiles ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT false;

-- 2. Prevent self-elevation of is_vip and is_admin by users via direct API updates
CREATE OR REPLACE FUNCTION public.prevent_profile_elevation()
RETURNS TRIGGER AS $$
BEGIN
    IF auth.uid() = NEW.id THEN
        IF NEW.is_vip IS DISTINCT FROM OLD.is_vip THEN
            NEW.is_vip := OLD.is_vip;
        END IF;
        IF NEW.is_admin IS DISTINCT FROM OLD.is_admin THEN
            NEW.is_admin := OLD.is_admin;
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

DROP TRIGGER IF EXISTS enforce_profile_elevation ON public.profiles;
CREATE TRIGGER enforce_profile_elevation
    BEFORE UPDATE ON public.profiles
    FOR EACH ROW EXECUTE FUNCTION public.prevent_profile_elevation();

-- 3. Create public.subscriptions table (isolated from globally-readable profiles)
CREATE TABLE IF NOT EXISTS public.subscriptions (
    id                     TEXT PRIMARY KEY, -- Stripe Subscription ID
    user_id                UUID NOT NULL UNIQUE REFERENCES auth.users(id) ON DELETE CASCADE,
    stripe_customer_id     TEXT UNIQUE,
    price_id               TEXT,
    tier                   TEXT NOT NULL DEFAULT 'free' CHECK (tier IN ('free', 'premium')),
    status                 TEXT NOT NULL DEFAULT 'inactive' CHECK (status IN (
        'active', 'trialing', 'past_due', 'canceled', 'unpaid', 'incomplete', 'incomplete_expired', 'inactive'
    )),
    current_period_end     TIMESTAMPTZ,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 4. Create indexes for high-performance querying
CREATE INDEX IF NOT EXISTS idx_subscriptions_user_id ON public.subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_stripe_customer_id ON public.subscriptions(stripe_customer_id);

-- 5. Enable Row-Level Security
ALTER TABLE public.subscriptions ENABLE ROW LEVEL SECURITY;

-- 6. RLS Policies
-- Subscriptions are only readable by their owner
DROP POLICY IF EXISTS "Users can view own subscription" ON public.subscriptions;
CREATE POLICY "Users can view own subscription" ON public.subscriptions
    FOR SELECT USING (auth.uid() = user_id);

-- 7. Trigger for updated_at tracking
DROP TRIGGER IF EXISTS set_subscriptions_updated_at ON public.subscriptions;
CREATE TRIGGER set_subscriptions_updated_at BEFORE UPDATE ON public.subscriptions
    FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();
