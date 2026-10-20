package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/VitalyMars/delivery_go/services/delivery/internal/event"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DeliveryRepository struct{ db *pgxpool.Pool }

func NewDeliveryRepository(db *pgxpool.Pool) *DeliveryRepository { return &DeliveryRepository{db} }
func (r *DeliveryRepository) CreateFromOrderEvent(ctx context.Context, e event.OrderReadyForDelivery) (*Delivery, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	orderID, _ := uuid.Parse(e.OrderID)
	eventID, _ := uuid.Parse(e.EventID)
	restaurantID, _ := uuid.Parse(e.Restaurants[0].RestaurantID)
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(728104)"); err != nil {
		return nil, err
	}
	var existing Delivery
	err = tx.QueryRow(ctx, "SELECT id,order_id,event_id,restaurant_id,status,delivery_address,total_amount,order_created_at FROM deliveries WHERE order_id=$1", orderID).Scan(&existing.ID, &existing.OrderID, &existing.EventID, &existing.RestaurantID, &existing.Status, &existing.DeliveryAddress, &existing.TotalAmount, &existing.OrderCreatedAt)
	if err == nil {
		return &existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	id := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO deliveries(id,order_id,event_id,restaurant_id,status,delivery_address,total_amount,order_created_at) VALUES ($1,$2,$3,$4,'waiting_for_courier',$5,$6,$7)`, id, orderID, eventID, restaurantID, e.DeliveryAddress, e.TotalAmount, e.OrderCreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert delivery: %w", err)
	}
	for _, item := range e.Restaurants[0].Items {
		if _, err = tx.Exec(ctx, "INSERT INTO delivery_items(delivery_id,dish_id,name,quantity,unit_price) VALUES ($1,$2,$3,$4,$5)", id, item.DishID, item.Name, item.Quantity, item.UnitPrice); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &Delivery{ID: id, OrderID: orderID, EventID: eventID, RestaurantID: restaurantID, Status: "waiting_for_courier", DeliveryAddress: e.DeliveryAddress, TotalAmount: e.TotalAmount, OrderCreatedAt: e.OrderCreatedAt}, nil
}
