package event

import "time"

type CourierAssigned struct {
	EventID             string    `json:"event_id"`
	EventType           string    `json:"event_type"`
	OccurredAt          time.Time `json:"occurred_at"`
	OrderID             string    `json:"order_id"`
	DeliveryID          string    `json:"delivery_id"`
	CourierID           string    `json:"courier_id"`
	CourierName         string    `json:"courier_name"`
	DeliveryAddress     string    `json:"delivery_address"`
	AssignedAt          time.Time `json:"assigned_at"`
	EstimatedDeliveryAt time.Time `json:"estimated_delivery_at"`
	TotalAmount         int64     `json:"total_amount"`
}
