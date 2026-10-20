package config

import (
	"os"

	"github.com/VitalyMars/delivery_go/pkg/transport"
)

type Config struct {
	DatabaseURL            string
	HTTPAddress            string
	KafkaBrokers           []string
	KafkaTopic             string
	KafkaGroupID           string
	UserServiceGRPCAddress string
	SMTPHost               string
	SMTPPort               string
	SMTPFrom               string
}

func Load() Config {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://app:catalog@localhost:5433/notification_db?sslmode=disable"
	}

	userServiceGRPCAddress := os.Getenv("USER_SERVICE_GRPC_ADDRESS")
	if userServiceGRPCAddress == "" {
		userServiceGRPCAddress = "localhost:9094"
	}

	httpAddress := os.Getenv("HTTP_ADDRESS")
	if httpAddress == "" {
		httpAddress = ":8084"
	}

	kafkaTopic := os.Getenv("KAFKA_TOPIC")
	if kafkaTopic == "" {
		kafkaTopic = "order.ready_for_delivery"
	}

	kafkaGroupID := os.Getenv("KAFKA_GROUP_ID")
	if kafkaGroupID == "" {
		kafkaGroupID = "notification-service-v1"
	}

	smtpHost := os.Getenv("SMTP_HOST")
	if smtpHost == "" {
		smtpHost = "localhost"
	}

	smtpPort := os.Getenv("SMTP_PORT")
	if smtpPort == "" {
		smtpPort = "1025"
	}

	smtpFrom := os.Getenv("SMTP_FROM")
	if smtpFrom == "" {
		smtpFrom = "notifications@delivery.local"
	}

	return Config{
		DatabaseURL:            databaseURL,
		HTTPAddress:            httpAddress,
		KafkaBrokers:           transport.Brokers(),
		KafkaTopic:             kafkaTopic,
		KafkaGroupID:           kafkaGroupID,
		UserServiceGRPCAddress: userServiceGRPCAddress,
		SMTPHost:               smtpHost,
		SMTPPort:               smtpPort,
		SMTPFrom:               smtpFrom,
	}
}
