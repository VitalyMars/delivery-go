package main

import (
	"context"
	"log"
	"time"

	"github.com/VitalyMars/delivery_go/services/order/internal/event"
	"github.com/VitalyMars/delivery_go/services/order/internal/kafka"
	"github.com/google/uuid"
)

func main() {
	producer, err := kafka.NewProducer([]string{
		"localhost:9092",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Close()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	now := time.Now().UTC()

	testEvent := event.OrderReadyForDeliveryEvent{
		EventID:         uuid.NewString(),
		EventType:       "order.ready_for_delivery",
		OccurredAt:      now,
		OrderCreatedAt:  now,
		OrderID:         uuid.NewString(),
		PaymentStatus:   "confirmed",
		DeliveryAddress: "Test address",
		Restaurants: []event.RestaurantOrder{
			{
				RestaurantID: "11111111-1111-4111-8111-111111111111",
				Items: []event.OrderItem{
					{
						DishID:    "22222222-2222-4222-8222-222222222222",
						Name:      "Test dish",
						Quantity:  2,
						UnitPrice: 5000,
					},
				},
			},
		},
		TotalAmount: 10000,
	}

	if err := producer.Publish(
		ctx,
		"order.ready_for_delivery",
		testEvent.OrderID,
		testEvent,
	); err != nil {
		log.Fatal(err)
	}

	log.Printf(
		"test event published: event_id=%s order_id=%s",
		testEvent.EventID,
		testEvent.OrderID,
	)
}
