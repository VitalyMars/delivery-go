package main

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/VitalyMars/delivery_go/pkg/transport"
	"github.com/VitalyMars/delivery_go/services/order/internal/config"
	"github.com/VitalyMars/delivery_go/services/order/internal/kafka"
	"github.com/google/uuid"
)

// Replay an existing paid order event with its original event_id.
func main() {
	orderID, err := uuid.Parse(transport.Env("ORDER_ID", ""))
	if err != nil || orderID == uuid.Nil {
		log.Fatal("ORDER_ID must be an existing paid order UUID")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := config.NewDB(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	var payload []byte
	err = db.QueryRow(ctx, "SELECT payload FROM outbox_events WHERE aggregate_id=$1 AND event_type='order.ready_for_delivery' ORDER BY created_at DESC,id DESC LIMIT 1", orderID).Scan(&payload)
	if err != nil {
		log.Fatalf("load original outbox event: %v", err)
	}
	producer, err := kafka.NewProducer(transport.Brokers())
	if err != nil {
		log.Fatal(err)
	}
	defer producer.Close()
	if err = producer.Publish(ctx, "order.ready_for_delivery", orderID.String(), json.RawMessage(payload)); err != nil {
		log.Fatal(err)
	}
	log.Printf("original event replayed: order_id=%s", orderID)
}
