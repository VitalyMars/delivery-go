ALTER TABLE order_items
    DROP COLUMN name;

ALTER TABLE orders
    DROP COLUMN total;

ALTER TABLE orders
    ALTER COLUMN status SET DEFAULT 'new';