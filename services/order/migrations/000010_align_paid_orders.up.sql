-- The previous demo flow left paid orders in 'created'. Preserve existing
-- terminal/assigned states while aligning unpaid-vs-paid lifecycle semantics.
WITH aligned AS (
    UPDATE orders SET status = 'ready_for_delivery', updated_at = now()
    WHERE status = 'created' AND payment_status = 'PAID'
    RETURNING id
)
INSERT INTO order_status_history (id, order_id, status, reason)
SELECT gen_random_uuid(), id, 'ready_for_delivery', 'Align legacy paid order'
FROM aligned;

CREATE INDEX idx_order_items_order_created ON order_items (order_id, created_at);
CREATE INDEX idx_order_status_history_order_created ON order_status_history (order_id, created_at);
