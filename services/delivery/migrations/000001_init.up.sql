CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE couriers (
id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
name TEXT NOT NULL,
phone TEXT NOT NULL UNIQUE,
status TEXT NOT NULL DEFAULT 'available'
CHECK (status IN ('available', 'busy', 'inactive')),
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE deliveries (
id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
order_id UUID NOT NULL UNIQUE,
event_id UUID NOT NULL UNIQUE,
restaurant_id UUID NOT NULL,
courier_id UUID REFERENCES couriers(id),
status TEXT NOT NULL DEFAULT 'courier_assigned'
CHECK (status IN (
'waiting_for_courier',
'courier_assigned',
'delivering',
'delivered',
'cancelled'
)),
delivery_address TEXT NOT NULL,
total_amount BIGINT NOT NULL CHECK (total_amount >= 0),
order_created_at TIMESTAMPTZ NOT NULL,
assigned_at TIMESTAMPTZ,
picked_up_at TIMESTAMPTZ,
delivered_at TIMESTAMPTZ,
created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_deliveries_courier_id ON deliveries(courier_id);
CREATE INDEX idx_deliveries_status ON deliveries(status);

CREATE TABLE delivery_items (
id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
delivery_id UUID NOT NULL REFERENCES deliveries(id) ON DELETE CASCADE,
dish_id UUID NOT NULL,
name TEXT NOT NULL,
quantity INT NOT NULL CHECK (quantity > 0),
unit_price BIGINT NOT NULL CHECK (unit_price >= 0)
);

CREATE INDEX idx_delivery_items_delivery_id
ON delivery_items(delivery_id);

INSERT INTO couriers (name, phone) VALUES
('Alex Courier', '+79990000001'),
('Maria Courier', '+79990000002'),
('Ivan Courier', '+79990000003');
