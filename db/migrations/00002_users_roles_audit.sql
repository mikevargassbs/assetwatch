-- +goose Up
CREATE TABLE roles (
    id          SERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT ''
);

INSERT INTO roles (name, description) VALUES
    ('admin', 'Full access, user management, field-definition management'),
    ('pm_pc', 'Receiving verification, branch allocation, store custody, site-discrepancy escalation'),
    ('encoder', 'Stage 1-A data entry'),
    ('configurator', 'Stage 1-B data entry'),
    ('qc', 'Quality control sign-off (Stage 1-A & 1-B)'),
    ('logistics', 'Stage 2 logistics'),
    ('field_technician', 'Stage 3 site installation'),
    ('bsp_acceptance_officer', 'Stage 4 internal acceptance'),
    ('reports_viewer', 'Read-only reporting access');

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    full_name     TEXT NOT NULL,
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT users_email_domain_chk CHECK (email ILIKE '%@sbs.com.pg')
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id INTEGER NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);

CREATE TABLE audit_trails (
    id            BIGSERIAL PRIMARY KEY,
    entity_type   TEXT NOT NULL,
    entity_id     TEXT NOT NULL,
    action        TEXT NOT NULL,
    performed_by  UUID REFERENCES users(id),
    performed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    old_value     JSONB,
    new_value     JSONB,
    ip_address    TEXT,
    notes         TEXT
);

CREATE INDEX idx_audit_trails_entity ON audit_trails(entity_type, entity_id);
CREATE INDEX idx_audit_trails_performed_at ON audit_trails(performed_at);

-- +goose Down
DROP TABLE audit_trails;
DROP TABLE refresh_tokens;
DROP TABLE user_roles;
DROP TABLE users;
DROP TABLE roles;
