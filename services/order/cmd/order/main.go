package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	orderv1 "github.com/VitalyMars/delivery_go/gen/order/v1"
	"github.com/VitalyMars/delivery_go/pkg/auth"
	"github.com/VitalyMars/delivery_go/pkg/transport"
	"github.com/VitalyMars/delivery_go/services/order/internal/catalog"
	"github.com/VitalyMars/delivery_go/services/order/internal/config"
	ordergrpc "github.com/VitalyMars/delivery_go/services/order/internal/grpc"
	"github.com/VitalyMars/delivery_go/services/order/internal/handler"
	"github.com/VitalyMars/delivery_go/services/order/internal/kafka"
	"github.com/VitalyMars/delivery_go/services/order/internal/repository"
	"github.com/VitalyMars/delivery_go/services/order/internal/service"
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
	var workers sync.WaitGroup

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

	log.Println("database connected")

	kafkaProducer, err := kafka.NewProducer(transport.Brokers())
	if err != nil {
		log.Fatal(err)
	}
	defer kafkaProducer.Close()

	log.Println("Kafka producer created")

	outboxRepository := repository.NewOutboxRepository(db)
	outboxPublisher := kafka.NewOutboxPublisher(
		outboxRepository,
		kafkaProducer,
	)

	workers.Go(func() { outboxPublisher.Run(ctx) })
	log.Println("outbox publisher started")

	catalogClient, err := catalog.NewClient(transport.Env("CATALOG_GRPC_ADDRESS", "127.0.0.1:9090"))
	if err != nil {
		log.Fatal(err)
	}
	defer catalogClient.Close()

	log.Println("catalog gRPC client created")

	orderRepository := repository.NewOrderRepository(
		db,
		outboxRepository,
	)

	orderConsumer, err := kafka.NewConsumer(
		transport.Brokers(),
		"order-service-delivery-events-v1",
		"delivery.courier_assigned",
		orderRepository,
	)
	if err != nil {
		log.Fatalf("create Order Kafka consumer: %v", err)
	}

	workers.Go(func() {
		if err := orderConsumer.Run(ctx); err != nil {
			log.Printf("consumer stopped: %v", err)
			stop()
		}
	})
	log.Println("Order Kafka consumer started")

	orderService := service.NewOrderService(
		orderRepository,
		catalogClient,
	)
	orderHandler := handler.NewOrderHandler(orderService)

	orderGRPCServer := ordergrpc.NewServer(orderService)
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(transport.InternalServer))

	orderv1.RegisterOrderServiceServer(grpcServer, orderGRPCServer)

	grpcListener, err := net.Listen("tcp", transport.Env("GRPC_ADDRESS", ":9091"))
	if err != nil {
		log.Fatal(err)
	}

	grpcErr := make(chan error, 1)

	go func() {
		log.Println("gRPC server started on :9091")
		if err := grpcServer.Serve(grpcListener); err != nil {
			grpcErr <- err
		}
	}()

	router := handler.NewRouter(orderHandler)

	httpServer := transport.Server(transport.Env("HTTP_ADDRESS", ":8081"), router)

	httpErr := make(chan error, 1)

	go func() {
		log.Println("HTTP server started on :8081")
		if err := httpServer.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			httpErr <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("shutdown signal received")
	case err := <-grpcErr:
		log.Printf("gRPC server stopped unexpectedly: %v", err)
	case err := <-httpErr:
		log.Printf("HTTP server stopped unexpectedly: %v", err)
	}

	stop()
	transport.StopGRPC(grpcServer)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
		_ = httpServer.Close()
	}

	log.Println("Order Service stopped")
	workers.Wait()

}
