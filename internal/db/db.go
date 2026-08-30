package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/mahirpatel04/transit-route-optimizer/internal/gtfs"
)

func InsertStops(ctx context.Context, conn *pgx.Conn, stops []gtfs.Stop) (int64, error) {
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
