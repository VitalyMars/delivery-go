package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/VitalyMars/delivery_go/pkg/testdb"
	"github.com/google/uuid"
)

func TestDurableRetryAndDeduplication(t *testing.T) {
	db := testdb.Open(t, "../../migrations")
	repo := NewNotificationRepository(db)
	ctx := context.Background()
	notification := &Notification{ID: uuid.New(), EventID: uuid.NewString(), UserID: uuid.New(), OrderID: uuid.New(), RecipientEmail: "user@example.com", Subject: "Order", Body: "Body", Status: "pending"}
	if created, err := repo.Create(ctx, notification); err != nil || !created {
		t.Fatal(created, err)
	}
	if created, err := repo.Create(ctx, notification); err != nil || created {
		t.Fatal("duplicate created", err)
	}
	calls := 0
	fail := func(context.Context, string, string, string) error { calls++; return errors.New("SMTP unavailable") }
	if processed, err := repo.DeliverNext(ctx, fail); err != nil || !processed {
		t.Fatal(processed, err)
	}
	saved, err := repo.GetByID(ctx, notification.ID)
	if err != nil || saved.Status != "failed" || saved.Attempts != 1 {
		t.Fatal(saved, err)
	}
	if processed, err := repo.DeliverNext(ctx, fail); err != nil || processed || calls != 1 {
		t.Fatal("retry ignored schedule", processed, err)
	}
	if _, err = db.Exec(ctx, "UPDATE notifications SET next_attempt_at=now() WHERE id=$1", notification.ID); err != nil {
		t.Fatal(err)
	}
	// A new repository instance models service restart: the task remains in PostgreSQL.
	repo = NewNotificationRepository(db)
	if processed, err := repo.DeliverNext(ctx, func(context.Context, string, string, string) error { calls++; return nil }); err != nil || !processed {
		t.Fatal(processed, err)
	}
	saved, err = repo.GetByID(ctx, notification.ID)
	if err != nil || saved.Status != "sent" || saved.Attempts != 2 || saved.SentAt == nil {
		t.Fatal(saved, err)
	}
	if processed, err := repo.DeliverNext(ctx, fail); err != nil || processed || calls != 2 {
		t.Fatal("sent twice", err)
	}
}
