package service

import (
	"context"

	"github.com/VitalyMars/delivery_go/services/order/internal/repository"
	"github.com/google/uuid"
)

type CartService struct {
	repository *repository.CartRepository
}

func NewCartService(repository *repository.CartRepository) *CartService {
	return &CartService{
		repository: repository,
	}
}

type AddCartItemInput struct {
	CartToken uuid.UUID
	DishID    uuid.UUID
	Quantity  int
	Price     int
}

func (s *CartService) AddItem(
	ctx context.Context,
	input AddCartItemInput,
) (*repository.CartItem, error) {
	item := &repository.CartItem{
		ID:        uuid.New(),
		CartToken: input.CartToken,
		DishID:    input.DishID,
		Quantity:  input.Quantity,
		Price:     input.Price,
	}

	if err := s.repository.AddItem(ctx, item); err != nil {
		return nil, err
	}

	return item, nil
}

func (s *CartService) GetItems(
	ctx context.Context,
	cartToken uuid.UUID,
) ([]repository.CartItem, error) {
	return s.repository.GetItems(ctx, cartToken)
}

func (s *CartService) UpdateItemQuantity(
	ctx context.Context,
	cartToken uuid.UUID,
	itemID uuid.UUID,
	quantity int,
) ([]repository.CartItem, bool, error) {
	return s.repository.UpdateItemQuantity(
		ctx,
		cartToken,
		itemID,
		quantity,
	)
}

func (s *CartService) DeleteItem(
	ctx context.Context,
	cartToken uuid.UUID,
	itemID uuid.UUID,
) ([]repository.CartItem, bool, error) {
	return s.repository.DeleteItem(
		ctx,
		cartToken,
		itemID,
	)
}
