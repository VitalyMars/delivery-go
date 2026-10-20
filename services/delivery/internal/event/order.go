package event

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

type OrderItem struct {
	DishID    string `json:"dish_id"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
}

type RestaurantOrder struct {
	RestaurantID string      `json:"restaurant_id"`
	Items        []OrderItem `json:"items"`
}

type OrderReadyForDelivery struct {
	EventID         string            `json:"event_id"`
	UserID          *string           `json:"user_id,omitempty"`
	EventType       string            `json:"event_type"`
	OccurredAt      time.Time         `json:"occurred_at"`
	OrderCreatedAt  time.Time         `json:"order_created_at"`
	OrderID         string            `json:"order_id"`
	PaymentStatus   string            `json:"payment_status"`
	DeliveryAddress string            `json:"delivery_address"`
	Restaurants     []RestaurantOrder `json:"restaurants"`
	TotalAmount     int64             `json:"total_amount"`
}

func (e OrderReadyForDelivery) Validate() error {
	for _, id := range []string{e.OrderID, e.EventID} {
		parsed, err := uuid.Parse(id)
		if err != nil || parsed == uuid.Nil {
			return errors.New("invalid event or order ID")
		}
	}
	if e.EventType != "order.ready_for_delivery" || e.PaymentStatus != "PAID" || len(e.Restaurants) != 1 || e.OrderCreatedAt.IsZero() || strings.TrimSpace(e.DeliveryAddress) == "" || e.TotalAmount < 0 {
		return errors.New("invalid order event")
	}
	restaurant := e.Restaurants[0]
	if id, err := uuid.Parse(restaurant.RestaurantID); err != nil || id == uuid.Nil {
		return errors.New("invalid restaurant")
	}
	if len(restaurant.Items) == 0 || len(restaurant.Items) > 100 {
		return errors.New("invalid item count")
	}
	var total int64
	for _, item := range restaurant.Items {
		if id, err := uuid.Parse(item.DishID); err != nil || id == uuid.Nil {
			return errors.New("invalid dish ID")
		}
		if item.Quantity <= 0 || item.Quantity > 1000 || item.UnitPrice < 0 || item.UnitPrice > math.MaxInt64/int64(item.Quantity) {
			return errors.New("invalid quantity or price")
		}
		amount := item.UnitPrice * int64(item.Quantity)
		if total > math.MaxInt64-amount {
			return errors.New("amount overflow")
		}
		total += amount
	}
	if total != e.TotalAmount {
		return errors.New("total does not match items")
	}
	return nil
}
