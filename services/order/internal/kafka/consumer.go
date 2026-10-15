package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

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
		kgo.ConsumeTopics(topic),
		kgo.ConsumeStartOffset(kgo.NewOffset().AtEnd()),
	)
	if err != nil {
		return nil, fmt.Errorf("create Kafka consumer: %w", err)
	}

	return &Consumer{
		client: client,
		repo:   repo,
	}, nil
}

func (c *Consumer) Run(ctx context.Context) {
	defer c.client.Close()
	log.Println("Order Kafka consumer started")

	for {
		if ctx.Err() != nil {
			return
		}

		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return
		}

		fetches.EachError(func(topic string, partition int32, err error) {
			log.Printf(
				"Kafka fetch error: topic=%s partition=%d: %v",
				topic, partition, err,
			)
		})

		fetches.EachPartition(func(p kgo.FetchTopicPartition) {
			if p.Err != nil {
				return
			}

			for _, record := range p.Records {
				if ctx.Err() != nil {
					return
				}

				if err := c.processRecord(ctx, record); err != nil {
					log.Printf(
						"Failed to process event at %s[%d] offset %d: %v",
						record.Topic, record.Partition, record.Offset, err,
					)
					return
				}

				if err := c.client.CommitRecords(ctx, record); err != nil {
					log.Printf(
						"Failed to commit Kafka offset at %s[%d] offset %d: %v",
						record.Topic, record.Partition, record.Offset, err,
					)
					return
				}
			}
		})
	}
}

func (c *Consumer) processRecord(
	ctx context.Context,
	record *kgo.Record,
) error {
	var e CourierAssignedEvent
	if err := json.Unmarshal(record.Value, &e); err != nil {
		return fmt.Errorf("decode courier-assigned event: %w", err)
	}

	if e.EventType != "delivery.courier_assigned" {
		return fmt.Errorf("unexpected event type %q", e.EventType)
	}

	orderID, err := uuid.Parse(e.OrderID)
	if err != nil {
		return fmt.Errorf("invalid order ID %q: %w", e.OrderID, err)
	}

	if err := c.repo.MarkCourierAssigned(ctx, orderID); err != nil {
		return fmt.Errorf("mark courier assigned for order %s: %w", orderID, err)
	}

	log.Printf(
		"Order updated: order_id=%s delivery_id=%s event=%s",
		e.OrderID, e.DeliveryID, e.EventID,
	)

	return nil
}
