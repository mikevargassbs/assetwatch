-- +goose Up
ALTER TABLE hardware_units
    ADD COLUMN alias TEXT,
    ADD COLUMN serial_number TEXT,
    ADD COLUMN device_make TEXT,
    ADD COLUMN device_model TEXT;

CREATE UNIQUE INDEX idx_hardware_units_serial_number ON hardware_units(serial_number) WHERE serial_number IS NOT NULL;

-- +goose Down
DROP INDEX idx_hardware_units_serial_number;

ALTER TABLE hardware_units
    DROP COLUMN alias,
    DROP COLUMN serial_number,
    DROP COLUMN device_make,
    DROP COLUMN device_model;
