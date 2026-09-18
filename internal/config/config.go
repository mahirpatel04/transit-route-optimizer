package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL    string
	PlaceIndexName string
}

func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		fmt.Printf("godotenv warning: %v\n", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL env variable not set")
	}

	placeIndexName := os.Getenv("PLACE_INDEX_NAME")
	if placeIndexName == "" {
		return nil, fmt.Errorf("PLACE_INDEX_NAME env variable not set")
	}

	return &Config{
		DatabaseURL:    databaseURL,
		PlaceIndexName: placeIndexName,
	}, nil
}
