package routing

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type calendarRow struct {
	serviceID string
	weekday   [7]bool // index 0 = Sunday, matching time.Weekday
	startDate time.Time
	endDate   time.Time
}

// activeServiceIDs returns every service_id active on the given calendar
// date, applying calendar_dates exceptions (1 = added, 2 = removed) on top
// of the base weekday/date-range rule from calendar.
func activeServiceIDs(ctx context.Context, conn *pgx.Conn, date time.Time) ([]string, error) {
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)

	rows, err := conn.Query(ctx, `
		SELECT service_id, sunday, monday, tuesday, wednesday, thursday, friday, saturday,
		       start_date, end_date
		FROM calendar
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query calendar: %w", err)
	}
	defer rows.Close()

	active := make(map[string]bool)
	for rows.Next() {
		var c calendarRow
		if err := rows.Scan(
			&c.serviceID,
			&c.weekday[0], &c.weekday[1], &c.weekday[2], &c.weekday[3],
			&c.weekday[4], &c.weekday[5], &c.weekday[6],
			&c.startDate, &c.endDate,
		); err != nil {
			return nil, fmt.Errorf("failed to scan calendar row: %w", err)
		}
		inRange := !date.Before(c.startDate) && !date.After(c.endDate)
		if inRange && c.weekday[int(date.Weekday())] {
			active[c.serviceID] = true
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed reading calendar: %w", err)
	}

	exceptionRows, err := conn.Query(ctx, `
		SELECT service_id, exception_type FROM calendar_dates WHERE date = $1
	`, date)
	if err != nil {
		return nil, fmt.Errorf("failed to query calendar_dates: %w", err)
	}
	defer exceptionRows.Close()

	for exceptionRows.Next() {
		var serviceID string
		var exceptionType int
		if err := exceptionRows.Scan(&serviceID, &exceptionType); err != nil {
			return nil, fmt.Errorf("failed to scan calendar_dates row: %w", err)
		}
		switch exceptionType {
		case 1:
			active[serviceID] = true
		case 2:
			delete(active, serviceID)
		}
	}
	if err := exceptionRows.Err(); err != nil {
		return nil, fmt.Errorf("failed reading calendar_dates: %w", err)
	}

	ids := make([]string, 0, len(active))
	for id := range active {
		ids = append(ids, id)
	}
	return ids, nil
}
