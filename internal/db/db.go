package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mahirpatel04/transit-route-optimizer/internal/gtfs"
)

// db is satisfied by both *pgx.Conn and pgx.Tx, letting the insert/truncate
// helpers run either standalone or inside a transaction.
type db interface {
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func TruncateAll(ctx context.Context, conn db) error {
	_, err := conn.Exec(ctx, "TRUNCATE stops, routes, calendar, calendar_dates, trips, stop_times RESTART IDENTITY CASCADE")
	if err != nil {
		return fmt.Errorf("failed to truncate tables: %w", err)
	}

	return nil
}

func InsertStops(ctx context.Context, conn db, stops []gtfs.Stop) (int64, error) {
	rows := make([][]any, len(stops))
	for i, s := range stops {
		var parentStation any
		if s.ParentStation == "" {
			parentStation = nil
		} else {
			parentStation = s.ParentStation
		}

		rows[i] = []any{s.StopId, s.StopName, s.Lat, s.Lon, parentStation}
	}

	copyCount, err := conn.CopyFrom(
		ctx,
		pgx.Identifier{"stops"},
		[]string{"stop_id", "stop_name", "lat", "lon", "parent_station"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to copy stops: %w", err)
	}

	return copyCount, nil
}

func InsertRoutes(ctx context.Context, conn db, routes []gtfs.Route) (int64, error) {
	rows := make([][]any, len(routes))
	for i, r := range routes {
		rows[i] = []any{r.RouteId, r.RouteName, r.RouteType}
	}

	copyCount, err := conn.CopyFrom(
		ctx,
		pgx.Identifier{"routes"},
		[]string{"route_id", "route_name", "route_type"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to copy routes: %w", err)
	}

	return copyCount, nil
}

func InsertCalendar(ctx context.Context, conn db, calendars []gtfs.Calendar) (int64, error) {
	rows := make([][]any, len(calendars))
	for i, c := range calendars {
		rows[i] = []any{
			c.ServiceId, c.Monday, c.Tuesday, c.Wednesday, c.Thursday,
			c.Friday, c.Saturday, c.Sunday, c.StartDate, c.EndDate,
		}
	}

	copyCount, err := conn.CopyFrom(
		ctx,
		pgx.Identifier{"calendar"},
		[]string{
			"service_id", "monday", "tuesday", "wednesday", "thursday",
			"friday", "saturday", "sunday", "start_date", "end_date",
		},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to copy calendar: %w", err)
	}

	return copyCount, nil
}

func InsertCalendarDates(ctx context.Context, conn db, dates []gtfs.CalendarDate) (int64, error) {
	rows := make([][]any, len(dates))
	for i, d := range dates {
		rows[i] = []any{d.ServiceId, d.Date, d.ExceptionType}
	}

	copyCount, err := conn.CopyFrom(
		ctx,
		pgx.Identifier{"calendar_dates"},
		[]string{"service_id", "date", "exception_type"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to copy calendar_dates: %w", err)
	}

	return copyCount, nil
}

func InsertTrips(ctx context.Context, conn db, trips []gtfs.Trip) (int64, error) {
	rows := make([][]any, len(trips))
	for i, t := range trips {
		rows[i] = []any{t.TripId, t.RouteId, t.ServiceId}
	}

	copyCount, err := conn.CopyFrom(
		ctx,
		pgx.Identifier{"trips"},
		[]string{"trip_id", "route_id", "service_id"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to copy trips: %w", err)
	}

	return copyCount, nil
}

func InsertStopTimes(ctx context.Context, conn db, stopTimes []gtfs.StopTime) (int64, error) {
	rows := make([][]any, len(stopTimes))
	for i, st := range stopTimes {
		rows[i] = []any{st.TripId, st.StopId, st.ArrivalTime, st.DepartureTime, st.StopSequence}
	}

	copyCount, err := conn.CopyFrom(
		ctx,
		pgx.Identifier{"stop_times"},
		[]string{"trip_id", "stop_id", "arrival_time", "departure_time", "stop_sequence"},
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to copy stop_times: %w", err)
	}

	return copyCount, nil
}
