-- +goose Up
CREATE TABLE defect_reports (
    id                             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hardware_unit_id               UUID NOT NULL REFERENCES hardware_units(id) ON DELETE CASCADE,
    declared_by                    UUID REFERENCES users(id),
    declared_date                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    defect_type                    TEXT NOT NULL CHECK (defect_type IN ('defective', 'damaged', 'wrong_item')),
    description                    TEXT,
    report_generated_at            TIMESTAMPTZ,
    emailed_to_supplier_at         TIMESTAMPTZ,
    supplier_email                 TEXT,
    replacement_status             TEXT NOT NULL DEFAULT 'pending'
        CHECK (replacement_status IN ('pending', 'shipped_back', 'replacement_received')),
    replacement_hardware_unit_id   UUID REFERENCES hardware_units(id),
    meta_data                      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at                     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_defect_reports_unit ON defect_reports(hardware_unit_id);

-- +goose Down
DROP TABLE defect_reports;
