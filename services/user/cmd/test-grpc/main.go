package main

import (
	"context"
	"log"
	"time"

	userv1 "github.com/VitalyMars/delivery_go/gen/user/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient(
		"127.0.0.1:9094",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	log.Println("Waiting for gRPC connection...")

	conn.Connect()

	if !waitForReady(ctx, conn) {
		log.Fatalf("gRPC connection not ready: %v", ctx.Err())
	}

	log.Println("gRPC connection ready")

	client := userv1.NewUserServiceClient(conn)

	user, err := client.GetUserByID(ctx, &userv1.GetUserByIDRequest{
		UserId: "bd01d3dd-9c6a-4f00-a1d6-8d03d0d69d8c",
	})
	if err != nil {
		log.Fatalf("GetUserByID failed: %v", err)
	}

	log.Printf(
		"User found: id=%s, name=%s, email=%s",
		user.GetUserId(),
		user.GetName(),
		user.GetEmail(),
	)
}

func waitForReady(ctx context.Context, conn *grpc.ClientConn) bool {
	for {
		if conn.GetState().String() == "READY" {
			return true
		}

		if !conn.WaitForStateChange(ctx, conn.GetState()) {
			return false
		}
	}
}
