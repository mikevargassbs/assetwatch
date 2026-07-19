-- +goose Up
CREATE TABLE goods_receipts (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hardware_unit_id   UUID NOT NULL REFERENCES hardware_units(id) ON DELETE CASCADE,
    received_by        UUID REFERENCES users(id),
    received_date      TIMESTAMPTZ NOT NULL DEFAULT now(),
    po_or_waybill_ref  TEXT,
    items_correct      BOOLEAN,
    discrepancy_notes  TEXT,
    escalated_to       TEXT,
    escalated_at       TIMESTAMPTZ,
    meta_data          JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE UNIQUE INDEX idx_goods_receipts_unit ON goods_receipts(hardware_unit_id);

-- +goose Down
DROP TABLE goods_receipts;
