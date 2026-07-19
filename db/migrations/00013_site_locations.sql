-- +goose Up
CREATE TABLE site_locations (
    id         SERIAL PRIMARY KEY,
    name       TEXT NOT NULL,
    active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_site_locations_name ON site_locations(lower(name));

-- +goose Down
DROP TABLE site_locations;
