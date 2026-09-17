package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mahirpatel04/transit-route-optimizer/internal/gtfs"
)

// db is satisfied by both *pgx.Conn and pgx.Tx, letting the insert/truncate
// helpers run either standalone or inside a transaction.
type db interface {
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func TruncateAll(ctx context.Context, conn db) error {
	_, err := conn.Exec(ctx, "TRUNCATE stops, routes, calendar, calendar_dates, trips, stop_times RESTART IDENTITY CASCADE")
	if err != nil {
		return fmt.Errorf("failed to truncate tables: %w", err)
	}

	return nil
}

// SetLastFetchTime records the given time as the most recent GTFS fetch,
// overwriting the single row in ingest_state.
func SetLastFetchTime(ctx context.Context, conn db, t time.Time) error {
	_, err := conn.Exec(ctx, `
		INSERT INTO ingest_state (id, last_fetch_time) VALUES (1, $1)
		ON CONFLICT (id) DO UPDATE SET last_fetch_time = EXCLUDED.last_fetch_time
	`, t)
	if err != nil {
		return fmt.Errorf("failed to set last fetch time: %w", err)
	}

	return nil
}

func GetLastFetchTime(ctx context.Context, conn db) (time.Time, error) {
	var t time.Time
	err := conn.QueryRow(ctx, "SELECT last_fetch_time FROM ingest_state WHERE id = 1").Scan(&t)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get last fetch time: %w", err)
	}
	return t, nil
}

func GetRoutes(ctx context.Context, conn db) ([]gtfs.Route, error) {
	rows, err := conn.Query(ctx, "SELECT route_id, route_name, route_type FROM routes")
	if err != nil {
		return nil, fmt.Errorf("failed to query routes: %w", err)
	}
	defer rows.Close()

	var routes []gtfs.Route
	for rows.Next() {
		var r gtfs.Route
		if err := rows.Scan(&r.RouteId, &r.RouteName, &r.RouteType); err != nil {
			return nil, fmt.Errorf("failed to scan route: %w", err)
		}
		routes = append(routes, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed reading routes: %w", err)
	}
	return routes, nil
}

func GetAllStops(ctx context.Context, conn db) ([]gtfs.Stop, error) {
	rows, err := conn.Query(ctx, "SELECT stop_id, stop_name, lat, lon, parent_station FROM stops")
	if err != nil {
		return nil, fmt.Errorf("failed to query stops: %w", err)
	}
	defer rows.Close()

	var stops []gtfs.Stop
	for rows.Next() {
		var s gtfs.Stop
		var parentStation *string
		if err := rows.Scan(&s.StopId, &s.StopName, &s.Lat, &s.Lon, &parentStation); err != nil {
			return nil, fmt.Errorf("failed to scan stop: %w", err)
		}
		if parentStation != nil {
			s.ParentStation = *parentStation
		}
		stops = append(stops, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed reading stops: %w", err)
	}
	return stops, nil
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
