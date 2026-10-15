package service

import (
	"context"
	"errors"

	"github.com/VitalyMars/delivery_go/services/catalog/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type DishService struct {
	repository *repository.DishRepository
}

var ErrDishNotFound = errors.New("dish not found")

func NewDishService(repository *repository.DishRepository) *DishService {
	return &DishService{
		repository: repository,
	}
}

func (s *DishService) GetDishesByRestaurantID(ctx context.Context, id uuid.UUID) ([]repository.Dish, error) {
	return s.repository.GetDishesByRestaurantID(ctx, id)
}

func (s *DishService) GetDishByID(ctx context.Context, id uuid.UUID) (*repository.Dish, error) {
	dish, err := s.repository.GetDishByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDishNotFound
		}

		return nil, err
	}

	return dish, nil
}

func (s *DishService) SetDishAvailability(
	ctx context.Context,
	id uuid.UUID,
	available bool,
) error {
	_, err := s.repository.GetDishByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrDishNotFound
		}

		return err
	}

	return s.repository.SetDishAvailability(ctx, id, available)
}
