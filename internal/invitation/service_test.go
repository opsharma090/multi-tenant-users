package invitation

import (
	"context"
	"testing"
	"time"
)

// Store interface embed karne se baaki methods available rehte hain.
// Is test mein sirf Create call hona chahiye.
type createStoreStub struct {
	Store

	create func(
		context.Context,
		int64,
		int64,
		string,
		string,
		time.Time,
	) (Invitation, error)
}

func (s createStoreStub) Create(
	ctx context.Context,
	tenantID, actorID int64,
	email, tokenHash string,
	expires time.Time,
) (Invitation, error) {
	return s.create(
		ctx, tenantID, actorID, email, tokenHash, expires,
	)
}

func TestCreateInvitationStoresOnlyHash(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)

	var storedHash string
	called := false

	store := createStoreStub{
		create: func(
			ctx context.Context,
			tenantID, actorID int64,
			email, tokenHash string,
			expires time.Time,
		) (Invitation, error) {
			called = true

			if tenantID != 1 || actorID != 2 {
				t.Fatal("tenant or actor scope changed")
			}

			if email != "member@example.com" {
				t.Fatalf("email not normalized: %q", email)
			}

			if !expires.Equal(now.Add(24 * time.Hour)) {
				t.Fatal("invitation expiry must be 24 hours")
			}

			storedHash = tokenHash

			return Invitation{
				ID:        10,
				TenantID:  tenantID,
				Email:     email,
				ExpiresAt: expires,
			}, nil
		},
	}

	service := NewService(store)
	service.now = func() time.Time {
		return now
	}

	created, err := service.Create(
		context.Background(),
		1,
		2,
		" Member@Example.com ",
	)
	if err != nil {
		t.Fatal(err)
	}

	if !called {
		t.Fatal("repository Create was not called")
	}

	if len(created.Token) != 64 {
		t.Fatal("expected a 64-character invitation token")
	}

	if storedHash == created.Token {
		t.Fatal("raw invitation token must not be stored")
	}

	if storedHash != hashToken(created.Token) {
		t.Fatal("stored hash does not match returned token")
	}

	if created.ID != 10 {
		t.Fatal("repository result was not returned")
	}
}
