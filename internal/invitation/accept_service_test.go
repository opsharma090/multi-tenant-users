package invitation

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type acceptStoreStub struct {
	Store

	account   Account
	called    bool
	tokenHash string
	email     string
}

func (s *acceptStoreStub) Account(
	ctx context.Context,
	email string,
) (Account, error) {
	return s.account, nil
}

func (s *acceptStoreStub) Accept(
	ctx context.Context,
	tokenHash string,
	userID int64,
	email string,
	now time.Time,
) (Membership, error) {
	s.called = true
	s.tokenHash = tokenHash
	s.email = email

	return Membership{
		TenantID: 1,
		UserID:   userID,
	}, nil
}

func TestAcceptRejectsWrongPasswordBeforeMembershipChange(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte("Correct@123"),
		bcrypt.MinCost,
	)
	if err != nil {
		t.Fatal(err)
	}

	store := &acceptStoreStub{
		account: Account{
			ID:           2,
			PasswordHash: string(passwordHash),
		},
	}

	service := NewService(store)

	// Valid format: 64 hexadecimal characters.
	token := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	_, err = service.Accept(
		context.Background(),
		token,
		"member@example.com",
		"Wrong@123",
	)

	if !errors.Is(err, ErrCredentials) {
		t.Fatalf("expected ErrCredentials, got %v", err)
	}

	if store.called {
		t.Fatal("wrong password reached membership mutation")
	}
}
func TestAcceptWithCorrectPasswordUsesVerifiedAccount(t *testing.T) {
	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte("Correct@123"),
		bcrypt.MinCost,
	)
	if err != nil {
		t.Fatal(err)
	}

	store := &acceptStoreStub{
		account: Account{
			ID:           2,
			PasswordHash: string(passwordHash),
		},
	}

	service := NewService(store)

	token := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	membership, err := service.Accept(
		context.Background(),
		token,
		" Member@Example.com ",
		"Correct@123",
	)
	if err != nil {
		t.Fatal(err)
	}

	if !store.called {
		t.Fatal("correct credentials did not reach acceptance")
	}

	if membership.UserID != 2 || membership.TenantID != 1 {
		t.Fatalf("unexpected membership: %+v", membership)
	}

	if store.email != "member@example.com" {
		t.Fatal("recipient email was not normalized")
	}

	if store.tokenHash == token ||
		store.tokenHash != hashToken(token) {
		t.Fatal("repository must receive only the token hash")
	}
}
