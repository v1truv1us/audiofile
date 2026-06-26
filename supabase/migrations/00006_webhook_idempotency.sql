CREATE TABLE IF NOT EXISTS public.paddle_webhook_events (
    event_id     TEXT PRIMARY KEY,
    event_type   TEXT NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_paddle_webhook_events_processed_at
    ON public.paddle_webhook_events(processed_at);
