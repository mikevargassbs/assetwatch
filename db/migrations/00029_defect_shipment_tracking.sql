-- +goose Up
ALTER TABLE defect_reports
    ADD COLUMN tracking_number     TEXT,
    ADD COLUMN carrier             TEXT,
    ADD COLUMN shipped_back_at     TIMESTAMPTZ,
    ADD COLUMN delivered_at        TIMESTAMPTZ,
    ADD COLUMN supplier_received_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE defect_reports
    DROP COLUMN tracking_number,
    DROP COLUMN carrier,
    DROP COLUMN shipped_back_at,
    DROP COLUMN delivered_at,
    DROP COLUMN supplier_received_at;
