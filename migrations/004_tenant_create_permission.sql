BEGIN;

INSERT INTO permissions (code)
VALUES ('tenant:create')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (tenant_id, role_id, permission_id)
SELECT r.tenant_id, r.id, p.id
FROM roles r
JOIN permissions p ON p.code = 'tenant:create'
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

COMMIT;