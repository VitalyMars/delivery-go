package main

import (
	"context"
	"fmt"
	"log"
	"time"

	orderv1 "github.com/VitalyMars/delivery_go/gen/order/v1"
	"github.com/VitalyMars/delivery_go/pkg/transport"
	"google.golang.org/grpc"
)

func main() {
	conn, err := grpc.NewClient(
		"127.0.0.1:9091",
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(transport.InternalClient),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := orderv1.NewOrderServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	orderID := transport.Env("ORDER_ID", "")
	if orderID == "" {
		log.Fatal("ORDER_ID is required")
	}

	fmt.Println("Getting order...")

	response, err := client.GetOrder(
		ctx,
		&orderv1.GetOrderRequest{
			OrderId: orderID,
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Order ID: %s\n", response.GetOrderId())
	fmt.Printf("Status: %s\n", response.GetStatus())
	fmt.Printf("Total: %d\n", response.GetTotal())
}
