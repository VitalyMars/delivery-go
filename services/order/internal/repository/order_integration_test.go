package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/VitalyMars/delivery_go/pkg/testdb"
	"github.com/google/uuid"
)

func TestOrderPaymentAndOutOfOrderDeliveryEvents(t *testing.T) {
	db := testdb.Open(t, "../../migrations")
	ctx := context.Background()
	repo := NewOrderRepository(db, NewOutboxRepository(db))
	order := &Order{ID: uuid.New(), RestaurantID: uuid.New(), Token: uuid.New(), CustomerName: "Name", CustomerPhone: "123", DeliveryAddress: "Address", Status: "created", Total: 100, IdempotencyKey: uuid.NewString(), RequestHash: "hash"}
	if err := repo.CreateOrder(ctx, order, []OrderItem{{ID: uuid.New(), OrderID: order.ID, DishID: uuid.New(), Name: "Dish", Quantity: 1, Price: 100}}); err != nil {
		t.Fatal(err)
	}
	for _, get := range []func() (*Order, error){func() (*Order, error) { return repo.GetOrderByID(ctx, order.ID) }, func() (*Order, error) { return repo.GetOrderByIDAndToken(ctx, order.ID, order.Token) }, func() (*Order, error) { return repo.GetOrderByIdempotencyKey(ctx, order.IdempotencyKey) }} {
		got, err := get()
		if err != nil || got.ID != order.ID || got.RequestHash != "hash" {
			t.Fatalf("read order: %+v %v", got, err)
		}
	}
	if err := repo.PayForTheOrder(ctx, order.ID, uuid.New()); !errors.Is(err, ErrInvalidOrderToken) {
		t.Fatal(err)
	}
	if err := repo.PayForTheOrder(ctx, order.ID, order.Token); err != nil {
		t.Fatal(err)
	}
	if err := repo.PayForTheOrder(ctx, order.ID, order.Token); !errors.Is(err, ErrOrderAlreadyPaid) {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(ctx, "SELECT count(*) FROM outbox_events").Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	paid, err := repo.GetOrderByID(ctx, order.ID)
	if err != nil || paid.Status != "ready_for_delivery" || paid.PaymentStatus != "PAID" {
		t.Fatal(paid, err)
	}
	// Separate topics do not guarantee assignment arrives before completion.
	if err := repo.ApplyDeliveryStatus(ctx, order.ID, "delivered"); err != nil {
		t.Fatal(err)
	}
	if err := repo.ApplyDeliveryStatus(ctx, order.ID, "courier_assigned"); err != nil {
		t.Fatal(err)
	}
	final, err := repo.GetOrderByID(ctx, order.ID)
	if err != nil || final.Status != "delivered" {
		t.Fatal(final, err)
	}
}
