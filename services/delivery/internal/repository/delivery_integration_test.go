package repository

import (
	"context"
	"testing"
	"time"

	"github.com/VitalyMars/delivery_go/pkg/testdb"
	"github.com/VitalyMars/delivery_go/services/delivery/internal/event"
	"github.com/google/uuid"
)

func testEvent(created time.Time) event.OrderReadyForDelivery {
	return event.OrderReadyForDelivery{EventID: uuid.NewString(), EventType: "order.ready_for_delivery", OrderID: uuid.NewString(), PaymentStatus: "PAID", DeliveryAddress: "Address", OrderCreatedAt: created, TotalAmount: 100, Restaurants: []event.RestaurantOrder{{RestaurantID: uuid.NewString(), Items: []event.OrderItem{{DishID: uuid.NewString(), Name: "Dish", Quantity: 1, UnitPrice: 100}}}}}
}

func TestFIFOAssignmentCompletionAndRecovery(t *testing.T) {
	db := testdb.Open(t, "../../migrations")
	repo := NewDeliveryRepository(db)
	ctx := context.Background()
	if _, err := db.Exec(ctx, "UPDATE couriers SET status='inactive'"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "UPDATE couriers SET status='available' WHERE id=(SELECT id FROM couriers ORDER BY id LIMIT 1)"); err != nil {
		t.Fatal(err)
	}
	firstEvent := testEvent(time.Now().Add(-time.Minute))
	secondEvent := testEvent(time.Now())
	first, err := repo.CreateFromOrderEvent(ctx, firstEvent)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.CreateFromOrderEvent(ctx, secondEvent)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := repo.CreateFromOrderEvent(ctx, firstEvent)
	if err != nil || duplicate.ID != first.ID {
		t.Fatal(duplicate, err)
	}
	assigned, err := repo.AssignNextWaitingDelivery(ctx)
	if err != nil || !assigned {
		t.Fatal(assigned, err)
	}
	var status string
	var eta *time.Time
	if err := db.QueryRow(ctx, "SELECT status,estimated_at FROM deliveries WHERE id=$1", first.ID).Scan(&status, &eta); err != nil || status != "courier_assigned" || eta == nil {
		t.Fatal(status, eta, err)
	}
	if assigned, err = repo.AssignNextWaitingDelivery(ctx); err != nil || assigned {
		t.Fatal("courier double assigned", err)
	}
	if err = repo.CompleteDelivery(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if err = repo.CompleteDelivery(ctx, first.ID); err != nil {
		t.Fatal("completion not idempotent", err)
	}
	var count int
	if err = db.QueryRow(ctx, "SELECT count(*) FROM outbox_events WHERE event_type='delivery.completed'").Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	if assigned, err = repo.AssignNextWaitingDelivery(ctx); err != nil || !assigned {
		t.Fatal(assigned, err)
	}
	if err = db.QueryRow(ctx, "SELECT status FROM deliveries WHERE id=$1", second.ID).Scan(&status); err != nil || status != "courier_assigned" {
		t.Fatal(status, err)
	}
}
