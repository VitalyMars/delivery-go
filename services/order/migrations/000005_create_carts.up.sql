CREATE TABLE carts (
    cart_token UUID PRIMARY KEY,
    restaurant_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO carts (cart_token)
SELECT DISTINCT cart_token
FROM cart_items
ON CONFLICT (cart_token) DO NOTHING;