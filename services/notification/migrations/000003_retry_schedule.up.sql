ALTER TABLE notifications ADD COLUMN next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE INDEX idx_notifications_retry ON notifications (next_attempt_at, created_at)
WHERE status IN ('pending', 'failed') AND attempts < 10;
