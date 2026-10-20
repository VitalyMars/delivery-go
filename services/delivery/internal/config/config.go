package config

import (
	"fmt"
	"os"

	"github.com/VitalyMars/delivery_go/pkg/transport"
)

type Config struct {
	DatabaseURL  string
	KafkaBrokers []string
	KafkaTopic   string
	KafkaGroupID string
	HTTPAddress  string
}

func Load() (Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://app:catalog@localhost:5433/delivery_db?sslmode=disable"
	}

	kafkaTopic := os.Getenv("KAFKA_TOPIC")
	if kafkaTopic == "" {
		kafkaTopic = "order.ready_for_delivery"
	}

	kafkaGroupID := os.Getenv("KAFKA_GROUP_ID")
	if kafkaGroupID == "" {
		kafkaGroupID = "delivery-service-v1"
	}

	httpAddress := os.Getenv("HTTP_ADDRESS")
	if httpAddress == "" {
		httpAddress = ":8082"
	}

	if databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}

	return Config{
		DatabaseURL:  databaseURL,
		KafkaBrokers: transport.Brokers(),
		KafkaTopic:   kafkaTopic,
		KafkaGroupID: kafkaGroupID,
		HTTPAddress:  httpAddress,
	}, nil
}
