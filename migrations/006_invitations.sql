BEGIN;

CREATE TABLE tenant_invitations (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL REFERENCES tenants(id),
    email VARCHAR(255) NOT NULL CHECK (email = LOWER(email)),
    token_hash VARCHAR(64) NOT NULL UNIQUE,
    invited_by BIGINT NOT NULL REFERENCES users(id),
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CHECK (expires_at > created_at),
    CHECK (accepted_at IS NULL OR revoked_at IS NULL)
);

CREATE INDEX idx_invitations_tenant_email
    ON tenant_invitations (tenant_id, email);

CREATE UNIQUE INDEX idx_invitations_pending
    ON tenant_invitations (tenant_id, email)
    WHERE accepted_at IS NULL AND revoked_at IS NULL;

INSERT INTO permissions (code)
VALUES ('user:invite')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM roles r
JOIN permissions p ON p.code = 'user:invite'
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

COMMIT;