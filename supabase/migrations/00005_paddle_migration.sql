-- Migration 00005: Migrate billing from Stripe to Paddle

-- 1. Rename stripe_customer_id column to paddle_customer_id
ALTER TABLE public.subscriptions RENAME COLUMN stripe_customer_id TO paddle_customer_id;

-- 2. Update index
DROP INDEX IF EXISTS idx_subscriptions_stripe_customer_id;
CREATE INDEX idx_subscriptions_paddle_customer_id ON public.subscriptions(paddle_customer_id);

-- 3. Update comment on subscriptions.id
COMMENT ON COLUMN public.subscriptions.id IS 'Paddle Transaction/Subscription ID';

-- 4. Add 'paused' to allowed status values for Paddle compatibility
ALTER TABLE public.subscriptions DROP CONSTRAINT IF EXISTS subscriptions_status_check;
ALTER TABLE public.subscriptions ADD CONSTRAINT subscriptions_status_check CHECK (status IN (
    'active', 'trialing', 'past_due', 'canceled', 'unpaid', 'incomplete', 'incomplete_expired', 'inactive', 'paused'
));
