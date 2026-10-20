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
	"github.com/VitalyMars/delivery_go/pkg/auth"
	"github.com/VitalyMars/delivery_go/pkg/transport"
	"github.com/VitalyMars/delivery_go/services/user/internal/config"
	usergrpc "github.com/VitalyMars/delivery_go/services/user/internal/grpc"
	"github.com/VitalyMars/delivery_go/services/user/internal/handler"
	"github.com/VitalyMars/delivery_go/services/user/internal/repository"
	"github.com/VitalyMars/delivery_go/services/user/internal/service"
	"google.golang.org/grpc"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()
	if err := transport.ValidateInternalKey(); err != nil {
		log.Fatal(err)
	}
	if err := auth.ValidateConfig(); err != nil {
		log.Fatal(err)
	}

	startupCtx, cancelStartup := context.WithTimeout(ctx, 10*time.Second)
	defer cancelStartup()
	db, err := config.NewDB(startupCtx)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(startupCtx); err != nil {
		log.Fatal(err)
	}
	log.Println("User Service: database connected")

	userRepository := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepository)
	userHandler := handler.NewUserHandler(userService)

	// gRPC server
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(transport.InternalServer))
	userGRPCHandler := usergrpc.NewServer(userService)
	userv1.RegisterUserServiceServer(grpcServer, userGRPCHandler)

	grpcListener, err := net.Listen("tcp", transport.Env("GRPC_ADDRESS", ":9094"))
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

	server := transport.Server(transport.Env("HTTP_ADDRESS", ":8083"), handler.NewRouter(userHandler))

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
		_ = server.Close()
	}

	transport.StopGRPC(grpcServer)

}
