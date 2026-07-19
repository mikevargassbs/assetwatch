-- +goose Up
ALTER TABLE app_settings
    ADD COLUMN barcode_label_fields TEXT[] NOT NULL
    DEFAULT ARRAY['serial_number', 'mac', 'ip', 'model'];

-- +goose Down
ALTER TABLE app_settings DROP COLUMN barcode_label_fields;
