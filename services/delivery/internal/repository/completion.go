package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrDeliveryNotFound       = errors.New("delivery not found")
	ErrDeliveryAlreadyDone    = errors.New("delivery already completed")
	ErrDeliveryCannotComplete = errors.New("delivery cannot be completed in its current status")
)

func (r *DeliveryRepository) CompleteDelivery(
	ctx context.Context,
	deliveryID uuid.UUID,
) error {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin completion transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var status string

	err = tx.QueryRow(ctx, `
		SELECT status
		FROM deliveries
		WHERE id = $1
		FOR UPDATE
	`, deliveryID).Scan(&status)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrDeliveryNotFound
	}
	if err != nil {
		return fmt.Errorf("get delivery: %w", err)
	}

	if status == "delivered" {
		return ErrDeliveryAlreadyDone
	}
	if status != "courier_assigned" && status != "delivering" {
		return ErrDeliveryCannotComplete
	}

	var courierID uuid.UUID

	err = tx.QueryRow(ctx, `
		SELECT courier_id
		FROM deliveries
		WHERE id = $1
	`, deliveryID).Scan(&courierID)
	if err != nil {
		return fmt.Errorf("get delivery courier: %w", err)
	}

	now := time.Now().UTC()

	_, err = tx.Exec(ctx, `
		UPDATE deliveries
		SET status = 'delivered',
		    delivered_at = $1,
		    updated_at = $1
		WHERE id = $2
	`, now, deliveryID)
	if err != nil {
		return fmt.Errorf("mark delivery completed: %w", err)
	}

	_, err = tx.Exec(ctx, `
		UPDATE couriers
		SET status = 'available'
		WHERE id = $1 AND status = 'busy'
	`, courierID)
	if err != nil {
		return fmt.Errorf("release courier: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit completion transaction: %w", err)
	}

	return nil
}
