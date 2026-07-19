-- +goose Up
ALTER TABLE client_acceptances
    ADD COLUMN client_email TEXT,
    ADD COLUMN client_link_emailed_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE client_acceptances
    DROP COLUMN client_email,
    DROP COLUMN client_link_emailed_at;
