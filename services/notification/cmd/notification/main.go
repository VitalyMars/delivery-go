package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/VitalyMars/delivery_go/pkg/transport"
	"github.com/VitalyMars/delivery_go/services/notification/internal/config"
	"github.com/VitalyMars/delivery_go/services/notification/internal/email"
	"github.com/VitalyMars/delivery_go/services/notification/internal/kafka"
	"github.com/VitalyMars/delivery_go/services/notification/internal/repository"
	"github.com/VitalyMars/delivery_go/services/notification/internal/service"
	userclient "github.com/VitalyMars/delivery_go/services/notification/internal/userClient"
	"github.com/jackc/pgx/v5/pgxpool"
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
	var workers sync.WaitGroup

	cfg := config.Load()

	startupCtx, cancelStartup := context.WithTimeout(ctx, 10*time.Second)
	defer cancelStartup()
	db, err := pgxpool.New(startupCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("create database pool: %v", err)
	}
	defer db.Close()

	if err := db.Ping(startupCtx); err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	log.Println("Notification Service: database connected")

	notificationRepository := repository.NewNotificationRepository(db)

	emailSender := email.NewSender(
		cfg.SMTPHost,
		cfg.SMTPPort,
		cfg.SMTPFrom,
	)

	notificationService := service.NewNotificationService(
		notificationRepository,
		emailSender,
	)

	userServiceClient, err := userclient.New(cfg.UserServiceGRPCAddress)
	if err != nil {
		log.Fatalf("create User Service gRPC client: %v", err)
	}
	defer userServiceClient.Close()

	consumer, err := kafka.NewConsumer(
		cfg.KafkaBrokers,
		cfg.KafkaGroupID,
		cfg.KafkaTopic,
		notificationService,
		userServiceClient,
	)
	if err != nil {
		log.Fatalf("create Kafka consumer: %v", err)
	}

	workers.Go(func() {
		if err := consumer.Run(ctx); err != nil {
			log.Printf("consumer stopped: %v", err)
			stop()
		}
	})
	workers.Go(func() { notificationService.Run(ctx) })

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancelPing := context.WithTimeout(r.Context(), time.Second)
		defer cancelPing()
		if err := db.Ping(pingCtx); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := transport.Server(cfg.HTTPAddress, mux)

	go func() {
		log.Printf("Notification Service HTTP listening on %s", cfg.HTTPAddress)
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
	workers.Wait()

}
