package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL string

	Hikvision HikvisionConfig
	Inspireface InspirefaceConfig
}

type HikvisionConfig struct {
	Host     string
	Username string
	Password string
}

type InspirefaceConfig struct {
	Host string
}

func Load() (Config, error) {
	_ = godotenv.Load(".env")

	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Hikvision: HikvisionConfig{
			Host:     os.Getenv("HIKVISION_HOST"),
			Username: os.Getenv("HIKVISION_USERNAME"),
			Password: os.Getenv("HIKVISION_PASSWORD"),
		},
		Inspireface: InspirefaceConfig{
			Host: os.Getenv("INSPIREFACE_HOST"),
		},
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL cannot be empty")
	}
	if cfg.Hikvision.Host == "" {
		return Config{}, errors.New("HIKVISION_HOST cannot be empty")
	}
	if cfg.Hikvision.Username == "" {
		return Config{}, errors.New("HIKVISION_USERNAME cannot be empty")
	}
	if cfg.Hikvision.Password == "" {
		return Config{}, errors.New("HIKVISION_PASSWORD cannot be empty")
	}
	if cfg.Inspireface.Host == "" {
		return Config{}, errors.New("INSPIREFACE_HOST cannot be empty")
	}

	return cfg, nil
}