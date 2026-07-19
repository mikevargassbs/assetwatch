-- +goose Up
ALTER TABLE site_installations
    ADD COLUMN site_name       TEXT,
    ADD COLUMN site_location   TEXT,
    ADD COLUMN site_ip         INET,
    ADD COLUMN site_subnet     CIDR,
    ADD COLUMN site_gateway    INET,
    ADD COLUMN deployment_date TIMESTAMPTZ,
    ADD COLUMN deployment_team TEXT,
    ADD COLUMN team_leader     TEXT;

-- +goose Down
ALTER TABLE site_installations
    DROP COLUMN site_name,
    DROP COLUMN site_location,
    DROP COLUMN site_ip,
    DROP COLUMN site_subnet,
    DROP COLUMN site_gateway,
    DROP COLUMN deployment_date,
    DROP COLUMN deployment_team,
    DROP COLUMN team_leader;
