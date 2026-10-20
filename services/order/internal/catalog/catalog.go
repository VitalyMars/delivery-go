package catalog

import (
	"context"
	"time"

	catalogv1 "github.com/VitalyMars/delivery_go/gen/catalog/v1"
	"github.com/VitalyMars/delivery_go/pkg/transport"
	"google.golang.org/grpc"
)

type Client struct {
	client catalogv1.CatalogServiceClient
	conn   *grpc.ClientConn
}

func NewClient(address string) (*Client, error) {
	conn, err := grpc.NewClient(address, grpc.WithInsecure(), grpc.WithUnaryInterceptor(transport.InternalClient))
	if err != nil {
		return nil, err
	}

	return &Client{
		client: catalogv1.NewCatalogServiceClient(conn),
		conn:   conn,
	}, nil
}

func (c *Client) ValidateItems(
	ctx context.Context,
	restaurantID string,
	items []*catalogv1.OrderItem,
) (*catalogv1.ValidateItemsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	return c.client.ValidateItems(ctx, &catalogv1.ValidateItemsRequest{
		RestaurantId: restaurantID,
		Items:        items,
	})
}

func (c *Client) Close() error {
	return c.conn.Close()
}
