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
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNoAvailableCourier = errors.New("no available courier")

type DeliveryRepository struct {
	db *pgxpool.Pool
}

func NewDeliveryRepository(db *pgxpool.Pool) *DeliveryRepository {
	return &DeliveryRepository{db: db}
}

func (r *DeliveryRepository) CreateFromOrderEvent(
	ctx context.Context,
	e event.OrderReadyForDelivery,
) (*Delivery, error) {
	orderID, err := uuid.Parse(e.OrderID)
	if err != nil {
		return nil, fmt.Errorf("parse order ID: %w", err)
	}

	eventID, err := uuid.Parse(e.EventID)
	if err != nil {
		return nil, fmt.Errorf("parse event ID: %w", err)
	}

	if len(e.Restaurants) == 0 {
		return nil, errors.New("order event has no restaurants")
	}

	if e.TotalAmount < 0 {
		return nil, errors.New("order total amount cannot be negative")
	}

	for _, restaurant := range e.Restaurants {
		if _, err := uuid.Parse(restaurant.RestaurantID); err != nil {
			return nil, fmt.Errorf("parse restaurant ID: %w", err)
		}

		for _, item := range restaurant.Items {
			if _, err := uuid.Parse(item.DishID); err != nil {
				return nil, fmt.Errorf("parse dish ID: %w", err)
			}
			if item.Quantity <= 0 {
				return nil, errors.New("item quantity must be positive")
			}
			if item.UnitPrice < 0 {
				return nil, errors.New("item unit price cannot be negative")
			}
		}
	}

	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin delivery transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Serialize delivery creation and queue assignment.
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(728104)`)
	if err != nil {
		return nil, fmt.Errorf("lock delivery queue: %w", err)
	}

	// A previously processed order must not receive another delivery.

	// A previously processed order must not receive another delivery.
	var existing Delivery
	err = tx.QueryRow(ctx, `
SELECT id, order_id, event_id, restaurant_id,
       status, delivery_address, total_amount, order_created_at
FROM deliveries
WHERE order_id = $1
	`, orderID).Scan(
		&existing.ID,
		&existing.OrderID,
		&existing.EventID,
		&existing.RestaurantID,
		&existing.Status,
		&existing.DeliveryAddress,
		&existing.TotalAmount,
		&existing.OrderCreatedAt,
	)

	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit duplicate check: %w", err)
		}
		return &existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("check existing delivery: %w", err)
	}

	// Lock one available courier so concurrent transactions cannot
	// assign the same courier.
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
		restaurantID, parseErr := uuid.Parse(e.Restaurants[0].RestaurantID)
		if parseErr != nil {
			return nil, fmt.Errorf("parse restaurant ID: %w", parseErr)
		}

		deliveryID := uuid.New()
		now := time.Now().UTC()

		_, err = tx.Exec(ctx, `
        INSERT INTO deliveries (
            id, order_id, event_id, restaurant_id, courier_id,
            status, delivery_address, total_amount, order_created_at,
            created_at, updated_at
        )
        VALUES ($1, $2, $3, $4, NULL, 'waiting_for_courier',
                $5, $6, $7, $8, $8)
    `,
			deliveryID,
			orderID,
			eventID,
			restaurantID,
			e.DeliveryAddress,
			e.TotalAmount,
			e.OrderCreatedAt,
			now,
		)
		if err != nil {
			return nil, fmt.Errorf("insert pending delivery: %w", err)
		}

		for _, restaurant := range e.Restaurants {
			for _, item := range restaurant.Items {
				_, err = tx.Exec(ctx, `
                INSERT INTO delivery_items (
                    delivery_id, dish_id, name, quantity, unit_price
                )
                VALUES ($1, $2, $3, $4, $5)
            `,
					deliveryID,
					item.DishID,
					item.Name,
					item.Quantity,
					item.UnitPrice,
				)
				if err != nil {
					return nil, fmt.Errorf("insert pending delivery item: %w", err)
				}
			}
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, fmt.Errorf("commit pending delivery: %w", err)
		}

		return &Delivery{
			ID:              deliveryID,
			OrderID:         orderID,
			EventID:         eventID,
			RestaurantID:    restaurantID,
			Status:          "waiting_for_courier",
			DeliveryAddress: e.DeliveryAddress,
			TotalAmount:     e.TotalAmount,
			OrderCreatedAt:  e.OrderCreatedAt,
		}, nil
	}

	if err != nil {
		return nil, fmt.Errorf("select available courier: %w", err)
	}

	now := time.Now().UTC()
	estimatedAt := now.Add(15 * time.Minute)
	deliveryID := uuid.New()
	restaurantID, _ := uuid.Parse(e.Restaurants[0].RestaurantID)

	_, err = tx.Exec(ctx, `
		UPDATE couriers
		SET status = 'busy'
		WHERE id = $1
	`, courier.ID)
	if err != nil {
		return nil, fmt.Errorf("mark courier busy: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO deliveries (
			id, order_id, event_id, restaurant_id, courier_id,
			status, delivery_address, total_amount, order_created_at,
			assigned_at, estimated_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, 'courier_assigned',
		        $6, $7, $8, $9, $10, $9, $9)
	`,
		deliveryID,
		orderID,
		eventID,
		restaurantID,
		courier.ID,
		e.DeliveryAddress,
		e.TotalAmount,
		e.OrderCreatedAt,
		now,
		estimatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert delivery: %w", err)
	}

	for _, restaurant := range e.Restaurants {
		for _, item := range restaurant.Items {
			_, err = tx.Exec(ctx, `
				INSERT INTO delivery_items (
					delivery_id, dish_id, name, quantity, unit_price
				)
				VALUES ($1, $2, $3, $4, $5)
			`,
				deliveryID,
				item.DishID,
				item.Name,
				item.Quantity,
				item.UnitPrice,
			)
			if err != nil {
				return nil, fmt.Errorf("insert delivery item: %w", err)
			}
		}
	}

	outgoing := event.CourierAssigned{
		EventID:             uuid.NewString(),
		EventType:           "delivery.courier_assigned",
		OccurredAt:          now,
		OrderID:             orderID.String(),
		DeliveryID:          deliveryID.String(),
		CourierID:           courier.ID.String(),
		CourierName:         courier.Name,
		DeliveryAddress:     e.DeliveryAddress,
		AssignedAt:          now,
		EstimatedDeliveryAt: estimatedAt,
		TotalAmount:         e.TotalAmount,
	}

	payload, err := json.Marshal(outgoing)
	if err != nil {
		return nil, fmt.Errorf("marshal courier assigned event: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (id, aggregate_id, event_type, payload)
		VALUES ($1, $2, $3, $4)
	`,
		outgoing.EventID,
		orderID,
		outgoing.EventType,
		payload,
	)
	if err != nil {
		return nil, fmt.Errorf("insert outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit delivery transaction: %w", err)
	}

	return &Delivery{
		ID:              deliveryID,
		OrderID:         orderID,
		EventID:         eventID,
		RestaurantID:    restaurantID,
		CourierID:       courier.ID,
		CourierName:     courier.Name,
		Status:          "courier_assigned",
		DeliveryAddress: e.DeliveryAddress,
		TotalAmount:     e.TotalAmount,
		OrderCreatedAt:  e.OrderCreatedAt,
		AssignedAt:      now,
		EstimatedAt:     estimatedAt,
	}, nil
}
