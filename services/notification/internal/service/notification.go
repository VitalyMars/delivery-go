package service

import (
	"context"
	"errors"
	"strings"

	"github.com/VitalyMars/delivery_go/services/notification/internal/repository"
	"github.com/google/uuid"
)

var ErrInvalidInput = errors.New("invalid notification input")

type NotificationRepository interface {
	Create(ctx context.Context, n *repository.Notification) error
	MarkSent(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, attempts int, lastError string) error
	GetByID(ctx context.Context, id uuid.UUID) (*repository.Notification, error)
}

type NotificationService struct {
	repository NotificationRepository
}

func NewNotificationService(repository NotificationRepository) *NotificationService {
	return &NotificationService{
		repository: repository,
	}
}

func (s *NotificationService) Create(
	ctx context.Context,
	userID uuid.UUID,
	orderID uuid.UUID,
	email string,
	subject string,
	body string,
) (*repository.Notification, error) {
	email = strings.TrimSpace(email)
	subject = strings.TrimSpace(subject)

	if userID == uuid.Nil ||
		orderID == uuid.Nil ||
		email == "" ||
		subject == "" ||
		strings.TrimSpace(body) == "" {
		return nil, ErrInvalidInput
	}

	notification := &repository.Notification{
		ID:             uuid.New(),
		UserID:         userID,
		OrderID:        orderID,
		RecipientEmail: email,
		Subject:        subject,
		Body:           body,
		Status:         "pending",
		Attempts:       0,
	}

	if err := s.repository.Create(ctx, notification); err != nil {
		return nil, err
	}

	return notification, nil
}

func (s *NotificationService) GetByID(
	ctx context.Context,
	id uuid.UUID,
) (*repository.Notification, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}

	return s.repository.GetByID(ctx, id)
}
