package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jackc/pgx/v5"
	"github.com/mahirpatel04/transit-route-optimizer/internal/config"
	"github.com/mahirpatel04/transit-route-optimizer/internal/db"
	"github.com/mahirpatel04/transit-route-optimizer/internal/gtfs"
)

func main() {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(run)
		return
	}

	if err := run(context.Background()); err != nil {
		log.Fatalf("ingest failed: %v", err)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	conn, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("unable to connect to database: %w", err)
	}
	defer conn.Close(ctx)

	body, err := gtfs.FetchData("https://rrgtfsfeeds.s3.amazonaws.com/gtfs_subway.zip")
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}

	zr, err := gtfs.OpenZip(body)
	if err != nil {
		return fmt.Errorf("zip open failed: %w", err)
	}

	stopsData, err := gtfs.ReadFileFromZip(zr, "stops.txt")
	if err != nil {
		return fmt.Errorf("read stops.txt failed: %w", err)
	}

	stops, err := gtfs.ParseStopFile(stopsData)
	if err != nil {
		return fmt.Errorf("parse stops.txt failed: %w", err)
	}

	routesData, err := gtfs.ReadFileFromZip(zr, "routes.txt")
	if err != nil {
		return fmt.Errorf("read routes.txt failed: %w", err)
	}

	routes, err := gtfs.ParseRouteFile(routesData)
	if err != nil {
		return fmt.Errorf("parse routes.txt failed: %w", err)
	}

	calendarData, err := gtfs.ReadFileFromZip(zr, "calendar.txt")
	if err != nil {
		return fmt.Errorf("read calendar.txt failed: %w", err)
	}

	calendars, err := gtfs.ParseCalendarFile(calendarData)
	if err != nil {
		return fmt.Errorf("parse calendar.txt failed: %w", err)
	}

	calendarDatesData, err := gtfs.ReadFileFromZip(zr, "calendar_dates.txt")
	if err != nil {
		return fmt.Errorf("read calendar_dates.txt failed: %w", err)
	}

	calendarDates, err := gtfs.ParseCalendarDatesFile(calendarDatesData)
	if err != nil {
		return fmt.Errorf("parse calendar_dates.txt failed: %w", err)
	}

	tripsData, err := gtfs.ReadFileFromZip(zr, "trips.txt")
	if err != nil {
		return fmt.Errorf("read trips.txt failed: %w", err)
	}

	trips, err := gtfs.ParseTripFile(tripsData)
	if err != nil {
		return fmt.Errorf("parse trips.txt failed: %w", err)
	}

	stopTimesData, err := gtfs.ReadFileFromZip(zr, "stop_times.txt")
	if err != nil {
		return fmt.Errorf("read stop_times.txt failed: %w", err)
	}

	stopTimes, err := gtfs.ParseStopTimeFile(stopTimesData)
	if err != nil {
		return fmt.Errorf("parse stop_times.txt failed: %w", err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := db.TruncateAll(ctx, tx); err != nil {
		return fmt.Errorf("truncate failed: %w", err)
	}

	if _, err := db.InsertStops(ctx, tx, stops); err != nil {
		return fmt.Errorf("insert stops failed: %w", err)
	}

	if _, err := db.InsertRoutes(ctx, tx, routes); err != nil {
		return fmt.Errorf("insert routes failed: %w", err)
	}

	if _, err := db.InsertCalendar(ctx, tx, calendars); err != nil {
		return fmt.Errorf("insert calendar failed: %w", err)
	}

	if _, err := db.InsertCalendarDates(ctx, tx, calendarDates); err != nil {
		return fmt.Errorf("insert calendar_dates failed: %w", err)
	}

	if _, err := db.InsertTrips(ctx, tx, trips); err != nil {
		return fmt.Errorf("insert trips failed: %w", err)
	}

	if _, err := db.InsertStopTimes(ctx, tx, stopTimes); err != nil {
		return fmt.Errorf("insert stop_times failed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}
