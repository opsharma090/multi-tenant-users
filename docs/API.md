# API reference

Protected requests use `Authorization: Bearer <access_token>`. Tenant identity comes exclusively from the verified JWT. All request examples are in the Postman collection.

| Method | Endpoint | Authorization / behavior | Success |
|---|---|---|---|
=> GET | `/health` | Public database readiness | 200 |
=> POST | `/auth/login` | Authenticate in one active tenant | 200 |
=> POST | `/auth/refresh` | Rotate a refresh token; old token returns 401 on reuse | 200 |
=> GET | `/me` | Any active authenticated member | 200 |
=> GET | `/permission-check` | user:create; demonstration endpoint | 200 |
=> GET | `/users?page=1&limit=20&search=` | user:read | 200 |
=> POST | `/users` | user:create; new identity only | 201 |
=> GET | `/users/{{user_id}}` | user:read | 200 |
=> PATCH | `/users/{{user_id}}` | user:update; tenant display name only | 200 |
=> GET | `/roles?page=1&limit=20&search=` | role:read | 200 |
=> POST | `/roles` | role:create | 201 |
=> GET | `/permissions` | permission:read | 200 |
=> PUT | `/roles/{{role_id}}/permissions` | permission:assign; replaces full list; [] clears it | 204 |
=> PUT | `/users/{{user_id}}/roles/{{role_id}}` | role:assign; additive and idempotent | 204 |
=> GET | `/tenant` | tenant:read | 200 |
=> PATCH | `/tenant` | tenant:update; both name and status required | 200 |
=> POST | `/tenants` | tenant:create; caller becomes new organization admin | 201 |
=> GET | `/audit-logs?page=1&limit=20` | audit:read; optional exact action filter | 200 |
=> DELETE | `/users/{{user_id}}` | user:delete; soft-delete membership, revoke its refresh tokens and remove its roles | 204 |

## Errors

```json
{"error":{"code":"forbidden","message":"Permission denied"}}
```

- 400: malformed input, invalid fields or pagination, unknown permission codes.
- 401: missing/invalid/expired token, invalid credentials/tenant, inactive membership or refresh credential reuse.
- 403: valid active identity without the required permission.
- 404: tenant-owned resource missing or belonging to another tenant (after authorization).
- 409: duplicate email, duplicate tenant code, duplicate role name in a tenant, or self-membership deletion.
- 500: internal processing/database error. Health can return 503 when the database is unavailable.

List endpoints use `data` arrays and `pagination: {page, limit}`; page defaults to 1, limit to 20, maximum limit 100 and maximum page 1,000,000. Users and roles support `search`; audit logs support exact `action`, e.g. `auth:login`. Permissions are a small unpaginated global catalog. Lists do not currently include total counts.

Create/get/update domain responses use `{"data": ...}`. Authentication responses contain access_token, refresh_token, token_type and expires_in. Identity returns user_id and tenant_id directly. Successful 204 responses have no JSON body.

POST user fields: name, email, password (8-72 bytes). PATCH user: name only. POST role: name (normalized lowercase). PUT permissions: permissions array; an empty array clears the role. PATCH tenant: name and status (active/suspended). POST tenant: name and unique code (normalized lowercase; 2-50 letters/digits/hyphens, starting with a letter/digit). Unknown JSON properties are rejected on these write endpoints.

The token-scoped `/tenant` endpoint manages only the current organization. `/tenants` is organization onboarding; log in with the new organization's code to switch scope. There is no client-controlled tenant override.

## Existing-user invitations

### POST /invitations

Requires an active tenant-scoped JWT and `user:invite` permission.

Request:

{
  "email": "admin@beta.com"
}

Returns HTTP 201. Response `data` contains:
- id
- tenant_id
- email
- expires_at
- invitation_token

The target tenant comes from the inviter's verified JWT.
The invitation expires after 24 hours.
Only the token hash is stored in the database.
The raw token is returned once for secure manual delivery.
Email delivery is not configured.

Reinviting revokes the previous pending invitation.
Inviting an existing active member returns HTTP 409.

### POST /invitations/accept

Requires the invitation token and the recipient's own credentials.
A tenant access token is not required.

Request:

{
  "invitation_token": "<token received from inviter>",
  "email": "admin@beta.com",
  "password": "<recipient password>"
}

Returns HTTP 200:

{
  "data": {
    "tenant_id": 1,
    "user_id": 2
  }
}

The existing global account joins the invited tenant.
No new global user is created.
No roles are granted automatically.
An authorized admin assigns roles separately.

Acceptance consumes the invitation in the same transaction as
membership creation and audit logging.

Expired, revoked, already accepted or wrong-recipient invitations
are rejected. Wrong credentials return HTTP 401.
An already-active membership returns HTTP 409.

Soft-deleted memberships can rejoin through a new invitation,
without restoring previous roles or refresh sessions.
Other tenant memberships remain unchanged.

After acceptance, log in with the target tenant code to obtain
tenant-scoped access and refresh tokens.