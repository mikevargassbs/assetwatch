-- +goose Up
CREATE TABLE installation_photos (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hardware_unit_id UUID NOT NULL REFERENCES hardware_units(id) ON DELETE CASCADE,
    file_path        TEXT NOT NULL,
    uploaded_by      UUID REFERENCES users(id),
    uploaded_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_installation_photos_unit ON installation_photos(hardware_unit_id);

-- +goose Down
DROP TABLE installation_photos;
