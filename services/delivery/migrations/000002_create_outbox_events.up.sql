CREATE TABLE outbox_events (
id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
aggregate_id UUID NOT NULL,
event_type TEXT NOT NULL,
payload JSONB NOT NULL,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
published_at TIMESTAMPTZ
);

CREATE INDEX idx_delivery_outbox_unpublished
ON outbox_events (created_at, id)
WHERE published_at IS NULL;
