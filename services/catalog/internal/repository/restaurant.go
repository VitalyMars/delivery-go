package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RestaurantRepository struct {
	db *pgxpool.Pool
}

type Restaurant struct {
	ID        uuid.UUID
	Name      string
	Address   string
	CreatedAt time.Time
}

func NewRestaurantRepository(db *pgxpool.Pool) *RestaurantRepository {
	return &RestaurantRepository{
		db: db,
	}
}

func (r *RestaurantRepository) GetRestaurants(ctx context.Context) ([]Restaurant, error) {

	rows, err := r.db.Query(ctx,
		`
	SELECT id, name, address, created_at
	FROM restaurants
	ORDER BY created_at
	`)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var restaurants []Restaurant

	for rows.Next() {
		var restaurant Restaurant

		err := rows.Scan(
			&restaurant.ID,
			&restaurant.Name,
			&restaurant.Address,
			&restaurant.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		restaurants = append(restaurants, restaurant)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return restaurants, nil
}

func (r *RestaurantRepository) GetRestaurantByID(
	ctx context.Context,
	id uuid.UUID,
) (*Restaurant, error) {
	var restaurant Restaurant

	err := r.db.QueryRow(ctx,
		`
	SELECT id, name, address, created_at
	FROM restaurants
	WHERE id = $1
	`,
		id,
	).Scan(
		&restaurant.ID,
		&restaurant.Name,
		&restaurant.Address,
		&restaurant.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &restaurant, nil
}
