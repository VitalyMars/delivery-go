package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Dish struct {
	ID           uuid.UUID
	RestaurantID uuid.UUID
	Name         string
	Description  string
	Price        int
	Available    bool
	CreatedAt    time.Time
}

type DishRepository struct {
	db *pgxpool.Pool
}

func NewDishRepository(db *pgxpool.Pool) *DishRepository {
	return &DishRepository{
		db: db,
	}
}

func (r *DishRepository) GetDishesByRestaurantID(ctx context.Context, id uuid.UUID) ([]Dish, error) {
	rows, err := r.db.Query(ctx,
		`
	SELECT id, restaurant_id, name, description, price, available, created_at
	FROM dishes
	WHERE restaurant_id = $1
	ORDER BY created_at
	`,
		id,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var dishes []Dish

	for rows.Next() {
		var dish Dish

		err := rows.Scan(
			&dish.ID,
			&dish.RestaurantID,
			&dish.Name,
			&dish.Description,
			&dish.Price,
			&dish.Available,
			&dish.CreatedAt,
		)

		if err != nil {
			return nil, err
		}

		dishes = append(dishes, dish)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return dishes, nil
}

func (r *DishRepository) GetDishByID(
	ctx context.Context,
	id uuid.UUID,
) (*Dish, error) {

	var dish Dish

	err := r.db.QueryRow(ctx,
		`
	SELECT id, restaurant_id, name, description, price, available, created_at
	FROM dishes
	WHERE id = $1
	ORDER BY created_at
	`,
		id,
	).Scan(
		&dish.ID,
		&dish.RestaurantID,
		&dish.Name,
		&dish.Description,
		&dish.Price,
		&dish.Available,
		&dish.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &dish, nil
}

func (r *DishRepository) SetDishAvailability(
	ctx context.Context,
	id uuid.UUID,
	available bool,
) error {
	_, err := r.db.Exec(
		ctx,
		`
		UPDATE dishes
		SET available = $1
		WHERE id = $2
		`, available, id,
	)

	return err
}
