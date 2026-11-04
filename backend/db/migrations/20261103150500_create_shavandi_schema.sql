-- +goose Up
CREATE SCHEMA shavandi;

-- +goose Down
DROP SCHEMA shavandi RESTRICT;
