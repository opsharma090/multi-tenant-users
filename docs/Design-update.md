# Design Update — Interview Feedback

## API containerization

The API now uses a multi-stage Docker build.
The build stage compiles Go source into a static binary.
The runtime stage runs the binary as a non-root user.

Docker Compose runs the API and PostgreSQL.
The API waits for PostgreSQL to become healthy before starting.
Database connectivity uses the Compose service name `postgres`.

JWT_SECRET is provided at runtime through environment configuration.
Database migrations are currently applied manually.

## Existing-user invitation flow

An active member with `user:invite` permission creates an invitation.
The target tenant comes exclusively from the inviter's verified JWT.

The service generates a random 256-bit token with a 24-hour expiry.
Only its SHA-256 hash is stored.
The raw token is returned once for secure manual delivery.
Email delivery is not configured.

The recipient accepts using the invitation token, email and password.
Password verification allows existing global users to accept without
already having an active tenant session.

Acceptance verifies recipient identity, invitation state and expiry,
active target tenant, and the inviter's active membership.

Row-level locking prevents the same invitation from being consumed twice.
Membership creation/restoration, invitation consumption and audit logging
commit in one transaction.

No roles are assigned automatically.
An authorized admin assigns roles through the existing API.
Other tenant memberships remain unchanged.

## Additional testing

Pure unit tests cover:
- Pending, expired, revoked and already accepted invitations.
- Wrong-recipient rejection and the exact expiry boundary.
- Token hashing, email normalization and 24-hour expiry.
- Wrong-password rejection before membership mutation.
- Correct-password acceptance using the verified global account.

The API integration test covers:
- Inviting an existing Alpha user into Beta.
- Wrong-password rejection without consuming the invitation.
- Successful acceptance and token reuse rejection.
- No automatic role grants.
- Preservation of the Alpha membership.
- Loss of Beta access after its membership is deleted.

## Remaining improvements

- Email delivery and invitation cancellation/listing.
- Request rate limiting.
- Automated version-tracked migrations.
- Broader concurrency and failure-path tests.