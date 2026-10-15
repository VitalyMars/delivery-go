package repository

import (
	"time"

	"github.com/google/uuid"
)

type Delivery struct {
	ID              uuid.UUID
	OrderID         uuid.UUID
	EventID         uuid.UUID
	RestaurantID    uuid.UUID
	CourierID       uuid.UUID
	CourierName     string
	Status          string
	DeliveryAddress string
	TotalAmount     int64
	OrderCreatedAt  time.Time
	AssignedAt      time.Time
	EstimatedAt     time.Time
}

type DeliveryItem struct {
	ID         uuid.UUID
	DeliveryID uuid.UUID
	DishID     uuid.UUID
	Name       string
	Quantity   int
	UnitPrice  int64
}

type Courier struct {
	ID     uuid.UUID
	Name   string
	Phone  string
	Status string
}
