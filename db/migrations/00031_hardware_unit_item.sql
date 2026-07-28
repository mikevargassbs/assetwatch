-- +goose Up
ALTER TABLE hardware_units
    ADD COLUMN item_id INTEGER REFERENCES items(id);

-- +goose Down
ALTER TABLE hardware_units
    DROP COLUMN item_id;
