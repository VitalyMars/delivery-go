package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/VitalyMars/delivery_go/pkg/kafkaconsumer"
	"github.com/VitalyMars/delivery_go/services/order/internal/repository"
	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Consumer struct {
	client *kgo.Client
	repo   *repository.OrderRepository
}

type CourierAssignedEvent struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	OrderID    string `json:"order_id"`
	DeliveryID string `json:"delivery_id"`
	CourierID  string `json:"courier_id"`
}

func NewConsumer(
	brokers []string,
	groupID string,
	topic string,
	repo *repository.OrderRepository,
) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topic, "delivery.completed"),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
		kgo.ConsumeStartOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("create Kafka consumer: %w", err)
	}

	return &Consumer{
		client: client,
		repo:   repo,
	}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	return kafkaconsumer.Run(ctx, c.client, c.processRecord)
}

func (c *Consumer) processRecord(
	ctx context.Context,
	record *kgo.Record,
) error {
	var e CourierAssignedEvent
	if err := json.Unmarshal(record.Value, &e); err != nil {
		return kafkaconsumer.Permanent(fmt.Errorf("decode delivery event: %w", err))
	}

	if e.EventType != "delivery.courier_assigned" && e.EventType != "delivery.completed" {
		return kafkaconsumer.Permanent(fmt.Errorf("unexpected event type %q", e.EventType))
	}

	if id, err := uuid.Parse(e.EventID); err != nil || id == uuid.Nil {
		return kafkaconsumer.Permanent(fmt.Errorf("invalid event ID"))
	}
	orderID, err := uuid.Parse(e.OrderID)
	if err != nil {
		return kafkaconsumer.Permanent(fmt.Errorf("invalid order ID %q: %w", e.OrderID, err))
	}

	target := "courier_assigned"
	if e.EventType == "delivery.completed" {
		target = "delivered"
	}
	if err := c.repo.ApplyDeliveryStatus(ctx, orderID, target); err != nil {
		return fmt.Errorf("mark courier assigned for order %s: %w", orderID, err)
	}

	log.Printf(
		"Order updated: order_id=%s delivery_id=%s event=%s",
		e.OrderID, e.DeliveryID, e.EventID,
	)

	return nil
}
