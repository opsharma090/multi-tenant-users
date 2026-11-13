# Invitation schema extension

This diagram extends the original database ER diagram.

```mermaid
erDiagram
    TENANTS ||--o{ TENANT_INVITATIONS : contains
    USERS ||--o{ TENANT_INVITATIONS : invited_by
    TENANTS ||--o{ TENANT_USERS : memberships
    USERS ||--o{ TENANT_USERS : joins

    TENANT_INVITATIONS {
        bigint id PK
        bigint tenant_id FK
        varchar email
        varchar token_hash UK
        bigint invited_by FK
        timestamptz expires_at
        timestamptz accepted_at
        timestamptz revoked_at
        timestamptz created_at
    }

    TENANT_USERS {
        bigint tenant_id PK,FK
        bigint user_id PK,FK
        varchar display_name
        timestamptz deleted_at
    }
```

## Constraints and behavior

- Each invitation belongs to one tenant.
- invited_by references the global user who created it.
- Recipient email is normalized to lowercase.
- Only the invitation token hash is stored.
- Token hashes are unique.
- A partial unique index allows one pending invitation per tenant/email.
- Reinviting revokes the previous pending invitation.
- Acceptance requires an existing matching global account and its password.
- Acceptance creates or restores a tenant_users membership.
- No roles are granted automatically.
- Membership creation, invitation consumption and audit logging commit together.
- Other tenant memberships remain unchanged.