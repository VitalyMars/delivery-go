package main

import (
	"context"
	"log"
	"time"

	userv1 "github.com/VitalyMars/delivery_go/gen/user/v1"
	"github.com/VitalyMars/delivery_go/pkg/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	userID := transport.Env("USER_ID", "")
	if userID == "" {
		log.Fatal("USER_ID is required")
	}
	conn, err := grpc.NewClient(
		"127.0.0.1:9094",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(transport.InternalClient),
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
		UserId: userID,
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
