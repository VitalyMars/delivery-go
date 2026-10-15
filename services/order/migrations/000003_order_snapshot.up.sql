ALTER TABLE orders
    ADD COLUMN total INTEGER NOT NULL DEFAULT 0;

ALTER TABLE order_items
    ADD COLUMN name TEXT NOT NULL DEFAULT '';

ALTER TABLE orders
    ALTER COLUMN status SET DEFAULT 'created';

UPDATE orders
SET status = 'created'
WHERE status = 'new';