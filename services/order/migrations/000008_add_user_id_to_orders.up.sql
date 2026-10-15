ALTER TABLE orders
ADD COLUMN user_id UUID;

CREATE INDEX idx_orders_user_id
ON orders (user_id)
WHERE user_id IS NOT NULL;