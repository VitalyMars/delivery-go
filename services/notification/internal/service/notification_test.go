package service

import (
	"context"
	"testing"

	"github.com/VitalyMars/delivery_go/services/notification/internal/repository"
	"github.com/google/uuid"
)

type fakeRepository struct {
	NotificationRepository
	created map[string]*repository.Notification
}

func (r *fakeRepository) Create(_ context.Context, n *repository.Notification) (bool, error) {
	if r.created[n.EventID] != nil {
		return false, nil
	}
	r.created[n.EventID] = n
	return true, nil
}

type forbiddenSender struct{}

func (forbiddenSender) Send(context.Context, string, string, string) error {
	panic("consumer attempted synchronous SMTP")
}

func TestCreatePersistsWithoutSendingAndDeduplicates(t *testing.T) {
	repo := &fakeRepository{created: map[string]*repository.Notification{}}
	service := NewNotificationService(repo, forbiddenSender{})
	eventID := uuid.NewString()
	userID, orderID := uuid.New(), uuid.New()
	created, err := service.Create(context.Background(), eventID, userID, orderID, "user@example.com", "Subject", "Body")
	if err != nil || !created {
		t.Fatal(created, err)
	}
	created, err = service.Create(context.Background(), eventID, userID, orderID, "user@example.com", "Subject", "Body")
	if err != nil || created || len(repo.created) != 1 {
		t.Fatal(created, err)
	}
	if repo.created[eventID].Status != "pending" {
		t.Fatal("task wasn't queued")
	}
}
