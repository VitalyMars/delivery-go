package service

import (
	"context"
	"errors"
	"math"
	"testing"

	catalogv1 "github.com/VitalyMars/delivery_go/gen/catalog/v1"
	"github.com/VitalyMars/delivery_go/services/order/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type memoryOrders struct {
	OrderRepository
	orders map[string]*repository.Order
}

func (r *memoryOrders) GetOrderByIdempotencyKey(_ context.Context, key string) (*repository.Order, error) {
	if o := r.orders[key]; o != nil {
		return o, nil
	}
	return nil, pgx.ErrNoRows
}
func (r *memoryOrders) CreateOrder(_ context.Context, o *repository.Order, _ []repository.OrderItem) error {
	if r.orders[o.IdempotencyKey] != nil {
		return repository.ErrIdempotencyKeyConflict
	}
	r.orders[o.IdempotencyKey] = o
	return nil
}

type fakeCatalog struct {
	calls int
	fail  bool
	price int32
}

func (c *fakeCatalog) ValidateItems(_ context.Context, restaurant string, items []*catalogv1.OrderItem) (*catalogv1.ValidateItemsResponse, error) {
	c.calls++
	if c.fail {
		return nil, errors.New("catalog unavailable")
	}
	return &catalogv1.ValidateItemsResponse{Valid: true, Items: []*catalogv1.ValidatedItem{{ItemId: items[0].DishId, RestaurantId: restaurant, RequestedQuantity: items[0].Quantity, Available: true, ActualPrice: c.price, Name: "Dish"}}}, nil
}
func validInput() CreateOrderInput {
	return CreateOrderInput{ClientScope: "guest:" + uuid.NewString(), CustomerName: "Name", CustomerPhone: "123", DeliveryAddress: "Address", IdempotencyKey: uuid.NewString(), RestaurantID: uuid.NewString(), Items: []*catalogv1.OrderItem{{DishId: uuid.NewString(), Quantity: 2}}}
}

func TestIdempotencyIsScopedAndChecksPayloadBeforeCatalog(t *testing.T) {
	repo := &memoryOrders{orders: map[string]*repository.Order{}}
	catalog := &fakeCatalog{price: 100}
	service := NewOrderService(repo, catalog)
	input := validInput()
	first, err := service.CreateOrder(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	catalog.fail = true
	second, err := service.CreateOrder(context.Background(), input)
	if err != nil || second.ID != first.ID || catalog.calls != 1 {
		t.Fatalf("replay consulted catalog: %v calls=%d", err, catalog.calls)
	}
	changed := input
	changed.DeliveryAddress = "Another address"
	if _, err = service.CreateOrder(context.Background(), changed); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatal(err)
	}
	catalog.fail = false
	another := input
	another.ClientScope = "guest:" + uuid.NewString()
	third, err := service.CreateOrder(context.Background(), another)
	if err != nil || third.ID == first.ID || third.Token == first.Token {
		t.Fatal("caller received another caller's order", err)
	}
	input.IdempotencyKey = ""
	if _, err = service.CreateOrder(context.Background(), input); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("accepted empty key", err)
	}
}

func TestRejectAmountOverflow(t *testing.T) {
	repo := &memoryOrders{orders: map[string]*repository.Order{}}
	service := NewOrderService(repo, &fakeCatalog{price: math.MaxInt32})
	if _, err := service.CreateOrder(context.Background(), validInput()); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("accepted overflow", err)
	}
	if len(repo.orders) != 0 {
		t.Fatal("persisted invalid amount")
	}
}
