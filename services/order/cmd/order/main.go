package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/VitalyMars/delivery_go/services/order/internal/catalog"
	"github.com/VitalyMars/delivery_go/services/order/internal/config"
	"github.com/VitalyMars/delivery_go/services/order/internal/handler"
	"github.com/VitalyMars/delivery_go/services/order/internal/kafka"
	"github.com/VitalyMars/delivery_go/services/order/internal/repository"
	"github.com/VitalyMars/delivery_go/services/order/internal/service"

	orderv1 "github.com/VitalyMars/delivery_go/gen/order/v1"
	ordergrpc "github.com/VitalyMars/delivery_go/services/order/internal/grpc"
	"google.golang.org/grpc"
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

	log.Println("database connected")

	kafkaProducer, err := kafka.NewProducer([]string{
		"localhost:9092",
	})
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

	go outboxPublisher.Run(ctx)
	log.Println("outbox publisher started")

	catalogClient, err := catalog.NewClient("127.0.0.1:9090")
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
		[]string{"localhost:9092"},
		"order-service-delivery-events-v1",
		"delivery.courier_assigned",
		orderRepository,
	)
	if err != nil {
		log.Fatalf("create Order Kafka consumer: %v", err)
	}

	go orderConsumer.Run(ctx)
	log.Println("Order Kafka consumer started")

	orderService := service.NewOrderService(
		orderRepository,
		catalogClient,
	)
	orderHandler := handler.NewOrderHandler(orderService)

	orderGRPCServer := ordergrpc.NewServer(orderService)
	grpcServer := grpc.NewServer()

	orderv1.RegisterOrderServiceServer(grpcServer, orderGRPCServer)

	grpcListener, err := net.Listen("tcp", ":9091")
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

	cartRepository := repository.NewCartRepository(db)
	cartService := service.NewCartService(cartRepository)
	cartHandler := handler.NewCartHandler(cartService)

	router := handler.NewRouter(orderHandler, cartHandler)

	httpServer := &http.Server{
		Addr:    ":8081",
		Handler: router,
	}

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
	grpcServer.GracefulStop()

	if err := httpServer.Shutdown(context.Background()); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}

	log.Println("Order Service stopped")
}
