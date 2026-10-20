DROP INDEX IF EXISTS idx_notifications_event_id;

ALTER TABLE notifications
DROP COLUMN IF EXISTS event_id;