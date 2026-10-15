package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	userv1 "github.com/VitalyMars/delivery_go/gen/user/v1"
	usergrpc "github.com/VitalyMars/delivery_go/services/user/internal/grpc"
	"google.golang.org/grpc"

	"github.com/VitalyMars/delivery_go/services/user/internal/config"
	"github.com/VitalyMars/delivery_go/services/user/internal/handler"
	"github.com/VitalyMars/delivery_go/services/user/internal/repository"
	"github.com/VitalyMars/delivery_go/services/user/internal/service"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	db, err := config.NewDB(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(ctx); err != nil {
		log.Fatal(err)
	}
	log.Println("User Service: database connected")

	userRepository := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepository)
	userHandler := handler.NewUserHandler(userService)

	// gRPC server
	grpcServer := grpc.NewServer()
	userGRPCHandler := usergrpc.NewServer(userService)
	userv1.RegisterUserServiceServer(grpcServer, userGRPCHandler)

	grpcListener, err := net.Listen("tcp", ":9094")
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		log.Println("User Service gRPC listening on :9094")
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Printf("gRPC server error: %v", err)
			stop()
		}
	}()

	server := &http.Server{
		Addr:              ":8083",
		Handler:           handler.NewRouter(userHandler),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Println("User Service HTTP listening on :8083")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}

	grpcServer.GracefulStop()
}
