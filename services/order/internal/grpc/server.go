package grpc

import (
	"context"
	"errors"

	orderv1 "github.com/VitalyMars/delivery_go/gen/order/v1"
	"github.com/VitalyMars/delivery_go/services/order/internal/service"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	orderv1.UnimplementedOrderServiceServer

	orderService *service.OrderService
}

func NewServer(orderService *service.OrderService) *Server {
	return &Server{
		orderService: orderService,
	}
}

func (s *Server) GetOrder(
	ctx context.Context,
	req *orderv1.GetOrderRequest,
) (*orderv1.GetOrderResponse, error) {
	orderID, err := uuid.Parse(req.GetOrderId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid order id")
	}

	result, err := s.orderService.GetOrderByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "order not found")
		}
		return nil, status.Error(codes.Internal, "failed to read order")
	}

	return &orderv1.GetOrderResponse{
		OrderId: result.ID.String(),
		Status:  result.Status,
		Total:   int32(result.Total),
	}, nil
}
