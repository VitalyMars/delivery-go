package service

import (
	"context"
	"errors"
	"log"
	"net/mail"
	"strings"
	"time"

	"github.com/VitalyMars/delivery_go/services/notification/internal/repository"
	"github.com/google/uuid"
)

var ErrInvalidInput = errors.New("invalid notification input")

type NotificationRepository interface {
	Create(context.Context, *repository.Notification) (bool, error)
	GetByID(context.Context, uuid.UUID) (*repository.Notification, error)
	DeliverNext(context.Context, func(context.Context, string, string, string) error) (bool, error)
}
type EmailSender interface {
	Send(context.Context, string, string, string) error
}
type NotificationService struct {
	repository  NotificationRepository
	emailSender EmailSender
}

func NewNotificationService(repo NotificationRepository, sender EmailSender) *NotificationService {
	return &NotificationService{repo, sender}
}

// Create persists a task; delivery is performed independently by Run.
func (s *NotificationService) Create(ctx context.Context, eventID string, userID, orderID uuid.UUID, email, subject, body string) (bool, error) {
	email, subject, eventID = strings.TrimSpace(email), strings.TrimSpace(subject), strings.TrimSpace(eventID)
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 || len(subject) > 255 || strings.ContainsAny(subject, "\r\n") || eventID == "" || len(eventID) > 255 || userID == uuid.Nil || orderID == uuid.Nil || subject == "" || strings.TrimSpace(body) == "" {
		return false, ErrInvalidInput
	}
	return s.repository.Create(ctx, &repository.Notification{ID: uuid.New(), EventID: eventID, UserID: userID, OrderID: orderID, RecipientEmail: email, Subject: subject, Body: body, Status: "pending"})
}
func (s *NotificationService) GetByID(ctx context.Context, id uuid.UUID) (*repository.Notification, error) {
	if id == uuid.Nil {
		return nil, ErrInvalidInput
	}
	return s.repository.GetByID(ctx, id)
}
func (s *NotificationService) ProcessNext(ctx context.Context) (bool, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return s.repository.DeliverNext(attemptCtx, s.emailSender.Send)
}
func (s *NotificationService) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		processed, err := s.ProcessNext(ctx)
		if err != nil && ctx.Err() == nil {
			log.Printf("notification worker: %v", err)
		}
		if processed && err == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
