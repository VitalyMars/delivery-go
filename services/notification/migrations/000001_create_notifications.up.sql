CREATE TABLE notifications (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    order_id UUID NOT NULL,
    recipient_email VARCHAR(254) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    body TEXT NOT NULL,

    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'sent', 'failed')),

    attempts INT NOT NULL DEFAULT 0
        CHECK (attempts >= 0),

    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ
);

CREATE INDEX idx_notifications_user_created_at
    ON notifications (user_id, created_at DESC);

CREATE INDEX idx_notifications_pending
    ON notifications (created_at)
    WHERE status IN ('pending', 'failed');