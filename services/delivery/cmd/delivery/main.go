package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VitalyMars/delivery_go/services/delivery/internal/config"
	"github.com/VitalyMars/delivery_go/services/delivery/internal/kafka"
	"github.com/VitalyMars/delivery_go/services/delivery/internal/repository"
	"github.com/google/uuid"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	dbCtx, cancelDB := context.WithTimeout(ctx, 10*time.Second)
	defer cancelDB()

	pool, err := repository.NewPostgresPool(dbCtx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to PostgreSQL: %v", err)
	}
	defer pool.Close()

	deliveryRepo := repository.NewDeliveryRepository(pool)

	// Восстанавливаем очередь после запуска сервиса.
	// Восстанавливаем очередь при запуске и затем проверяем её каждые 5 секунд.
	go func() {
		processQueue(ctx, deliveryRepo)

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				processQueue(ctx, deliveryRepo)
			}
		}
	}()

	outboxRepo := repository.NewOutboxRepository(pool)

	producer, err := kafka.NewProducer(cfg.KafkaBrokers)
	if err != nil {
		log.Fatalf("create Kafka producer: %v", err)
	}
	defer producer.Close()

	publisher := kafka.NewOutboxPublisher(outboxRepo, producer)
	go publisher.Run(ctx)

	consumer, err := kafka.NewConsumer(
		cfg.KafkaBrokers,
		cfg.KafkaGroupID,
		cfg.KafkaTopic,
		deliveryRepo,
	)
	if err != nil {
		log.Fatalf("create Kafka consumer: %v", err)
	}

	go consumer.Run(ctx)

	mux := http.NewServeMux()

	mux.HandleFunc("POST /deliveries/{id}/complete", func(w http.ResponseWriter, r *http.Request) {
		deliveryID, err := uuid.Parse(r.PathValue("id"))
		if err != nil {
			http.Error(w, "invalid delivery ID", http.StatusBadRequest)
			return
		}

		if err := deliveryRepo.CompleteDelivery(r.Context(), deliveryID); err != nil {
			switch {
			case errors.Is(err, repository.ErrDeliveryNotFound):
				http.Error(w, "delivery not found", http.StatusNotFound)
			case errors.Is(err, repository.ErrDeliveryAlreadyDone):
				http.Error(w, "delivery already completed", http.StatusConflict)
			case errors.Is(err, repository.ErrDeliveryCannotComplete):
				http.Error(w, "delivery cannot be completed in its current status", http.StatusConflict)
			default:
				log.Printf("complete delivery %s: %v", deliveryID, err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
			return
		}

		// The courier has been released. Assign the next waiting delivery immediately.
		processQueue(r.Context(), deliveryRepo)

		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("Delivery Service health endpoint listening on %s", cfg.HTTPAddress)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("health server error: %v", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Println("Shutting down Delivery Service...")

	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancelShutdown()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("health server shutdown error: %v", err)
	}
}

func processQueue(ctx context.Context, repo *repository.DeliveryRepository) {
	for {
		assigned, err := repo.AssignNextWaitingDelivery(ctx)
		if err != nil {
			log.Printf("process delivery queue: %v", err)
			return
		}

		if !assigned {
			return
		}

		log.Println("Assigned courier to next waiting delivery")
	}
}
