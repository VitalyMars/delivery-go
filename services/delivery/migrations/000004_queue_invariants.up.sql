CREATE UNIQUE INDEX idx_deliveries_active_courier
ON deliveries (courier_id)
WHERE courier_id IS NOT NULL AND status IN ('courier_assigned', 'delivering');

CREATE INDEX idx_deliveries_waiting_fifo
ON deliveries (order_created_at, created_at, id)
WHERE status = 'waiting_for_courier' AND courier_id IS NULL;
