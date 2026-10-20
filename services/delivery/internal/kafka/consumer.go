package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/VitalyMars/delivery_go/pkg/kafkaconsumer"
	"github.com/VitalyMars/delivery_go/services/delivery/internal/event"
	"github.com/VitalyMars/delivery_go/services/delivery/internal/repository"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Consumer struct {
	client *kgo.Client
	repo   *repository.DeliveryRepository
}

func NewConsumer(
	brokers []string,
	groupID string,
	topic string,
	repo *repository.DeliveryRepository,
) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topic),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
		kgo.ConsumeStartOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, err
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
	var e event.OrderReadyForDelivery
	if err := json.Unmarshal(record.Value, &e); err != nil {
		return kafkaconsumer.Permanent(errors.New("invalid order event JSON: " + err.Error()))
	}

	if err := e.Validate(); err != nil {
		return kafkaconsumer.Permanent(err)
	}
	delivery, err := c.repo.CreateFromOrderEvent(ctx, e)
	if err != nil {
		return err
	}

	log.Printf(
		"Delivery processed: order_id=%s delivery_id=%s status=%s",
		delivery.OrderID,
		delivery.ID,
		delivery.Status,
	)

	return nil
}
