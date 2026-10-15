DROP INDEX IF EXISTS idx_orders_user_id;

ALTER TABLE orders
DROP COLUMN IF EXISTS user_id;