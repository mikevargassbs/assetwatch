-- +goose Up
-- Second, sequential sign-off stage: Head Office / Security Manager signs
-- after the Branch Manager (the existing client_* columns) has signed.
ALTER TABLE client_acceptances
    ADD COLUMN head_office_signature_link_token TEXT,
    ADD COLUMN head_office_signature_link_expires_at TIMESTAMPTZ,
    ADD COLUMN head_office_email TEXT,
    ADD COLUMN head_office_link_emailed_at TIMESTAMPTZ,
    ADD COLUMN head_office_name TEXT,
    ADD COLUMN head_office_signed_at TIMESTAMPTZ,
    ADD COLUMN head_office_signature_data TEXT;

CREATE UNIQUE INDEX idx_client_acceptances_head_office_link_token ON client_acceptances(head_office_signature_link_token)
    WHERE head_office_signature_link_token IS NOT NULL;

-- +goose Down
DROP INDEX idx_client_acceptances_head_office_link_token;
ALTER TABLE client_acceptances
    DROP COLUMN head_office_signature_link_token,
    DROP COLUMN head_office_signature_link_expires_at,
    DROP COLUMN head_office_email,
    DROP COLUMN head_office_link_emailed_at,
    DROP COLUMN head_office_name,
    DROP COLUMN head_office_signed_at,
    DROP COLUMN head_office_signature_data;
