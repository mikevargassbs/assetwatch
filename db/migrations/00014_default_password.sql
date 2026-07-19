-- +goose Up
ALTER TABLE device_configurations RENAME COLUMN default_password_enc TO default_password;

-- +goose Down
ALTER TABLE device_configurations RENAME COLUMN default_password TO default_password_enc;
