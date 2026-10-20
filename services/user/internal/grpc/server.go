package grpc

import (
	"context"
	"errors"

	userv1 "github.com/VitalyMars/delivery_go/gen/user/v1"
	"github.com/VitalyMars/delivery_go/services/user/internal/repository"
	"github.com/VitalyMars/delivery_go/services/user/internal/service"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	userv1.UnimplementedUserServiceServer

	userService *service.UserService
}

func NewServer(userService *service.UserService) *Server {
	return &Server{
		userService: userService,
	}
}

func (s *Server) GetUserByID(
	ctx context.Context,
	req *userv1.GetUserByIDRequest,
) (*userv1.GetUserByIDResponse, error) {

	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user id")
	}

	user, err := s.userService.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "failed to read user")
	}

	return &userv1.GetUserByIDResponse{
		UserId: user.ID.String(),
		Name:   user.Name,
		Email:  user.Email,
	}, nil
}
