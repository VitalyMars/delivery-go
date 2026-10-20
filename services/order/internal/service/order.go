package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"strings"

	catalogv1 "github.com/VitalyMars/delivery_go/gen/catalog/v1"
	"github.com/VitalyMars/delivery_go/services/order/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type OrderRepository interface {
	CreateOrder(context.Context, *repository.Order, []repository.OrderItem) error
	GetOrderByIdempotencyKey(context.Context, string) (*repository.Order, error)
	GetOrderByIDAndToken(context.Context, uuid.UUID, uuid.UUID) (*repository.Order, error)
	GetOrderItems(context.Context, uuid.UUID) ([]repository.OrderItem, error)
	GetOrderByID(context.Context, uuid.UUID) (*repository.Order, error)
	PayForTheOrder(context.Context, uuid.UUID, uuid.UUID) error
}
type CatalogClient interface {
	ValidateItems(context.Context, string, []*catalogv1.OrderItem) (*catalogv1.ValidateItemsResponse, error)
}

var ErrInvalidInput = errors.New("invalid order input")
var ErrIdempotencyConflict = errors.New("idempotency key reused with different input")

type OrderService struct {
	repository    OrderRepository
	catalogClient CatalogClient
}

func NewOrderService(
	repository OrderRepository,
	catalogClient CatalogClient,
) *OrderService {
	return &OrderService{
		repository:    repository,
		catalogClient: catalogClient,
	}
}

type CreateOrderInput struct {
	ClientScope     string
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

	scopedKey, requestHash, err := prepareRequest(&input)
	if err != nil {
		return nil, err
	}
	existing, err := s.repository.GetOrderByIdempotencyKey(ctx, scopedKey)
	if err == nil {
		if existing.RequestHash != requestHash {
			return nil, ErrIdempotencyConflict
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	response, err := s.catalogClient.ValidateItems(
		ctx,
		input.RestaurantID,
		input.Items,
	)
	if err != nil {
		return nil, err
	}

	if response == nil || !response.Valid || len(response.Items) != len(input.Items) {
		return nil, ErrInvalidInput
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
		IdempotencyKey:  scopedKey, RequestHash: requestHash,
	}

	orderItems := make([]repository.OrderItem, 0, len(response.Items))

	for index, item := range response.Items {
		if item == nil || item.GetItemId() != input.Items[index].GetDishId() || item.GetRequestedQuantity() != input.Items[index].GetQuantity() || item.GetRestaurantId() != input.RestaurantID || !item.GetAvailable() {
			return nil, errors.New("catalog returned inconsistent items")
		}

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

		if orderItem.Price < 0 || orderItem.Quantity <= 0 || orderItem.Quantity > 1000 {
			return nil, ErrInvalidInput
		}
		amount := int64(orderItem.Price) * int64(orderItem.Quantity)
		// Existing proto and SQL contracts use signed 32-bit amounts.
		if amount > math.MaxInt32-int64(order.Total) {
			return nil, ErrInvalidInput
		}
		order.Total += int(amount)

		orderItems = append(orderItems, orderItem)
	}

	if len(orderItems) == 0 {
		return nil, ErrInvalidInput
	}

	if err := s.repository.CreateOrder(ctx, order, orderItems); err != nil {
		if errors.Is(err, repository.ErrIdempotencyKeyConflict) {
			existingOrder, err := s.repository.GetOrderByIdempotencyKey(
				ctx,
				scopedKey,
			)
			if err != nil {
				return nil, err
			}

			if existingOrder.RequestHash != requestHash {
				return nil, ErrIdempotencyConflict
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

// prepareRequest binds retry keys to a caller and to a canonical request snapshot.
func prepareRequest(input *CreateOrderInput) (string, string, error) {
	key, err := uuid.Parse(input.IdempotencyKey)
	if err != nil || key == uuid.Nil || input.ClientScope == "" || len(input.Items) == 0 || len(input.Items) > 100 {
		return "", "", ErrInvalidInput
	}
	input.CustomerName = strings.TrimSpace(input.CustomerName)
	input.CustomerPhone = strings.TrimSpace(input.CustomerPhone)
	input.DeliveryAddress = strings.TrimSpace(input.DeliveryAddress)
	if input.CustomerName == "" || len(input.CustomerName) > 100 || input.CustomerPhone == "" || len(input.CustomerPhone) > 32 || input.DeliveryAddress == "" || len(input.DeliveryAddress) > 1000 || len(input.Comment) > 2000 {
		return "", "", ErrInvalidInput
	}
	if id, err := uuid.Parse(input.RestaurantID); err != nil || id == uuid.Nil {
		return "", "", ErrInvalidInput
	} else {
		input.RestaurantID = id.String()
	}
	items := make([]*catalogv1.OrderItem, 0, len(input.Items))
	for _, item := range input.Items {
		if item == nil || item.Quantity <= 0 || item.Quantity > 1000 {
			return "", "", ErrInvalidInput
		}
		if id, err := uuid.Parse(item.DishId); err != nil || id == uuid.Nil {
			return "", "", ErrInvalidInput
		} else {
			items = append(items, &catalogv1.OrderItem{DishId: id.String(), Quantity: item.Quantity})
		}
	}
	input.Items = items
	scoped := sha256.Sum256([]byte(input.ClientScope + "\x00" + key.String()))
	input.IdempotencyKey = key.String()
	payload, err := json.Marshal(input)
	if err != nil {
		return "", "", err
	}
	hash := sha256.Sum256(payload)
	return hex.EncodeToString(scoped[:]), hex.EncodeToString(hash[:]), nil
}
