ALTER TABLE orders
    ADD COLUMN payment_status TEXT NOT NULL DEFAULT 'pending';

ALTER TABLE orders
    ADD COLUMN payment_id UUID;