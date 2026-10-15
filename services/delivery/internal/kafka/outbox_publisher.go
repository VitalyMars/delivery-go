package kafka

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/VitalyMars/delivery_go/services/delivery/internal/repository"
)

const (
	outboxBatchSize = 100
	outboxPollDelay = time.Second
)

type OutboxPublisher struct {
	repository *repository.OutboxRepository
	producer   *Producer
}

func NewOutboxPublisher(
	repo *repository.OutboxRepository,
	producer *Producer,
) *OutboxPublisher {
	return &OutboxPublisher{
		repository: repo,
		producer:   producer,
	}
}

func (p *OutboxPublisher) Run(ctx context.Context) {
	ticker := time.NewTicker(outboxPollDelay)
	defer ticker.Stop()

	log.Println("Delivery outbox publisher started")

	for {
		if err := p.PublishBatch(ctx); err != nil && ctx.Err() == nil {
			log.Printf("delivery outbox publisher error: %v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *OutboxPublisher) PublishBatch(ctx context.Context) error {
	events, err := p.repository.GetUnpublished(ctx, outboxBatchSize)
	if err != nil {
		return err
	}

	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := p.producer.Publish(
			ctx,
			event.EventType,
			event.AggregateID.String(),
			json.RawMessage(event.Payload),
		); err != nil {
			log.Printf("failed to publish outbox event %s: %v", event.ID, err)
			continue
		}

		if err := p.repository.MarkPublished(ctx, event.ID); err != nil {
			log.Printf(
				"event %s sent to Kafka but could not be marked published: %v",
				event.ID,
				err,
			)
			continue
		}

		log.Printf(
			"delivery outbox event published: id=%s type=%s",
			event.ID,
			event.EventType,
		)
	}

	return nil
}
