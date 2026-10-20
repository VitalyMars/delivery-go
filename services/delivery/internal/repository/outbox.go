package repository

import (
	"context"

	"github.com/VitalyMars/delivery_go/pkg/outbox"
	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxRepository struct {
	db *pgxpool.Pool
}

func NewOutboxRepository(db *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) PublishBatch(ctx context.Context, publish outbox.Publish) error {
	return outbox.Batch(ctx, r.db, publish)
}
