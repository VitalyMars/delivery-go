package userclient

import (
	"context"
	"errors"
	"strings"
	"time"

	userv1 "github.com/VitalyMars/delivery_go/gen/user/v1"
	"github.com/VitalyMars/delivery_go/pkg/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var ErrEmptyEmail = errors.New("user service returned an empty email")

type Client struct {
	conn   *grpc.ClientConn
	client userv1.UserServiceClient
}

func New(address string) (*Client, error) {
	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(transport.InternalClient),
	)
	if err != nil {
		return nil, err
	}

	return &Client{
		conn:   conn,
		client: userv1.NewUserServiceClient(conn),
	}, nil
}

func (c *Client) GetEmail(ctx context.Context, userID string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	user, err := c.client.GetUserByID(ctx, &userv1.GetUserByIDRequest{
		UserId: userID,
	})
	if err != nil {
		return "", err
	}

	email := strings.TrimSpace(user.GetEmail())
	if email == "" {
		return "", ErrEmptyEmail
	}

	return email, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}
