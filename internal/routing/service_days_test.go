package routing

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func testConn(t *testing.T) *pgx.Conn {
	t.Helper()
	godotenv.Load("../../.env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; run `make up && make ingest-dev` for local Postgres with real data")
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { conn.Close(ctx) })
	return conn
}

func TestActiveServiceIDs_ReturnsNonEmptyForARealWeekday(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	// A Wednesday. The exact date doesn't matter as long as it's a real
	// weekday within some calendar row's start/end range — NYC subway GTFS
	// calendars are typically valid for long, ongoing windows.
	date := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)

	ids, err := activeServiceIDs(ctx, conn, date)
	if err != nil {
		t.Fatalf("activeServiceIDs: %v", err)
	}
	if len(ids) == 0 {
		t.Fatal("expected at least one active service_id for a real weekday, got none")
	}
}

func TestActiveServiceIDs_OutsideCalendarRangeReturnsEmpty(t *testing.T) {
	conn := testConn(t)
	ctx := context.Background()

	// Far in the past, before any GTFS calendar's start_date.
	date := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)

	ids, err := activeServiceIDs(ctx, conn, date)
	if err != nil {
		t.Fatalf("activeServiceIDs: %v", err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected no active services in 1990, got %v", ids)
	}
}
