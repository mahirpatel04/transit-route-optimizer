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

	_, err = db.InsertStops(context.Background(), conn, stops)
	if err != nil {
		log.Fatalf("insert stops failed: %v", err)
	}

	routesData, err := gtfs.ReadFileFromZip(zr, "routes.txt")
	if err != nil {
		log.Fatalf("read routes.txt failed: %v", err)
	}

	routes, err := gtfs.ParseRouteFile(routesData)
	if err != nil {
		log.Fatalf("parse routes.txt failed: %v", err)
	}

	_, err = db.InsertRoutes(context.Background(), conn, routes)
	if err != nil {
		log.Fatalf("insert routes failed: %v", err)
	}

	calendarData, err := gtfs.ReadFileFromZip(zr, "calendar.txt")
	if err != nil {
		log.Fatalf("read calendar.txt failed: %v", err)
	}

	calendars, err := gtfs.ParseCalendarFile(calendarData)
	if err != nil {
		log.Fatalf("parse calendar.txt failed: %v", err)
	}

	_, err = db.InsertCalendar(context.Background(), conn, calendars)
	if err != nil {
		log.Fatalf("insert calendar failed: %v", err)
	}

	calendarDatesData, err := gtfs.ReadFileFromZip(zr, "calendar_dates.txt")
	if err != nil {
		log.Fatalf("read calendar_dates.txt failed: %v", err)
	}

	calendarDates, err := gtfs.ParseCalendarDatesFile(calendarDatesData)
	if err != nil {
		log.Fatalf("parse calendar_dates.txt failed: %v", err)
	}

	_, err = db.InsertCalendarDates(context.Background(), conn, calendarDates)
	if err != nil {
		log.Fatalf("insert calendar_dates failed: %v", err)
	}

	tripsData, err := gtfs.ReadFileFromZip(zr, "trips.txt")
	if err != nil {
		log.Fatalf("read trips.txt failed: %v", err)
	}

	trips, err := gtfs.ParseTripFile(tripsData)
	if err != nil {
		log.Fatalf("parse trips.txt failed: %v", err)
	}

	_, err = db.InsertTrips(context.Background(), conn, trips)
	if err != nil {
		log.Fatalf("insert trips failed: %v", err)
	}

	stopTimesData, err := gtfs.ReadFileFromZip(zr, "stop_times.txt")
	if err != nil {
		log.Fatalf("read stop_times.txt failed: %v", err)
	}

	stopTimes, err := gtfs.ParseStopTimeFile(stopTimesData)
	if err != nil {
		log.Fatalf("parse stop_times.txt failed: %v", err)
	}

	_, err = db.InsertStopTimes(context.Background(), conn, stopTimes)
	if err != nil {
		log.Fatalf("insert stop_times failed: %v", err)
	}
}
