package invitation

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrValidation    = errors.New("invalid invitation input")
	ErrInvalid       = errors.New("invalid, expired or consumed invitation")
	ErrCredentials   = errors.New("invalid credentials")
	ErrAlreadyMember = errors.New("user is already an active tenant member")
	ErrForbidden     = errors.New("invitation permission is unavailable")
)

type Invitation struct {
	ID         int64      `json:"id"`
	TenantID   int64      `json:"tenant_id"`
	Email      string     `json:"email"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"-"`
	RevokedAt  *time.Time `json:"-"`
}

type Created struct {
	Invitation
	Token string `json:"invitation_token"`
}

type Account struct {
	ID           int64
	PasswordHash string
}

type Membership struct {
	TenantID int64 `json:"tenant_id"`
	UserID   int64 `json:"user_id"`
}

// Service ka database contract.
// Repository next step mein is interface ko implement karegi.
type Store interface {
	Create(
		context.Context,
		int64,
		int64,
		string,
		string,
		time.Time,
	) (Invitation, error)

	Account(context.Context, string) (Account, error)

	Accept(
		context.Context,
		string,
		int64,
		string,
		time.Time,
	) (Membership, error)
}

type Service struct {
	store  Store
	now    func() time.Time
	random func([]byte) (int, error)
}

func NewService(store Store) *Service {
	return &Service{
		store:  store,
		now:    time.Now,
		random: rand.Read,
	}
}

func normalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))

	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 255 {
		return "", ErrValidation
	}

	return email, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func validateInvitation(
	inv Invitation,
	email string,
	now time.Time,
) error {
	if inv.Email != email ||
		inv.AcceptedAt != nil ||
		inv.RevokedAt != nil ||
		!now.Before(inv.ExpiresAt) {
		return ErrInvalid
	}

	return nil
}

func (s *Service) Create(
	ctx context.Context,
	tenantID, actorID int64,
	email string,
) (Created, error) {
	email, err := normalizeEmail(email)
	if err != nil || tenantID <= 0 || actorID <= 0 {
		return Created{}, ErrValidation
	}

	randomBytes := make([]byte, 32)

	n, err := s.random(randomBytes)
	if err != nil {
		return Created{}, err
	}
	if n != len(randomBytes) {
		return Created{}, errors.New("incomplete invitation entropy")
	}

	token := hex.EncodeToString(randomBytes)

	inv, err := s.store.Create(
		ctx,
		tenantID,
		actorID,
		email,
		hashToken(token),
		s.now().Add(24*time.Hour),
	)
	if err != nil {
		return Created{}, err
	}

	return Created{
		Invitation: inv,
		Token:      token,
	}, nil
}

func (s *Service) Accept(
	ctx context.Context,
	token, email, password string,
) (Membership, error) {
	email, err := normalizeEmail(email)
	if err != nil || len(password) == 0 || len(password) > 72 {
		return Membership{}, ErrValidation
	}

	decoded, err := hex.DecodeString(token)
	if err != nil || len(decoded) != 32 || len(token) != 64 {
		return Membership{}, ErrValidation
	}

	account, err := s.store.Account(ctx, email)
	if err != nil {
		return Membership{}, err
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(account.PasswordHash),
		[]byte(password),
	)
	if err != nil {
		return Membership{}, ErrCredentials
	}

	return s.store.Accept(
		ctx,
		hashToken(token),
		account.ID,
		email,
		s.now(),
	)
}
