-- +goose Up
CREATE TABLE ingest_state (
    id              INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    last_fetch_time TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE ingest_state;
