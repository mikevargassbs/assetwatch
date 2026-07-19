-- +goose Up
CREATE TABLE firmware_configurations (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hardware_unit_id     UUID NOT NULL REFERENCES hardware_units(id) ON DELETE CASCADE,
    configured_by        UUID REFERENCES users(id),
    configured_date      TIMESTAMPTZ,
    firmware_updated     BOOLEAN NOT NULL DEFAULT FALSE,
    firmware_version     TEXT,
    configuration_qc_by  UUID REFERENCES users(id),
    qc_date              TIMESTAMPTZ,
    signed_off_at        TIMESTAMPTZ,
    meta_data            JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_firmware_configurations_unit ON firmware_configurations(hardware_unit_id);

CREATE TABLE store_custody_logs (
    id               BIGSERIAL PRIMARY KEY,
    hardware_unit_id UUID NOT NULL REFERENCES hardware_units(id) ON DELETE CASCADE,
    action           TEXT NOT NULL CHECK (action IN ('checked_out', 'checked_in')),
    by_user          UUID REFERENCES users(id),
    at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    purpose          TEXT NOT NULL CHECK (purpose IN ('for_configuration', 'for_dispatch')),
    notes            TEXT
);

CREATE INDEX idx_store_custody_logs_unit ON store_custody_logs(hardware_unit_id);

-- +goose Down
DROP TABLE store_custody_logs;
DROP TABLE firmware_configurations;
