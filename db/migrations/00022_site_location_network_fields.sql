-- +goose Up
ALTER TABLE site_locations
    ADD COLUMN region      TEXT,
    ADD COLUMN ip_gateway  TEXT,
    ADD COLUMN subnet_mask TEXT;

-- +goose Down
ALTER TABLE site_locations
    DROP COLUMN region,
    DROP COLUMN ip_gateway,
    DROP COLUMN subnet_mask;
