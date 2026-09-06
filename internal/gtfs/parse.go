package gtfs

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"
)

type Stop struct {
	StopId   string
	StopName string
	Lat      float64
	Lon      float64
	// LocationType  string
	ParentStation string
}

type Route struct {
	RouteId   string
	RouteName string
	RouteType int
}

type Calendar struct {
	ServiceId string
	Monday    bool
	Tuesday   bool
	Wednesday bool
	Thursday  bool
	Friday    bool
	Saturday  bool
	Sunday    bool
	StartDate time.Time
	EndDate   time.Time
}

type CalendarDate struct {
	ServiceId     string
	Date          time.Time
	ExceptionType int
}

type Trip struct {
	TripId    string
	RouteId   string
	ServiceId string
}

type StopTime struct {
	TripId        string
	StopId        string
	ArrivalTime   time.Duration
	DepartureTime time.Duration
	StopSequence  int
}

// header maps a GTFS column name to its index in a CSV row.
type header map[string]int

func newHeader(row []string) header {
	h := make(header, len(row))
	for i, name := range row {
		h[name] = i
	}
	return h
}

func (h header) get(row []string, name string) string {
	i, ok := h[name]
	if !ok || i >= len(row) {
		return ""
	}
	return row[i]
}

func readRows(data []byte) ([][]string, error) {
	r := csv.NewReader(bytes.NewReader(data))

	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	return rows, nil
}

func parseGTFSDate(s string) (time.Time, error) {
	return time.Parse("20060102", s)
}

func parseGTFSBool(s string) (bool, error) {
	switch s {
	case "1":
		return true, nil
	case "0":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q", s)
	}
}

func parseGTFSTime(s string) (time.Duration, error) {
	var h, m, sec int
	_, err := fmt.Sscanf(s, "%d:%d:%d", &h, &m, &sec)
	if err != nil {
		return 0, fmt.Errorf("invalid time %q: %w", s, err)
	}

	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second, nil
}

func ParseStopFile(data []byte) ([]Stop, error) {
	rows, err := readRows(data)
	if err != nil {
		return nil, err
	}

	h := newHeader(rows[0])

	var stops []Stop
	for i, row := range rows[1:] {
		lat, err := strconv.ParseFloat(h.get(row, "stop_lat"), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid lat on row %d: %w", i+1, err)
		}

		lon, err := strconv.ParseFloat(h.get(row, "stop_lon"), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid lon on row %d: %w", i+1, err)
		}

		stops = append(stops, Stop{
			StopId:        h.get(row, "stop_id"),
			StopName:      h.get(row, "stop_name"),
			Lat:           lat,
			Lon:           lon,
			ParentStation: h.get(row, "parent_station"),
		})
	}

	return stops, nil
}

func ParseRouteFile(data []byte) ([]Route, error) {
	rows, err := readRows(data)
	if err != nil {
		return nil, err
	}

	h := newHeader(rows[0])

	var routes []Route
	for i, row := range rows[1:] {
		routeType, err := strconv.Atoi(h.get(row, "route_type"))
		if err != nil {
			return nil, fmt.Errorf("invalid route_type on row %d: %w", i+1, err)
		}

		routes = append(routes, Route{
			RouteId:   h.get(row, "route_id"),
			RouteName: h.get(row, "route_long_name"),
			RouteType: routeType,
		})
	}

	return routes, nil
}

func ParseCalendarFile(data []byte) ([]Calendar, error) {
	rows, err := readRows(data)
	if err != nil {
		return nil, err
	}

	h := newHeader(rows[0])

	var calendars []Calendar
	for i, row := range rows[1:] {
		days := make([]bool, 7)
		for di, name := range []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"} {
			v, err := parseGTFSBool(h.get(row, name))
			if err != nil {
				return nil, fmt.Errorf("invalid %s on row %d: %w", name, i+1, err)
			}
			days[di] = v
		}

		startDate, err := parseGTFSDate(h.get(row, "start_date"))
		if err != nil {
			return nil, fmt.Errorf("invalid start_date on row %d: %w", i+1, err)
		}

		endDate, err := parseGTFSDate(h.get(row, "end_date"))
		if err != nil {
			return nil, fmt.Errorf("invalid end_date on row %d: %w", i+1, err)
		}

		calendars = append(calendars, Calendar{
			ServiceId: h.get(row, "service_id"),
			Monday:    days[0],
			Tuesday:   days[1],
			Wednesday: days[2],
			Thursday:  days[3],
			Friday:    days[4],
			Saturday:  days[5],
			Sunday:    days[6],
			StartDate: startDate,
			EndDate:   endDate,
		})
	}

	return calendars, nil
}

func ParseCalendarDatesFile(data []byte) ([]CalendarDate, error) {
	rows, err := readRows(data)
	if err != nil {
		return nil, err
	}

	h := newHeader(rows[0])

	var dates []CalendarDate
	for i, row := range rows[1:] {
		date, err := parseGTFSDate(h.get(row, "date"))
		if err != nil {
			return nil, fmt.Errorf("invalid date on row %d: %w", i+1, err)
		}

		exceptionType, err := strconv.Atoi(h.get(row, "exception_type"))
		if err != nil {
			return nil, fmt.Errorf("invalid exception_type on row %d: %w", i+1, err)
		}

		dates = append(dates, CalendarDate{
			ServiceId:     h.get(row, "service_id"),
			Date:          date,
			ExceptionType: exceptionType,
		})
	}

	return dates, nil
}

func ParseTripFile(data []byte) ([]Trip, error) {
	rows, err := readRows(data)
	if err != nil {
		return nil, err
	}

	h := newHeader(rows[0])

	var trips []Trip
	for _, row := range rows[1:] {
		trips = append(trips, Trip{
			TripId:    h.get(row, "trip_id"),
			RouteId:   h.get(row, "route_id"),
			ServiceId: h.get(row, "service_id"),
		})
	}

	return trips, nil
}

func ParseStopTimeFile(data []byte) ([]StopTime, error) {
	rows, err := readRows(data)
	if err != nil {
		return nil, err
	}

	h := newHeader(rows[0])

	var stopTimes []StopTime
	for i, row := range rows[1:] {
		arrival, err := parseGTFSTime(h.get(row, "arrival_time"))
		if err != nil {
			return nil, fmt.Errorf("invalid arrival_time on row %d: %w", i+1, err)
		}

		departure, err := parseGTFSTime(h.get(row, "departure_time"))
		if err != nil {
			return nil, fmt.Errorf("invalid departure_time on row %d: %w", i+1, err)
		}

		stopSequence, err := strconv.Atoi(h.get(row, "stop_sequence"))
		if err != nil {
			return nil, fmt.Errorf("invalid stop_sequence on row %d: %w", i+1, err)
		}

		stopTimes = append(stopTimes, StopTime{
			TripId:        h.get(row, "trip_id"),
			StopId:        h.get(row, "stop_id"),
			ArrivalTime:   arrival,
			DepartureTime: departure,
			StopSequence:  stopSequence,
		})
	}

	return stopTimes, nil
}
