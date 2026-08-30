package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/mahirpatel04/transit-route-optimizer/internal/config"
	"github.com/mahirpatel04/transit-route-optimizer/internal/db"
	"github.com/mahirpatel04/transit-route-optimizer/internal/gtfs"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	conn, err := pgx.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	body, err := gtfs.FetchData("https://rrgtfsfeeds.s3.amazonaws.com/gtfs_subway.zip")
	if err != nil {
		log.Fatalf("fetch failed: %v", err)
	}

	zr, err := gtfs.OpenZip(body)
	if err != nil {
		log.Fatalf("zip open failed: %v", err)
	}

	stopsData, err := gtfs.ReadFileFromZip(zr, "stops.txt")
	if err != nil {
		log.Fatalf("read stops.txt failed: %v", err)
	}

	stops, err := gtfs.ParseStopFile(stopsData)
	if err != nil {
		log.Fatalf("parse stops.txt failed: %v", err)
	}

	inserted, err := db.InsertStops(context.Background(), conn, stops)
	if err != nil {
		log.Fatalf("insert stops failed: %v", err)
	}

	fmt.Printf("inserted %d stops\n", inserted)
}
