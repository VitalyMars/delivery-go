package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/VitalyMars/delivery_go/pkg/kafkaconsumer"
	"github.com/VitalyMars/delivery_go/services/notification/internal/service"
	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
)

type OrderReadyForDeliveryEvent struct {
	EventID         string    `json:"event_id"`
	UserID          *string   `json:"user_id,omitempty"`
	EventType       string    `json:"event_type"`
	OccurredAt      time.Time `json:"occurred_at"`
	OrderCreatedAt  time.Time `json:"order_created_at"`
	OrderID         string    `json:"order_id"`
	PaymentStatus   string    `json:"payment_status"`
	DeliveryAddress string    `json:"delivery_address"`
	TotalAmount     int64     `json:"total_amount"`
}

type UserServiceClient interface {
	GetEmail(ctx context.Context, userID string) (string, error)
}

type Consumer struct {
	client              *kgo.Client
	notificationService *service.NotificationService
	userServiceClient   UserServiceClient
}

func NewConsumer(
	brokers []string,
	groupID string,
	topic string,
	notificationService *service.NotificationService,
	userServiceClient UserServiceClient,
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
		client:              client,
		notificationService: notificationService,
		userServiceClient:   userServiceClient,
	}, nil
}

func (c *Consumer) Run(ctx context.Context) error {
	return kafkaconsumer.Run(ctx, c.client, func(ctx context.Context, record *kgo.Record) error {
		var event OrderReadyForDeliveryEvent
		if err := json.Unmarshal(record.Value, &event); err != nil {
			return kafkaconsumer.Permanent(err)
		}
		return c.processEvent(ctx, event)
	})
}

func validateEvent(event OrderReadyForDeliveryEvent) error {
	if id, err := uuid.Parse(event.EventID); err != nil || id == uuid.Nil {
		return kafkaconsumer.Permanent(errors.New("event_id is required"))
	}
	if id, err := uuid.Parse(event.OrderID); err != nil || id == uuid.Nil {
		return kafkaconsumer.Permanent(errors.New("order_id is required"))
	}
	return nil
}

func (c *Consumer) processEvent(
	ctx context.Context,
	event OrderReadyForDeliveryEvent,
) error {
	if err := validateEvent(event); err != nil {
		return kafkaconsumer.Permanent(err)
	}

	if event.EventType != "order.ready_for_delivery" {
		return kafkaconsumer.Permanent(errors.New("unsupported event type"))
	}

	if event.UserID == nil || *event.UserID == "" {
		log.Printf(
			"Skipping notification for guest order: order_id=%s",
			event.OrderID,
		)
		return nil
	}

	userID, err := uuid.Parse(*event.UserID)
	if err != nil {
		return kafkaconsumer.Permanent(errors.New("invalid user_id"))
	}

	orderID, err := uuid.Parse(event.OrderID)
	if err != nil {
		return kafkaconsumer.Permanent(errors.New("invalid order_id"))
	}

	email, err := c.userServiceClient.GetEmail(ctx, userID.String())
	if err != nil {
		return err
	}

	subject := "Заказ принят в доставку"
	body := fmt.Sprintf(
		"Ваш заказ %s принят в доставку. Адрес доставки: %s.",
		orderID,
		event.DeliveryAddress,
	)

	created, err := c.notificationService.Create(
		ctx,
		event.EventID,
		userID,
		orderID,
		email,
		subject,
		body,
	)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			return kafkaconsumer.Permanent(err)
		}
		return err
	}

	if !created {
		log.Printf(
			"Duplicate notification event ignored: event_id=%s order_id=%s",
			event.EventID,
			orderID,
		)
		return nil
	}

	log.Printf(
		"Notification created: event_id=%s order_id=%s",
		event.EventID,
		orderID,
	)

	return nil
}
