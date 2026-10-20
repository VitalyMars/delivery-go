package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/VitalyMars/delivery_go/pkg/outbox"
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

func (r *OutboxRepository) PublishBatch(ctx context.Context, publish outbox.Publish) error {
	return outbox.Batch(ctx, r.db, publish)
}
