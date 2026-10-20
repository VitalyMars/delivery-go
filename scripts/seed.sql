-- Explicit development data; never applied by migrations automatically.
INSERT INTO restaurants (id, name, address)
VALUES ('11111111-1111-4111-8111-111111111111', 'Demo Restaurant', 'Demo street, 1')
ON CONFLICT (id) DO NOTHING;
INSERT INTO dishes (id, restaurant_id, name, description, price, available)
VALUES ('22222222-2222-4222-8222-222222222222', '11111111-1111-4111-8111-111111111111',
        'Demo dish', 'For local API exercises', 35000, true)
ON CONFLICT (id) DO NOTHING;
