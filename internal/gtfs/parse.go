package gtfs

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
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

func ParseStopFile(data []byte) ([]Stop, error) {
	r := csv.NewReader(bytes.NewReader(data))

	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	var stops []Stop
	for i, row := range rows {
		if i == 0 {
			continue
		}

		lat, err := strconv.ParseFloat(row[2], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid lat on row %d: %w", i, err)
		}

		lon, err := strconv.ParseFloat(row[3], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid lon on row %d: %w", i, err)
		}

		stops = append(stops, Stop{
			StopId:   row[0],
			StopName: row[1],
			Lat:      lat,
			Lon:      lon,
			// LocationType:  row[4],
			ParentStation: row[5],
		})
	}

	return stops, nil
}

func ParseRouteFile(data []byte) ([]Route, error) {
	r := csv.NewReader(bytes.NewReader(data))

	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	var routes []Route
	for i, row := range rows {
		if i == 0 {
			continue
		}

		routeType, err := strconv.Atoi(row[5])
		if err != nil {
			return nil, fmt.Errorf("invalid route_type on row %d: %w", i, err)
		}

		routes = append(routes, Route{
			RouteId:   row[0],
			RouteName: row[3],
			RouteType: routeType,
		})
	}

	return routes, nil
}
