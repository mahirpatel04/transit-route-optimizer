-- +goose Up
CREATE TABLE stops (
    stop_id        TEXT PRIMARY KEY,
    stop_name      TEXT NOT NULL,
    lat            DOUBLE PRECISION NOT NULL,
    lon            DOUBLE PRECISION NOT NULL,
    parent_station TEXT REFERENCES stops(stop_id)
);

CREATE TABLE routes (
    route_id    TEXT PRIMARY KEY,
    route_name  TEXT NOT NULL,
    route_type  INT NOT NULL
);

CREATE TABLE calendar (
    service_id TEXT PRIMARY KEY,
    monday     BOOLEAN NOT NULL,
    tuesday    BOOLEAN NOT NULL,
    wednesday  BOOLEAN NOT NULL,
    thursday   BOOLEAN NOT NULL,
    friday     BOOLEAN NOT NULL,
    saturday   BOOLEAN NOT NULL,
    sunday     BOOLEAN NOT NULL,
    start_date DATE NOT NULL,
    end_date   DATE NOT NULL
);

CREATE TABLE calendar_dates (
    service_id     TEXT NOT NULL REFERENCES calendar(service_id),
    date           DATE NOT NULL,
    exception_type INT NOT NULL,  -- 1 = service added, 2 = service removed
    PRIMARY KEY (service_id, date)
);

CREATE TABLE trips (
    trip_id    TEXT PRIMARY KEY,
    route_id   TEXT NOT NULL REFERENCES routes(route_id),
    service_id TEXT NOT NULL REFERENCES calendar(service_id)
);

CREATE TABLE stop_times (
    trip_id        TEXT NOT NULL REFERENCES trips(trip_id),
    stop_id        TEXT NOT NULL REFERENCES stops(stop_id),
    arrival_time   INTERVAL NOT NULL,
    departure_time INTERVAL NOT NULL,
    stop_sequence  INT NOT NULL,
    PRIMARY KEY (trip_id, stop_sequence)
);

CREATE INDEX idx_stop_times_stop_id ON stop_times(stop_id);
CREATE INDEX idx_stop_times_trip_id ON stop_times(trip_id);
CREATE INDEX idx_stops_parent_station ON stops(parent_station);

-- +goose Down
DROP TABLE stop_times;
DROP TABLE trips;
DROP TABLE calendar_dates;
DROP TABLE calendar;
DROP TABLE routes;
DROP TABLE stops;