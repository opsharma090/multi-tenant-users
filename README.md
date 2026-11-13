# Multi-Tenant User Management API

Go + PostgreSQL REST API with tenant-scoped JWT access tokens, rotating refresh tokens, RBAC, audit logs and soft-deleted memberships.

## Requirements

- Go compatible with go.mod (development used Go 1.26.4).
- Docker Desktop with Docker Compose.
- PowerShell examples below run from the repository root.

## Start PostgreSQL

```powershell
docker compose up -d
docker compose exec postgres pg_isready -U app -d tenant_users
```

## Apply migrations to a fresh database

Run these once, in order. The migration files must be included with the source repository. The current SQL files are not a version-tracking migration runner: do not rerun successful schema migrations on an existing database.

```powershell
$ErrorActionPreference = "Stop"
$migrations = @("001_init.sql", "002_seed.sql", "003_member_profile.sql", "004_tenant_create_permission.sql", "005_permission_cache.sql")
foreach ($migration in $migrations) {
    docker compose cp "./migrations/$migration" "postgres:/tmp/$migration"
    if ($LASTEXITCODE -ne 0) { throw "Copy failed: $migration" }
    docker compose exec postgres psql -v ON_ERROR_STOP=1 -U app -d tenant_users -f "/tmp/$migration"
    if ($LASTEXITCODE -ne 0) { throw "Migration failed: $migration" }
}
```

## Run API

```powershell
$env:DATABASE_URL = "postgres://app:app_password@localhost:5432/tenant_users?sslmode=disable"
$env:JWT_SECRET = "local-assignment-secret-change-me-123456789"
go mod download
go run ./cmd/api
```

Local example credentials and secret are for development only. Use a strong externally supplied secret and TLS for deployment. Environment variables must be set in the terminal running the server.

Health: GET http://localhost:8080/health

## Seed accounts

| Tenant code | Email | Password | Initial role |
|---|---|---|---|
| alpha | admin@alpha.com | Admin@123 | admin |
| beta | admin@beta.com | Admin@123 | admin |
| alpha / beta | shared@example.com | User@123 | viewer |

Manual demonstrations may have added a manager role to the shared user in Alpha. Automated RBAC tests use a fresh user to avoid depending on this state.

## Test

Keep the API running in one terminal. In another:

```powershell
$env:TEST_API_URL = "http://localhost:8080"
go test ./... -v -count=1
```

Without TEST_API_URL, integration tests skip; middleware and service unit tests still run. Tests require active seeded Alpha and Beta tenants. They create users and soft-delete their memberships. Audit rows, global user rows and cache-test roles remain for inspection. Use a dedicated local test database.

Tests cover missing/malformed tokens, incorrect credentials, permission denial, cross-tenant user reads/updates/deletes, cross-tenant role assignment, refresh rotation/reuse, user updates and deleted-member access. The cache test additionally checks immediate grant/revocation visibility.

## Postman

Import docs/MultiTenant.postman_collection.json. Collection variables include base_url, tenant_code, email and password. Run Login first: its test script stores access_token and refresh_token. Run Refresh to rotate and replace both saved values. Access tokens expire after 15 minutes; refresh tokens after seven days. Refresh credentials belong to exactly one user/tenant pair. Do not send tenant IDs in headers or bodies.

Create User saves user_id. Create Role saves role_id. Set Role Permissions and Assign Role use those IDs. Delete User soft-deletes that user's current-tenant membership. Organization creation uses org_code; change it for each new organization. Login with that code to obtain a token for the new organization.

## Architecture and documentation

- cmd/api: HTTP wiring, process lifecycle and database pool.
- internal/{auth,user,role,tenant,audit}: handlers, services and repositories.
- internal/middleware: JWT validation, active membership and permission cache.
- internal/platform: PostgreSQL pool and JSON responses.
- migrations: schema and development seeds.
- docs/API.md: endpoint reference and status semantics.
- docs/ER-diagram.md: normalized schema and Mermaid diagram.
- docs/Design.html: two-page print layout for the design document.

Open docs/Design.html in a browser and Print -> Save as PDF using A4, 100% scale and browser headers/footers disabled.

## Design boundaries and next steps

- A user is a global identity; tenant_users holds memberships and display-name overrides. PATCH /users/{id} updates the current-tenant display name only. Email/password changes are outside this endpoint.
- POST /users creates a new global identity. Existing users can join another tenant through POST /invitations and POST /invitations/accept.
- POST /tenants is authorized organization onboarding: tenant:create in the current tenant permits creating a new organization owned by the caller. It does not permit managing arbitrary existing tenants.
- Suspending the current tenant blocks login, refresh and protected access. Reactivation currently requires an operator database action; no separate platform-admin recovery API exists.
- Roles support creation, listing, permission replacement and additive user assignment. Role rename/delete and individual assignment removal endpoints are future additions.
- Cache entries are in-process, bounded to 1,000 keys and expire after one minute. Each permission check reads the tenant's database version; triggers advance it on user-role/role-permission changes. Active membership is checked separately on every authenticated request.
- PostgreSQL and the API run through Docker Compose. Database migrations are currently applied manually. Rate limiting, secret rotation and broader concurrency tests remain next steps.
- Refresh tokens are random opaque credentials, hashed with SHA-256 at rest. Access tokens are JWTs. Rotation prevents reuse of a consumed refresh credential; family-wide replay revocation is not implemented.
- Successful authentication and domain changes are audited. Failed-login telemetry, before/after change payloads and retention/export policies are future improvements.

## Submission

Include the complete Go source, go.mod, go.sum, migrations, docker-compose.yml, tests and these documentation files in the Git/Bitbucket repository. This documentation bundle alone is not the application source.
## Docker API setup

The API and PostgreSQL both run through Docker Compose.
The API uses a multi-stage Docker build and runs as a non-root user.

### First-time setup

Create the local environment file:

```powershell
Copy-Item .env.example .env
```

If `.env` already exists, preserve it. Configure `JWT_SECRET`
with at least 32 characters.

Start PostgreSQL:

```powershell
docker compose up -d postgres
```

For a fresh database, apply these migrations once, in order:

1. migrations/001_init.sql
2. migrations/002_seed.sql
3. migrations/003_member_profile.sql
4. migrations/004_tenant_create_permission.sql
5. migrations/005_permission_cache.sql
6. migrations/006_invitations.sql

Use the manual migration commands documented earlier.
For an existing database with migrations 001–005 already applied,
run only 006_invitations.sql. Do not rerun successful migrations.

Stop any locally running `go run ./cmd/api` process to free port 8080.

Build and start the API:

```powershell
docker compose up --build -d api
```

Check containers and database connectivity:

```powershell
docker compose ps
Invoke-RestMethod -Uri "http://localhost:8080/health"
```

### Rebuild after application changes

```powershell
docker compose up --build -d api
```

### View API logs

```powershell
docker compose logs --tail=50 api
```

### Run tests

Keep the Docker API running:

```powershell
$env:TEST_API_URL = "http://localhost:8080"
go vet ./...
go test ./... -v -count=1
```

Invitation unit tests run without PostgreSQL or the API:

```powershell
go test ./internal/invitation -v
```

Integration tests verify invitation acceptance, wrong-password
rejection, token reuse rejection, membership isolation and no
automatic role grants.

### Stop containers

```powershell
docker compose down
```

This preserves the database volume.