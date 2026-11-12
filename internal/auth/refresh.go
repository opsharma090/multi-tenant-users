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
)

var ErrRefreshToken = errors.New("invalid or expired refresh token")

func (s *Service) Refresh(
	ctx context.Context,
	refreshToken string,
) (Tokens, error) {
	hash := sha256.Sum256([]byte(refreshToken))
	tokenHash := hex.EncodeToString(hash[:])

	tx, err := s.repo.db.Begin(ctx)
	if err != nil {
		return Tokens{}, err
	}
	defer tx.Rollback(ctx)

	var tokenID, tenantID, userID int64

	// Lock the token so concurrent requests cannot reuse it.
	err = tx.QueryRow(ctx, `
		SELECT rt.id, rt.tenant_id, rt.user_id
		FROM refresh_tokens rt
		WHERE rt.token_hash = $1
		  AND rt.revoked_at IS NULL
		  AND rt.expires_at > NOW()
		FOR UPDATE OF rt
	`, tokenHash).Scan(&tokenID, &tenantID, &userID)

	if errors.Is(err, pgx.ErrNoRows) {
		return Tokens{}, ErrRefreshToken
	}
	if err != nil {
		return Tokens{}, err
	}

	// Tenant and membership must still be active.
	var memberID int64
	err = tx.QueryRow(ctx, `
		SELECT tu.user_id
		FROM tenant_users tu
		JOIN tenants t ON t.id = tu.tenant_id
		WHERE tu.tenant_id = $1
		  AND tu.user_id = $2
		  AND tu.deleted_at IS NULL
		  AND t.status = 'active'
		FOR SHARE OF tu, t
	`, tenantID, userID).Scan(&memberID)

	if errors.Is(err, pgx.ErrNoRows) {
		return Tokens{}, ErrRefreshToken
	}
	if err != nil {
		return Tokens{}, err
	}

	now := time.Now()

	claims := Claims{
		TenantID: tenantID,
		Type:     "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "multi-tenant-users",
			Subject:   strconv.FormatInt(userID, 10),
			Audience:  jwt.ClaimStrings{"multi-tenant-api"},
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	accessToken, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256, claims,
	).SignedString(s.secret)
	if err != nil {
		return Tokens{}, err
	}

	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return Tokens{}, err
	}

	newRefreshToken := hex.EncodeToString(randomBytes)
	newHash := sha256.Sum256([]byte(newRefreshToken))

	_, err = tx.Exec(ctx, `
		UPDATE refresh_tokens
		SET revoked_at = NOW()
		WHERE id = $1
	`, tokenID)
	if err != nil {
		return Tokens{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO refresh_tokens
			(tenant_id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, tenantID, userID, hex.EncodeToString(newHash[:]),
		now.Add(7*24*time.Hour))
	if err != nil {
		return Tokens{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_logs
			(tenant_id, actor_id, action, target_type, target_id)
		VALUES ($1, $2, 'auth:refresh', 'user', $2)
	`, tenantID, userID)
	if err != nil {
		return Tokens{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Tokens{}, err
	}

	return Tokens{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    900,
	}, nil
}
