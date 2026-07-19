-- +goose Up
-- Barcode stickers get reused across physically distinct units in the field
-- (misprints, relabeling), so uniqueness can no longer be enforced on
-- barcode. serial_number (see 00012_hardware_unit_identity.sql) is the
-- actual unique identity column now.
ALTER TABLE hardware_units DROP CONSTRAINT hardware_units_barcode_key;

-- +goose Down
ALTER TABLE hardware_units ADD CONSTRAINT hardware_units_barcode_key UNIQUE (barcode);
