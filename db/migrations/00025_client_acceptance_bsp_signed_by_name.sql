-- +goose Up
-- The BSP Acceptance "Signed by" field is a free-text name (the actual
-- physical signer — a Branch Manager or BSP representative on site), which
-- may differ from bsp_acceptance_by (the logged-in system account that
-- submitted the form on their behalf). Previously this typed name was
-- captured in the UI but never persisted.
ALTER TABLE client_acceptances
    ADD COLUMN bsp_signed_by_name TEXT;

-- +goose Down
ALTER TABLE client_acceptances
    DROP COLUMN bsp_signed_by_name;
