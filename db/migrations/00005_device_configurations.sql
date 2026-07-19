-- +goose Up
CREATE TABLE device_configurations (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hardware_unit_id     UUID NOT NULL REFERENCES hardware_units(id) ON DELETE CASCADE,
    device_make          TEXT,
    device_model         TEXT,
    device_name_dns      TEXT,
    client_ip_address    INET,
    serial_number        TEXT,
    mac_address          MACADDR,
    dns_server_1         INET,
    dns_server_2         INET,
    ntp_server           TEXT,
    frequency_hz         INTEGER DEFAULT 50,
    default_username     TEXT DEFAULT 'root',
    default_password_enc TEXT,
    encoded_by           UUID REFERENCES users(id),
    encoded_at           TIMESTAMPTZ,
    configured_by        UUID REFERENCES users(id),
    configured_at        TIMESTAMPTZ,
    qc_by                UUID REFERENCES users(id),
    qc_at                TIMESTAMPTZ,
    barcode_printed_at   TIMESTAMPTZ,
    signed_off_at        TIMESTAMPTZ,
    meta_data            JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_device_configurations_unit ON device_configurations(hardware_unit_id);
CREATE INDEX idx_device_configurations_meta_data ON device_configurations USING GIN (meta_data);

-- +goose Down
DROP TABLE device_configurations;
