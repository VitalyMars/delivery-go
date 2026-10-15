package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/VitalyMars/delivery_go/services/order/internal/event"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxEvent struct {
	ID          uuid.UUID
	AggregateID uuid.UUID
	EventType   string
	Payload     []byte
}

type OutboxRepository struct {
	db *pgxpool.Pool
}

func NewOutboxRepository(db *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) GetUnpublished(
	ctx context.Context,
	limit int,
) ([]OutboxEvent, error) {
	rows, err := r.db.Query(
		ctx,
		`
		SELECT id, aggregate_id, event_type, payload
		FROM outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at, id
		LIMIT $1
		`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get unpublished outbox events: %w", err)
	}
	defer rows.Close()

	events := make([]OutboxEvent, 0)

	for rows.Next() {
		var event OutboxEvent

		if err := rows.Scan(
			&event.ID,
			&event.AggregateID,
			&event.EventType,
			&event.Payload,
		); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}

	return events, nil
}

func (r *OutboxRepository) MarkPublished(
	ctx context.Context,
	eventID uuid.UUID,
) error {
	result, err := r.db.Exec(
		ctx,
		`
		UPDATE outbox_events
		SET published_at = now()
		WHERE id = $1 AND published_at IS NULL
		`,
		eventID,
	)
	if err != nil {
		return fmt.Errorf("mark outbox event as published: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("outbox event %s was not marked as published", eventID)
	}

	return nil
}

func (r *OutboxRepository) CreateTx(
	ctx context.Context,
	tx pgx.Tx,
	event OutboxEvent,
) error {
	_, err := tx.Exec(ctx, `
        INSERT INTO outbox_events (
            id,
            aggregate_id,
            event_type,
            payload
        )
        VALUES ($1, $2, $3, $4)
    `,
		event.ID,
		event.AggregateID,
		event.EventType,
		event.Payload,
	)

	return err
}

func createPaymentEvent(
	event event.OrderReadyForDeliveryEvent,
) (OutboxEvent, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return OutboxEvent{}, err
	}

	eventID, err := uuid.Parse(event.EventID)
	if err != nil {
		return OutboxEvent{}, fmt.Errorf("parse event ID: %w", err)
	}

	orderID, err := uuid.Parse(event.OrderID)
	if err != nil {
		return OutboxEvent{}, fmt.Errorf("parse order ID: %w", err)
	}

	return OutboxEvent{
		ID:          eventID,
		AggregateID: orderID,
		EventType:   "order.ready_for_delivery",
		Payload:     payload,
	}, nil
}
