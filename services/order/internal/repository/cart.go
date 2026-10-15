package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CartItem struct {
	ID        uuid.UUID
	CartToken uuid.UUID
	DishID    uuid.UUID
	Quantity  int
	Price     int
}

type CartRepository struct {
	db *pgxpool.Pool
}

func NewCartRepository(db *pgxpool.Pool) *CartRepository {
	return &CartRepository{
		db: db,
	}
}

func (r *CartRepository) AddItem(
	ctx context.Context,
	item *CartItem,
) error {
	_, err := r.db.Exec(
		ctx,
		`
		INSERT INTO cart_items (
			id,
			cart_token,
			dish_id,
			quantity,
			price
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			$5
		)
		`,
		item.ID,
		item.CartToken,
		item.DishID,
		item.Quantity,
		item.Price,
	)

	return err
}

func (r *CartRepository) GetItems(
	ctx context.Context,
	cartToken uuid.UUID,
) ([]CartItem, error) {

	rows, err := r.db.Query(
		ctx,
		`
		SELECT 
		id, cart_token, dish_id, quantity, price
		FROM cart_items
		WHERE cart_token = $1
		ORDER BY created_at
		`,
		cartToken,
	)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var items []CartItem

	for rows.Next() {
		var item CartItem

		if err := rows.Scan(
			&item.ID,
			&item.CartToken,
			&item.DishID,
			&item.Quantity,
			&item.Price,
		); err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *CartRepository) UpdateItemQuantity(
	ctx context.Context,
	cartToken uuid.UUID,
	itemID uuid.UUID,
	quantity int,
) ([]CartItem, bool, error) {

	commandTag, err := r.db.Exec(
		ctx,
		`
		UPDATE cart_items
		SET 
			quantity = $1,
			updated_at = now()
		WHERE id = $2
			AND cart_token = $3
		`,
		quantity,
		itemID,
		cartToken,
	)

	if err != nil {
		return nil, false, err
	}

	if commandTag.RowsAffected() == 0 {
		return nil, false, nil
	}

	items, err := r.GetItems(ctx, cartToken)
	if err != nil {
		return nil, false, err
	}

	return items, true, nil
}

func (r *CartRepository) DeleteItem(
	ctx context.Context,
	cartToken uuid.UUID,
	itemID uuid.UUID,
) ([]CartItem, bool, error) {

	commadTag, err := r.db.Exec(
		ctx,
		`
		DELETE FROM cart_items
		WHERE id = $1
			AND cart_token = $2
		`,
		itemID,
		cartToken,
	)

	if err != nil {
		return nil, false, err
	}

	if commadTag.RowsAffected() == 0 {
		return nil, false, nil
	}

	items, err := r.GetItems(ctx, cartToken)

	if err != nil {
		return nil, false, err
	}

	return items, true, nil
}
