-- +goose Up
CREATE INDEX idx_stop_times_stop_id_departure_time ON stop_times(stop_id, departure_time);

-- +goose Down
DROP INDEX idx_stop_times_stop_id_departure_time;
