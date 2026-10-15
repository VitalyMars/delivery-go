package service

import (
	"context"
	"errors"

	"github.com/VitalyMars/delivery_go/services/catalog/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type RestaurantService struct {
	repository *repository.RestaurantRepository
}

var ErrRestaurantNotFound = errors.New("restaurant not found")

func NewRestaurantService(repository *repository.RestaurantRepository) *RestaurantService {
	return &RestaurantService{
		repository: repository,
	}
}

func (s *RestaurantService) GetRestaurants(ctx context.Context) ([]repository.Restaurant, error) {
	return s.repository.GetRestaurants(ctx)
}

func (s *RestaurantService) GetRestaurantByID(ctx context.Context, id uuid.UUID) (*repository.Restaurant, error) {
	restaurant, err := s.repository.GetRestaurantByID(ctx, id)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRestaurantNotFound
		}

		return nil, err
	}

	return restaurant, nil
}
