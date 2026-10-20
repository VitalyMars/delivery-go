DROP INDEX idx_notifications_retry;
ALTER TABLE notifications DROP COLUMN next_attempt_at;
