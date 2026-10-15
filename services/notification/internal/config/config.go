package config

import "os"

type Config struct {
	DatabaseURL string
	HTTPAddress string
}

func Load() Config {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://app:catalog@localhost:5433/notification_db?sslmode=disable"
	}

	httpAddress := os.Getenv("HTTP_ADDRESS")
	if httpAddress == "" {
		httpAddress = ":8084"
	}

	return Config{
		DatabaseURL: databaseURL,
		HTTPAddress: httpAddress,
	}
}
