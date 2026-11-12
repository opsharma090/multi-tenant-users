BEGIN;

-- Local demo passwords ke bcrypt hashes generate karega.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

INSERT INTO tenants (name, code)
VALUES
    ('Organization Alpha', 'alpha'),
    ('Organization Beta', 'beta');

INSERT INTO users (name, email, password_hash)
VALUES
    ('Alpha Admin', 'admin@alpha.com',
        crypt('Admin@123', gen_salt('bf', 10))),
    ('Beta Admin', 'admin@beta.com',
        crypt('Admin@123', gen_salt('bf', 10))),
    ('Shared User', 'shared@example.com',
        crypt('User@123', gen_salt('bf', 10)));

INSERT INTO tenant_users (tenant_id, user_id)
SELECT t.id, u.id
FROM tenants t
JOIN users u ON
    (t.code = 'alpha' AND u.email = 'admin@alpha.com')
    OR (t.code = 'beta' AND u.email = 'admin@beta.com')
    OR u.email = 'shared@example.com';

INSERT INTO permissions (code)
VALUES
    ('tenant:read'),
    ('tenant:update'),
    ('user:create'),
    ('user:read'),
    ('user:update'),
    ('user:delete'),
    ('role:create'),
    ('role:read'),
    ('role:update'),
    ('role:delete'),
    ('role:assign'),
    ('permission:read'),
    ('permission:assign'),
    ('audit:read');

INSERT INTO roles (tenant_id, name)
SELECT id, 'admin' FROM tenants;

INSERT INTO roles (tenant_id, name)
SELECT id, 'viewer' FROM tenants;

INSERT INTO role_permissions (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'admin';

INSERT INTO role_permissions (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'viewer'
  AND p.code IN ('user:read', 'role:read');

INSERT INTO user_roles (tenant_id, user_id, role_id)
SELECT tu.tenant_id, tu.user_id, r.id
FROM tenant_users tu
JOIN users u ON u.id = tu.user_id
JOIN roles r ON r.tenant_id = tu.tenant_id
WHERE
    (u.email IN ('admin@alpha.com', 'admin@beta.com')
        AND r.name = 'admin')
    OR
    (u.email = 'shared@example.com' AND r.name = 'viewer');

COMMIT;