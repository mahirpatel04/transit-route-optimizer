package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/jackc/pgx/v5"
	"github.com/mahirpatel04/transit-route-optimizer/internal/api"
	"github.com/mahirpatel04/transit-route-optimizer/internal/config"
	"github.com/mahirpatel04/transit-route-optimizer/internal/db"
	"github.com/mahirpatel04/transit-route-optimizer/internal/geocode"
	"github.com/mahirpatel04/transit-route-optimizer/internal/gtfs"
)

func isCronEvent(raw json.RawMessage) bool {
	var probe struct {
		Source string `json:"source"`
	}
	_ = json.Unmarshal(raw, &probe)
	return probe.Source == "aws.events"
}

func main() {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") != "" {
		lambda.Start(handler)
		return
	}

	if err := run(context.Background()); err != nil {
		log.Fatalf("ingest failed: %v", err)
	}
}

func handler(ctx context.Context, raw json.RawMessage) (any, error) {
	if isCronEvent(raw) {
		return nil, run(ctx)
	}

	var req events.APIGatewayV2HTTPRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		log.Printf("handler: failed to parse HTTP request event: %v", err)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       `{"error":"internal error"}`,
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}

	cfg, err := config.Load()
	if err != nil {
		log.Printf("handler: config error: %v", err)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       `{"error":"internal error"}`,
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}

	conn, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Printf("handler: unable to connect to database: %v", err)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       `{"error":"internal error"}`,
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}
	defer conn.Close(ctx)

	geocoder, err := geocode.NewLocationServiceGeocoder(ctx, cfg.PlaceIndexName)
	if err != nil {
		log.Printf("handler: failed to init geocoder: %v", err)
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 500,
			Body:       `{"error":"internal error"}`,
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}
	return httpadapter.NewV2(api.NewMux(conn, geocoder)).ProxyWithContext(ctx, req)
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

	fetchTime := time.Now()

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

	if err := db.SetLastFetchTime(ctx, tx, fetchTime); err != nil {
		return fmt.Errorf("set last fetch time failed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}
