package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/mahirpatel04/transit-route-optimizer/internal/gtfs"
)

func testConn(t *testing.T) *pgx.Conn {
	t.Helper()
	godotenv.Load("../../.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; run `make up` for local Postgres")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { conn.Close(ctx) })

	if err := TruncateAll(ctx, conn); err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return conn
}

func TestGetLastFetchTime(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	want := time.Now().UTC().Truncate(time.Second)
	if err := SetLastFetchTime(ctx, conn, want); err != nil {
		t.Fatalf("SetLastFetchTime: %v", err)
	}

	got, err := GetLastFetchTime(ctx, conn)
	if err != nil {
		t.Fatalf("GetLastFetchTime: %v", err)
	}
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGetRoutes(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	want := []gtfs.Route{
		{RouteId: "A", RouteName: "8th Avenue Express", RouteType: 1},
		{RouteId: "B", RouteName: "6th Avenue Express", RouteType: 1},
	}
	if _, err := InsertRoutes(ctx, conn, want); err != nil {
		t.Fatalf("InsertRoutes: %v", err)
	}

	got, err := GetRoutes(ctx, conn)
	if err != nil {
		t.Fatalf("GetRoutes: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d routes, want %d", len(got), len(want))
	}
}

func TestGetAllStops(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	want := []gtfs.Stop{
		{StopId: "127", StopName: "Times Sq-42 St", Lat: 40.7549, Lon: -73.9871},
		{StopId: "635", StopName: "Grand Central-42 St", Lat: 40.7527, Lon: -73.9772},
	}
	if _, err := InsertStops(ctx, conn, want); err != nil {
		t.Fatalf("InsertStops: %v", err)
	}

	got, err := GetAllStops(ctx, conn)
	if err != nil {
		t.Fatalf("GetAllStops: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d stops, want %d", len(got), len(want))
	}
}
