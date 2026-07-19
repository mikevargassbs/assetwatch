-- +goose Up
CREATE SEQUENCE delivery_docket_seq;

CREATE TABLE delivery_dockets (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    docket_number      TEXT NOT NULL UNIQUE DEFAULT ('DOC-' || lpad(nextval('delivery_docket_seq')::text, 6, '0')),
    waybill_number     TEXT,
    shipped_via        TEXT CHECK (shipped_via IN ('air', 'sea', 'land', 'others')),
    shipping_provider  TEXT,
    destination_site   TEXT,
    status             TEXT NOT NULL DEFAULT 'draft'
                           CHECK (status IN ('draft', 'dispatched', 'in_transit', 'arrived_at_site', 'delivered')),
    dispatched_by      TEXT,
    dispatched_at      TIMESTAMPTZ,
    received_by        TEXT,
    received_at        TIMESTAMPTZ,
    notes              TEXT,
    meta_data          JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by         UUID REFERENCES users(id),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE delivery_docket_items (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    docket_id              UUID NOT NULL REFERENCES delivery_dockets(id) ON DELETE CASCADE,
    hardware_unit_id       UUID NOT NULL REFERENCES hardware_units(id),
    serial_number_snapshot TEXT,
    added_by               UUID REFERENCES users(id),
    added_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (docket_id, hardware_unit_id)
);

CREATE INDEX idx_delivery_docket_items_unit ON delivery_docket_items(hardware_unit_id);

CREATE TABLE delivery_docket_tracking_events (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    docket_id            UUID NOT NULL REFERENCES delivery_dockets(id) ON DELETE CASCADE,
    status               TEXT,
    description          TEXT NOT NULL,
    occurred_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    is_system_generated  BOOLEAN NOT NULL DEFAULT false,
    created_by           UUID REFERENCES users(id),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_delivery_docket_tracking_events_docket ON delivery_docket_tracking_events(docket_id, occurred_at);

-- +goose Down
DROP TABLE delivery_docket_tracking_events;
DROP TABLE delivery_docket_items;
DROP TABLE delivery_dockets;
DROP SEQUENCE delivery_docket_seq;
