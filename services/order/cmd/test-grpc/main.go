package main

import (
	"context"
	"fmt"
	"log"
	"time"

	orderv1 "github.com/VitalyMars/delivery_go/gen/order/v1"
	"google.golang.org/grpc"
)

func main() {
	conn, err := grpc.NewClient(
		"127.0.0.1:9091",
		grpc.WithInsecure(),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := orderv1.NewOrderServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	orderID := "6854de01-e84d-4ac1-9f1c-775b0b80b96d"

	fmt.Println("Changing order status to ready_for_delivery...")

	changeResponse, err := client.ChangeOrderStatus(
		ctx,
		&orderv1.ChangeOrderStatusRequest{
			OrderId: orderID,
			Status:  "ready_for_delivery",
			Reason:  "order prepared",
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Ready for delivery: %v\n", changeResponse.GetSuccess())

	fmt.Println()
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
