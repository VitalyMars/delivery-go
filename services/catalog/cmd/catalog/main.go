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

	v1 "github.com/VitalyMars/delivery_go/gen/catalog/v1"
	"github.com/VitalyMars/delivery_go/pkg/transport"
	"github.com/VitalyMars/delivery_go/services/catalog/internal/config"
	grpcserver "github.com/VitalyMars/delivery_go/services/catalog/internal/grpc"
	"github.com/VitalyMars/delivery_go/services/catalog/internal/handler"
	"github.com/VitalyMars/delivery_go/services/catalog/internal/repository"
	"github.com/VitalyMars/delivery_go/services/catalog/internal/service"
	"google.golang.org/grpc"
)

func main() {
	if err := transport.ValidateInternalKey(); err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	db, err := config.NewDB(startup)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err = db.Ping(startup); err != nil {
		log.Fatal(err)
	}
	restaurants := service.NewRestaurantService(repository.NewRestaurantRepository(db))
	dishes := service.NewDishService(repository.NewDishRepository(db))
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(transport.InternalServer))
	v1.RegisterCatalogServiceServer(grpcServer, grpcserver.NewServer(restaurants, dishes))
	listener, err := net.Listen("tcp", transport.Env("GRPC_ADDRESS", ":9090"))
	if err != nil {
		log.Fatal(err)
	}
	server := transport.Server(transport.Env("HTTP_ADDRESS", ":8080"), handler.NewRouter(handler.NewRestaurantHandler(restaurants), handler.NewDishHandler(dishes)))
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Printf("gRPC: %v", err)
			stop()
		}
	}()
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP: %v", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdown); err != nil {
		log.Printf("HTTP shutdown: %v", err)
		_ = server.Close()
	}
	transport.StopGRPC(grpcServer)
}
