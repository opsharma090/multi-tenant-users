package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

var ErrCredentials = errors.New("invalid credentials or tenant")

type Claims struct {
	TenantID int64  `json:"tenant_id"`
	Type     string `json:"type"`
	jwt.RegisteredClaims
}

type Tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type Service struct {
	repo   *Repository
	secret []byte
}

func NewService(repo *Repository, secret string) *Service {
	return &Service{
		repo:   repo,
		secret: []byte(secret),
	}
}

func (s *Service) Login(
	ctx context.Context,
	email, password, tenantCode string,
) (Tokens, error) {
	identity, err := s.repo.FindIdentity(ctx, email, tenantCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return Tokens{}, ErrCredentials
	}
	if err != nil {
		return Tokens{}, err
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(identity.PasswordHash),
		[]byte(password),
	); err != nil {
		return Tokens{}, ErrCredentials
	}

	now := time.Now()

	claims := Claims{
		TenantID: identity.TenantID,
		Type:     "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "multi-tenant-users",
			Subject:   strconv.FormatInt(identity.UserID, 10),
			Audience:  jwt.ClaimStrings{"multi-tenant-api"},
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	accessToken, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	).SignedString(s.secret)
	if err != nil {
		return Tokens{}, err
	}

	refreshBytes := make([]byte, 32)
	if _, err := rand.Read(refreshBytes); err != nil {
		return Tokens{}, err
	}

	refreshToken := hex.EncodeToString(refreshBytes)
	refreshHash := sha256.Sum256([]byte(refreshToken))

	tx, err := s.repo.db.Begin(ctx)
	if err != nil {
		return Tokens{}, err
	}
	defer tx.Rollback(ctx)

	// Recheck membership and tenant status inside the transaction.
	var userID int64
	err = tx.QueryRow(ctx, `
		SELECT tu.user_id
		FROM tenant_users tu
		JOIN tenants t ON t.id = tu.tenant_id
		WHERE tu.tenant_id = $1
		  AND tu.user_id = $2
		  AND tu.deleted_at IS NULL
		  AND t.status = 'active'
		FOR SHARE OF tu, t
	`, identity.TenantID, identity.UserID).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Tokens{}, ErrCredentials
	}
	if err != nil {
		return Tokens{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO refresh_tokens
			(tenant_id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`,
		identity.TenantID,
		identity.UserID,
		hex.EncodeToString(refreshHash[:]),
		now.Add(7*24*time.Hour),
	)
	if err != nil {
		return Tokens{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs
			(tenant_id, actor_id, action, target_type, target_id)
		VALUES ($1, $2, 'auth:login', 'user', $2)
	`, identity.TenantID, identity.UserID)
	if err != nil {
		return Tokens{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Tokens{}, err
	}

	return Tokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    900,
	}, nil
}
