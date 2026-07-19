-- +goose Up
ALTER TABLE hardware_units
    ADD COLUMN part_number TEXT;

-- +goose Down
ALTER TABLE hardware_units
    DROP COLUMN part_number;
