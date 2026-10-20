package grpc

import (
	"context"
	"errors"

	v1 "github.com/VitalyMars/delivery_go/gen/catalog/v1"
	"github.com/VitalyMars/delivery_go/services/catalog/internal/service"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	v1.UnimplementedCatalogServiceServer

	restaurantService *service.RestaurantService
	dishService       *service.DishService
}

func NewServer(restaurantService *service.RestaurantService, dishService *service.DishService) *Server {
	return &Server{
		restaurantService: restaurantService,
		dishService:       dishService,
	}
}

func (s *Server) GetRestaurant(
	ctx context.Context,
	req *v1.GetRestaurantRequest,
) (*v1.GetRestaurantResponse, error) {
	restaurantID, err := uuid.Parse(req.GetRestaurantId())

	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid restaurant id")
	}

	restaurant, err := s.restaurantService.GetRestaurantByID(ctx, restaurantID)
	if err != nil {
		if errors.Is(err, service.ErrRestaurantNotFound) {
			return nil, status.Error(codes.NotFound, "restaurant not found")
		}

		return nil, status.Error(codes.Internal, "failed to get restaurant")
	}

	return &v1.GetRestaurantResponse{
		Id:      restaurant.ID.String(),
		Name:    restaurant.Name,
		Address: restaurant.Address,
	}, nil

}

func (s *Server) ValidateItems(
	ctx context.Context,
	req *v1.ValidateItemsRequest,
) (*v1.ValidateItemsResponse, error) {
	restaurantID, err := uuid.Parse(req.GetRestaurantId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid restaurant id")
	}

	_, err = s.restaurantService.GetRestaurantByID(ctx, restaurantID)
	if err != nil {
		if errors.Is(err, service.ErrRestaurantNotFound) {
			return nil, status.Error(codes.NotFound, "restaurant not found")
		}

		return nil, status.Error(codes.Internal, "failed to get restaurant")
	}

	if len(req.GetItems()) == 0 || len(req.GetItems()) > 100 {
		return nil, status.Error(codes.InvalidArgument, "items must contain 1..100 entries")
	}
	response := &v1.ValidateItemsResponse{
		Valid: true,
		Items: make([]*v1.ValidatedItem, 0, len(req.GetItems())),
	}

	for _, item := range req.GetItems() {
		if item.GetQuantity() <= 0 || item.GetQuantity() > 1000 {
			return nil, status.Error(codes.InvalidArgument, "quantity must be greater than zero")
		}

		dishID, err := uuid.Parse(item.GetDishId())
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid dish id")
		}

		dish, err := s.dishService.GetDishByID(ctx, dishID)
		if err != nil {
			if errors.Is(err, service.ErrDishNotFound) {
				return nil, status.Error(codes.NotFound, "dish not found")
			}

			return nil, status.Error(codes.Internal, "failed to get dish")
		}

		if dish.RestaurantID != restaurantID {
			return nil, status.Error(codes.InvalidArgument, "dish belongs to another restaurant")
		}

		if !dish.Available {
			return nil, status.Error(codes.InvalidArgument, "dish is unavailable")
		}
		if dish.Price < 0 || int64(dish.Price) > 2147483647 {
			return nil, status.Error(codes.Internal, "invalid catalog price")
		}

		response.Items = append(response.Items, &v1.ValidatedItem{
			ItemId:            dish.ID.String(),
			RestaurantId:      dish.RestaurantID.String(),
			RequestedQuantity: item.GetQuantity(),
			Available:         dish.Available,
			ActualPrice:       int32(dish.Price),
			Name:              dish.Name,
		})
	}

	return response, nil
}

func (s *Server) SetDishAvailability(
	ctx context.Context,
	req *v1.SetDishAvailabilityRequest,
) (*v1.SetDishAvailabilityResponse, error) {
	dishID, err := uuid.Parse(req.GetDishId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid dish id")
	}

	err = s.dishService.SetDishAvailability(
		ctx,
		dishID,
		req.GetAvailable(),
	)
	if err != nil {
		if errors.Is(err, service.ErrDishNotFound) {
			return nil, status.Error(codes.NotFound, "dish not found")
		}

		return nil, status.Error(codes.Internal, "failed to set dish availability")
	}

	return &v1.SetDishAvailabilityResponse{
		Success: true,
	}, nil
}
