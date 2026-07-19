-- +goose Up
CREATE TABLE site_installations (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hardware_unit_id       UUID NOT NULL REFERENCES hardware_units(id) ON DELETE CASCADE,
    received_on_site_at    TIMESTAMPTZ,
    confirmed_correct      BOOLEAN,
    discrepancy_notes      TEXT,
    escalated_to_pmpc_at   TIMESTAMPTZ,
    date_installed         TIMESTAMPTZ,
    installed_by           UUID REFERENCES users(id),
    installed_at           TIMESTAMPTZ,
    inspected_by           UUID REFERENCES users(id),
    inspected_at           TIMESTAMPTZ,
    fit_focus_by           UUID REFERENCES users(id),
    fit_focus_completed_at TIMESTAMPTZ,
    network_attached       BOOLEAN,
    device_contactable     BOOLEAN,
    ping_checked_at        TIMESTAMPTZ,
    installed_location     TEXT,
    installed_height_m     NUMERIC(6, 2),
    signed_off_at          TIMESTAMPTZ,
    meta_data              JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_site_installations_unit ON site_installations(hardware_unit_id);

-- +goose Down
DROP TABLE site_installations;
