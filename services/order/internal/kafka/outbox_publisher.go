package kafka

import (
	"context"
	"log"
	"time"

	"github.com/VitalyMars/delivery_go/services/order/internal/repository"
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

	for {
		if err := p.PublishBatch(ctx); err != nil {
			log.Printf("outbox publisher error: %v", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (p *OutboxPublisher) PublishBatch(ctx context.Context) error {
	batchCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return p.repository.PublishBatch(batchCtx, p.producer.Publish)
}
