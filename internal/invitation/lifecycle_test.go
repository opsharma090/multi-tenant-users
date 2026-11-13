package invitation

import (
	"errors"
	"testing"
	"time"
)

func TestInvitationLifecycle(t *testing.T) {
	now := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name  string
		inv   Invitation
		valid bool
	}{
		{
			name: "pending invitation",
			inv: Invitation{
				Email:     "member@example.com",
				ExpiresAt: future,
			},
			valid: true,
		},
		{
			name: "wrong recipient",
			inv: Invitation{
				Email:     "other@example.com",
				ExpiresAt: future,
			},
		},
		{
			name: "expired invitation",
			inv: Invitation{
				Email:     "member@example.com",
				ExpiresAt: past,
			},
		},
		{
			name: "exact expiry boundary",
			inv: Invitation{
				Email:     "member@example.com",
				ExpiresAt: now,
			},
		},
		{
			name: "already accepted",
			inv: Invitation{
				Email:      "member@example.com",
				ExpiresAt:  future,
				AcceptedAt: &past,
			},
		},
		{
			name: "revoked invitation",
			inv: Invitation{
				Email:     "member@example.com",
				ExpiresAt: future,
				RevokedAt: &past,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateInvitation(
				tc.inv,
				"member@example.com",
				now,
			)

			if tc.valid {
				if err != nil {
					t.Fatalf("expected valid invitation, got %v", err)
				}
				return
			}

			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("expected ErrInvalid, got %v", err)
			}
		})
	}
}
