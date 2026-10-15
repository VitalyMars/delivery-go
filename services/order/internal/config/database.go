package config

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewDB(ctx context.Context) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, "postgres://app:catalog@localhost:5433/order_db?sslmode=disable")
}
