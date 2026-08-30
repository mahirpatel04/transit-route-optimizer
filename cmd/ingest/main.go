package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/mahirpatel04/transit-route-optimizer/internal/config"
)

func main() {
	// load the config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// connect to the database
	conn, err := pgx.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	var count int
	err = conn.QueryRow(context.Background(), "SELECT count(*) FROM stops").Scan(&count)
	if err != nil {
		log.Fatalf("query failed: %v", err)
	}

	fmt.Printf("stops table has %d rows\n", count)
}
