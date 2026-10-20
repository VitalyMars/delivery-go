DROP INDEX idx_order_status_history_order_created;
DROP INDEX idx_order_items_order_created;
-- Status/history backfill is intentionally not reversed: newer delivery events
-- may already have advanced these orders. A schema rollback must not regress them.
