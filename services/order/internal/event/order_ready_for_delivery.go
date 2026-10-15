package event

import "time"

// OrderItem описывает блюдо внутри заказа.
type OrderItem struct {
	DishID    string `json:"dish_id"`
	Name      string `json:"name"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price"`
}

// RestaurantOrder содержит ресторан и заказанные в нём блюда.
type RestaurantOrder struct {
	RestaurantID string      `json:"restaurant_id"`
	Items        []OrderItem `json:"items"`
}

// OrderReadyForDeliveryEvent описывает событие передачи заказа в доставку.
type OrderReadyForDeliveryEvent struct {
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
