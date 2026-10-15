package service

import (
	"context"
	"errors"
	"log"

	catalogv1 "github.com/VitalyMars/delivery_go/gen/catalog/v1"
	"github.com/VitalyMars/delivery_go/services/order/internal/catalog"
	"github.com/VitalyMars/delivery_go/services/order/internal/repository"
	"github.com/google/uuid"
)

type OrderService struct {
	repository    *repository.OrderRepository
	catalogClient *catalog.Client
}

func NewOrderService(
	repository *repository.OrderRepository,
	catalogClient *catalog.Client,
) *OrderService {
	return &OrderService{
		repository:    repository,
		catalogClient: catalogClient,
	}
}

type CreateOrderInput struct {
	UserID          *uuid.UUID
	CustomerName    string
	CustomerPhone   string
	DeliveryAddress string
	Comment         string
	IdempotencyKey  string
	RestaurantID    string
	Items           []*catalogv1.OrderItem
}

var ErrInvalidOrderToken = repository.ErrInvalidOrderToken
var ErrOrderCannotBePaid = repository.ErrOrderCannotBePaid

func (s *OrderService) CreateOrder(
	ctx context.Context,
	input CreateOrderInput,
) (*repository.Order, error) {

	log.Printf("sending restaurant_id=%q", input.RestaurantID)

	for _, item := range input.Items {
		log.Printf("sending dish_id=%q quantity=%d", item.GetDishId(), item.GetQuantity())
	}

	response, err := s.catalogClient.ValidateItems(
		ctx,
		input.RestaurantID,
		input.Items,
	)
	if err != nil {
		return nil, err
	}

	if !response.Valid {
		return nil, errors.New("items are not valid")
	}

	restaurantID, err := uuid.Parse(input.RestaurantID)
	if err != nil {
		return nil, errors.New("invalid restaurant id")
	}

	order := &repository.Order{
		ID:              uuid.New(),
		UserID:          input.UserID,
		RestaurantID:    restaurantID,
		Token:           uuid.New(),
		CustomerName:    input.CustomerName,
		CustomerPhone:   input.CustomerPhone,
		DeliveryAddress: input.DeliveryAddress,
		Comment:         input.Comment,
		Status:          "created",
		IdempotencyKey:  input.IdempotencyKey,
	}

	orderItems := make([]repository.OrderItem, 0, len(response.Items))

	for _, item := range response.Items {
		log.Printf("catalog returned item_id=%q", item.GetItemId())

		dishID, err := uuid.Parse(item.GetItemId())
		if err != nil {
			return nil, errors.New("invalid dish id from catalog")
		}

		orderItem := repository.OrderItem{
			ID:       uuid.New(),
			OrderID:  order.ID,
			DishID:   dishID,
			Name:     item.GetName(),
			Quantity: int(item.GetRequestedQuantity()),
			Price:    int(item.GetActualPrice()),
		}

		order.Total += orderItem.Price * orderItem.Quantity

		orderItems = append(orderItems, orderItem)
	}

	if len(orderItems) == 0 {
		return nil, errors.New("order must contain at least one item")
	}

	if err := s.repository.CreateOrder(ctx, order, orderItems); err != nil {
		if errors.Is(err, repository.ErrIdempotencyKeyConflict) {
			existingOrder, err := s.repository.GetOrderByIdempotencyKey(
				ctx,
				input.IdempotencyKey,
			)
			if err != nil {
				return nil, err
			}

			return existingOrder, nil
		}

		return nil, err
	}

	return order, nil
}

type GetOrderResult struct {
	Order *repository.Order
	Items []repository.OrderItem
}

func (s *OrderService) GetOrder(
	ctx context.Context,
	orderID uuid.UUID,
	token uuid.UUID,
) (*GetOrderResult, error) {

	order, err := s.repository.GetOrderByIDAndToken(
		ctx,
		orderID,
		token,
	)

	if err != nil {
		return nil, err
	}

	items, err := s.repository.GetOrderItems(
		ctx,
		orderID,
	)

	if err != nil {
		return nil, err
	}

	return &GetOrderResult{
		Order: order,
		Items: items,
	}, nil
}

func (s *OrderService) GetOrderByID(
	ctx context.Context,
	orderID uuid.UUID,
) (*repository.Order, error) {
	return s.repository.GetOrderByID(ctx, orderID)
}

func (s *OrderService) PayForTheOrder(
	ctx context.Context,
	orderID uuid.UUID,
	token uuid.UUID,
) error {
	return s.repository.PayForTheOrder(
		ctx,
		orderID,
		token,
	)
}
