-- +goose Up
CREATE TABLE shipments (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hardware_unit_id         UUID NOT NULL REFERENCES hardware_units(id) ON DELETE CASCADE,
    site_name                TEXT,
    site_location            TEXT,
    site_ip                  INET,
    site_subnet              CIDR,
    site_gateway             INET,
    deployment_date          TIMESTAMPTZ,
    deployment_team          TEXT,
    team_leader              TEXT,
    issued_to                TEXT,
    date_checked_out         TIMESTAMPTZ,
    date_checked_in          TIMESTAMPTZ,
    shipped_via              TEXT CHECK (shipped_via IN ('air', 'sea', 'land')),
    shipping_provider        TEXT,
    waybill_number           TEXT,
    packing_list_printed_at  TIMESTAMPTZ,
    delivered_at             TIMESTAMPTZ,
    meta_data                JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_shipments_unit ON shipments(hardware_unit_id);

-- +goose Down
DROP TABLE shipments;
