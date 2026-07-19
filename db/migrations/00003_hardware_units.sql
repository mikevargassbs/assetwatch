-- +goose Up
CREATE TYPE board_column_t AS ENUM (
    'pre_deployment', 'configuration', 'shipment',
    'installation', 'commissioning', 'completed'
);

CREATE TABLE hardware_units (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    barcode          TEXT NOT NULL UNIQUE,
    status           TEXT NOT NULL DEFAULT 'active',
    current_stage    TEXT NOT NULL DEFAULT 'receiving',
    board_column     board_column_t NOT NULL DEFAULT 'pre_deployment',
    is_exception     BOOLEAN NOT NULL DEFAULT FALSE,
    allocated_branch TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    meta_data        JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_hardware_units_board_column ON hardware_units(board_column);
CREATE INDEX idx_hardware_units_is_exception ON hardware_units(is_exception) WHERE is_exception;
CREATE INDEX idx_hardware_units_meta_data ON hardware_units USING GIN (meta_data);

CREATE TABLE custom_field_definitions (
    id          SERIAL PRIMARY KEY,
    stage       TEXT NOT NULL,
    field_key   TEXT NOT NULL,
    label       TEXT NOT NULL,
    data_type   TEXT NOT NULL DEFAULT 'text',
    is_required BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order  INTEGER NOT NULL DEFAULT 0,
    active      BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE (stage, field_key)
);

-- Seed Stage 1-A's known dynamic feature-flag fields
INSERT INTO custom_field_definitions (stage, field_key, label, data_type, is_required, sort_order) VALUES
    ('stage1a', 'axis_object_analytics_installed', 'Axis Object Analytics Installed', 'boolean', FALSE, 10),
    ('stage1a', 'axis_object_analytics_enabled', 'Axis Object Analytics Enabled', 'boolean', FALSE, 20),
    ('stage1a', 'axis_video_motion_detection_installed', 'Axis Video Motion Detection Installed', 'boolean', FALSE, 30),
    ('stage1a', 'axis_video_motion_detection_enabled', 'Axis Video Motion Detection Enabled', 'boolean', FALSE, 40),
    ('stage1a', 'axis_scene_metadata_enabled', 'Axis Scene Metadata Enabled', 'boolean', FALSE, 50),
    ('stage1a', 'certificate_installed', 'Certificate Installed', 'boolean', FALSE, 60);

-- +goose Down
DROP TABLE custom_field_definitions;
DROP TABLE hardware_units;
DROP TYPE board_column_t;
