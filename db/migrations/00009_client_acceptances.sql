-- +goose Up
CREATE TABLE client_acceptances (
    id                             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hardware_unit_id               UUID NOT NULL REFERENCES hardware_units(id) ON DELETE CASCADE,
    method                         TEXT CHECK (method IN ('e_signature', 'manual_upload')),
    bsp_acceptance_by              UUID REFERENCES users(id),
    bsp_signature                  TEXT,
    bsp_acceptance_date            TIMESTAMPTZ,
    client_signature_link_token    TEXT,
    client_signature_link_expires_at TIMESTAMPTZ,
    client_name                    TEXT,
    client_signed_at               TIMESTAMPTZ,
    client_signature_data          TEXT,
    uploaded_document_path         TEXT,
    comments                       TEXT,
    final_acceptance_notes         TEXT,
    signed_off_at                  TIMESTAMPTZ,
    meta_data                      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at                     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_client_acceptances_unit ON client_acceptances(hardware_unit_id);
CREATE UNIQUE INDEX idx_client_acceptances_link_token ON client_acceptances(client_signature_link_token)
    WHERE client_signature_link_token IS NOT NULL;

-- +goose Down
DROP TABLE client_acceptances;
