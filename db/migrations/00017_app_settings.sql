-- +goose Up
CREATE TABLE app_settings (
    id                      BOOLEAN PRIMARY KEY DEFAULT TRUE,
    barcode_label_width_mm  NUMERIC NOT NULL DEFAULT 80,
    barcode_label_height_mm NUMERIC NOT NULL DEFAULT 45,
    CONSTRAINT app_settings_singleton CHECK (id)
);

INSERT INTO app_settings (id) VALUES (TRUE);

-- +goose Down
DROP TABLE app_settings;
