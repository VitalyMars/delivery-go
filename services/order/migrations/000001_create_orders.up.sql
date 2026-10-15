CREATE TABLE orders (
    id UUID PRIMARY KEY,
    token UUID NOT NULL UNIQUE,

    customer_name TEXT NOT NULL,
    customer_phone TEXT NOT NULL,
    delivery_address TEXT NOT NULL,
    comment TEXT,

    status TEXT NOT NULL DEFAULT 'new',

    idempotency_key TEXT NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_orders_idempotency_key
    ON orders(idempotency_key);


CREATE TABLE order_items (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,

    dish_id UUID NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    price INTEGER NOT NULL CHECK (price >= 0),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);


CREATE TABLE order_status_history (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,

    status TEXT NOT NULL,
    reason TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);