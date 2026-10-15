package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"log"

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
		kgo.ConsumeStartOffset(kgo.NewOffset().AtEnd()),
	)
	if err != nil {
		return nil, err
	}

	return &Consumer{
		client: client,
		repo:   repo,
	}, nil
}

func (c *Consumer) Run(ctx context.Context) {
	defer c.client.Close()

	log.Println("Kafka consumer started")

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

		// Each partition is processed independently.
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
						record.Topic,
						record.Partition,
						record.Offset,
						err,
					)

					// Do not process or commit later records
					// from this partition in this batch.
					return
				}

				if err := c.client.CommitRecords(ctx, record); err != nil {
					log.Printf(
						"Failed to commit Kafka offset at %s[%d] offset %d: %v",
						record.Topic,
						record.Partition,
						record.Offset,
						err,
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
	var e event.OrderReadyForDelivery
	if err := json.Unmarshal(record.Value, &e); err != nil {
		return errors.New("invalid order event JSON: " + err.Error())
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
