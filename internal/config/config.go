package config

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	HikvisionHost     string
	HikvisionUsername string
	HikvisionPassword string

	InspireFaceHost string

	DatabaseURL string
}

func Load() *Config {
	if err := godotenv.Load(".env"); err != nil {
		slog.Info("No .env file found")
	}
	return &Config{
		HikvisionHost:     os.Getenv("HIKVISION_HOST"),
		HikvisionUsername: os.Getenv("HIKVISION_USERNAME"),
		HikvisionPassword: os.Getenv("HIKVISION_PASSWORD"),

		InspireFaceHost: os.Getenv("INSPIREFACE_HOST"),

		DatabaseURL: os.Getenv("DatabaseURL"),
	}
}
