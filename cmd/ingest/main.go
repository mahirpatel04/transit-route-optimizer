package main

import (
	"context"
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

	ctx := context.Background()

	conn, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("unable to connect to database: %v", err)
	}
	defer conn.Close(ctx)

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

	routesData, err := gtfs.ReadFileFromZip(zr, "routes.txt")
	if err != nil {
		log.Fatalf("read routes.txt failed: %v", err)
	}

	routes, err := gtfs.ParseRouteFile(routesData)
	if err != nil {
		log.Fatalf("parse routes.txt failed: %v", err)
	}

	calendarData, err := gtfs.ReadFileFromZip(zr, "calendar.txt")
	if err != nil {
		log.Fatalf("read calendar.txt failed: %v", err)
	}

	calendars, err := gtfs.ParseCalendarFile(calendarData)
	if err != nil {
		log.Fatalf("parse calendar.txt failed: %v", err)
	}

	calendarDatesData, err := gtfs.ReadFileFromZip(zr, "calendar_dates.txt")
	if err != nil {
		log.Fatalf("read calendar_dates.txt failed: %v", err)
	}

	calendarDates, err := gtfs.ParseCalendarDatesFile(calendarDatesData)
	if err != nil {
		log.Fatalf("parse calendar_dates.txt failed: %v", err)
	}

	tripsData, err := gtfs.ReadFileFromZip(zr, "trips.txt")
	if err != nil {
		log.Fatalf("read trips.txt failed: %v", err)
	}

	trips, err := gtfs.ParseTripFile(tripsData)
	if err != nil {
		log.Fatalf("parse trips.txt failed: %v", err)
	}

	stopTimesData, err := gtfs.ReadFileFromZip(zr, "stop_times.txt")
	if err != nil {
		log.Fatalf("read stop_times.txt failed: %v", err)
	}

	stopTimes, err := gtfs.ParseStopTimeFile(stopTimesData)
	if err != nil {
		log.Fatalf("parse stop_times.txt failed: %v", err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Fatalf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	if err := db.TruncateAll(ctx, tx); err != nil {
		log.Fatalf("truncate failed: %v", err)
	}

	if _, err := db.InsertStops(ctx, tx, stops); err != nil {
		log.Fatalf("insert stops failed: %v", err)
	}

	if _, err := db.InsertRoutes(ctx, tx, routes); err != nil {
		log.Fatalf("insert routes failed: %v", err)
	}

	if _, err := db.InsertCalendar(ctx, tx, calendars); err != nil {
		log.Fatalf("insert calendar failed: %v", err)
	}

	if _, err := db.InsertCalendarDates(ctx, tx, calendarDates); err != nil {
		log.Fatalf("insert calendar_dates failed: %v", err)
	}

	if _, err := db.InsertTrips(ctx, tx, trips); err != nil {
		log.Fatalf("insert trips failed: %v", err)
	}

	if _, err := db.InsertStopTimes(ctx, tx, stopTimes); err != nil {
		log.Fatalf("insert stop_times failed: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit failed: %v", err)
	}
}
