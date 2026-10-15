package config

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewDB(ctx context.Context) (*pgxpool.Pool, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://app:catalog@localhost:5433/user_db?sslmode=disable"
	}

	return pgxpool.New(ctx, databaseURL)
}
