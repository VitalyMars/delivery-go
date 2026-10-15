CREATE TABLE cart_items (
    id UUID PRIMARY KEY,

    cart_token UUID NOT NULL,

    dish_id UUID NOT NULL,

    quantity INTEGER NOT NULL CHECK (quantity > 0),

    price INTEGER NOT NULL CHECK (price >= 0),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_cart_items_cart_token
    ON cart_items(cart_token);