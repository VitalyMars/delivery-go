package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/VitalyMars/delivery_go/services/delivery/internal/event"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *DeliveryRepository) AssignNextWaitingDelivery(
	ctx context.Context,
) (bool, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, fmt.Errorf("begin queue transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Сериализуем обработку очереди, чтобы сохранять строгий FIFO.
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(728104)`)
	if err != nil {
		return false, fmt.Errorf("lock delivery queue: %w", err)
	}

	// Lock a free courier. Concurrent workers cannot claim this courier.
	var courier Courier
	err = tx.QueryRow(ctx, `
	SELECT c.id, c.name, c.phone, c.status
	FROM couriers c
	WHERE c.status = 'available'
	  AND NOT EXISTS (
		  SELECT 1
		  FROM deliveries d
		  WHERE d.courier_id = c.id
		    AND d.status IN ('courier_assigned', 'delivering')
	  )
	ORDER BY c.created_at, c.id
	LIMIT 1
	FOR UPDATE OF c SKIP LOCKED
`).Scan(&courier.ID, &courier.Name, &courier.Phone, &courier.Status)

	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("select available courier: %w", err)
	}

	// Claim the oldest waiting delivery.
	var delivery Delivery
	err = tx.QueryRow(ctx, `
		SELECT id, order_id, event_id, restaurant_id,
		       delivery_address, total_amount, order_created_at
		FROM deliveries
		WHERE status = 'waiting_for_courier'
		  AND courier_id IS NULL
		ORDER BY order_created_at, created_at, id
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`).Scan(
		&delivery.ID,
		&delivery.OrderID,
		&delivery.EventID,
		&delivery.RestaurantID,
		&delivery.DeliveryAddress,
		&delivery.TotalAmount,
		&delivery.OrderCreatedAt,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("select waiting delivery: %w", err)
	}

	now := time.Now().UTC()
	estimatedAt := now.Add(15 * time.Minute)

	_, err = tx.Exec(ctx, `
		UPDATE couriers
		SET status = 'busy'
		WHERE id = $1
	`, courier.ID)
	if err != nil {
		return false, fmt.Errorf("mark courier busy: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE deliveries
		SET courier_id = $1,
		    status = 'courier_assigned',
		    assigned_at = $2,
		    updated_at = $2, estimated_at = $4
		WHERE id = $3
	`, courier.ID, now, delivery.ID, estimatedAt)
	if err != nil {
		return false, fmt.Errorf("assign waiting delivery: %w", err)
	}

	outgoing := event.CourierAssigned{
		EventID:             uuid.NewString(),
		EventType:           "delivery.courier_assigned",
		OccurredAt:          now,
		OrderID:             delivery.OrderID.String(),
		DeliveryID:          delivery.ID.String(),
		CourierID:           courier.ID.String(),
		CourierName:         courier.Name,
		DeliveryAddress:     delivery.DeliveryAddress,
		AssignedAt:          now,
		EstimatedDeliveryAt: estimatedAt,
		TotalAmount:         delivery.TotalAmount,
	}

	payload, err := json.Marshal(outgoing)
	if err != nil {
		return false, fmt.Errorf("marshal courier assigned event: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (id, aggregate_id, event_type, payload)
		VALUES ($1, $2, $3, $4)
	`, outgoing.EventID, delivery.OrderID, outgoing.EventType, payload)
	if err != nil {
		return false, fmt.Errorf("insert courier assigned outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit queue assignment: %w", err)
	}

	return true, nil
}
