-- +goose Up
-- Comments and Final Acceptance Notes were two separate free-text fields on
-- the same form with no distinct purpose in practice — merge into one.
UPDATE client_acceptances
SET comments = TRIM(BOTH E'\n' FROM
    COALESCE(comments, '') ||
    CASE WHEN final_acceptance_notes IS NOT NULL AND final_acceptance_notes <> ''
         THEN (CASE WHEN comments IS NOT NULL AND comments <> '' THEN E'\n' ELSE '' END) || final_acceptance_notes
         ELSE '' END)
WHERE final_acceptance_notes IS NOT NULL AND final_acceptance_notes <> '';

ALTER TABLE client_acceptances
    DROP COLUMN final_acceptance_notes;

-- +goose Down
ALTER TABLE client_acceptances
    ADD COLUMN final_acceptance_notes TEXT;
