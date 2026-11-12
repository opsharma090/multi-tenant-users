package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"multi-tenant-users/internal/auth"
	"multi-tenant-users/internal/platform"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type contextKey struct{}

type Identity struct {
	UserID   int64 `json:"user_id"`
	TenantID int64 `json:"tenant_id"`
}

func CurrentIdentity(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(contextKey{}).(Identity)
	return identity, ok
}

type Auth struct {
	db     *pgxpool.Pool
	secret []byte
	cache  *PermissionCache
}

func NewAuth(db *pgxpool.Pool, secret string) *Auth {
	return &Auth{
		db:     db,
		secret: []byte(secret),
		cache:  NewPermissionCache(),
	}
}

func (a *Auth) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))

		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			platform.Error(w, 401, "unauthorized", "Bearer token required")
			return
		}

		claims := &auth.Claims{}

		token, err := jwt.ParseWithClaims(
			parts[1],
			claims,
			func(token *jwt.Token) (any, error) {
				return a.secret, nil
			},
			jwt.WithValidMethods([]string{"HS256"}),
			jwt.WithIssuer("multi-tenant-users"),
			jwt.WithAudience("multi-tenant-api"),
			jwt.WithExpirationRequired(),
			jwt.WithIssuedAt(),
		)

		if err != nil || token == nil || !token.Valid ||
			claims.Type != "access" || claims.TenantID <= 0 {
			platform.Error(w, 401, "unauthorized", "Invalid or expired token")
			return
		}

		userID, err := strconv.ParseInt(claims.Subject, 10, 64)
		if err != nil || userID <= 0 {
			platform.Error(w, 401, "unauthorized", "Invalid token subject")
			return
		}

		// Deleted membership or suspended tenant loses access immediately.
		var active bool
		err = a.db.QueryRow(r.Context(), `
			SELECT EXISTS (
				SELECT 1
				FROM tenant_users tu
				JOIN tenants t ON t.id = tu.tenant_id
				WHERE tu.tenant_id = $1
				  AND tu.user_id = $2
				  AND tu.deleted_at IS NULL
				  AND t.status = 'active'
			)
		`, claims.TenantID, userID).Scan(&active)

		if err != nil {
			platform.Error(w, 500, "internal_error",
				"Unable to validate membership")
			return
		}

		if !active {
			platform.Error(w, 401, "unauthorized",
				"Tenant membership is unavailable")
			return
		}

		identity := Identity{
			UserID:   userID,
			TenantID: claims.TenantID,
		}

		ctx := context.WithValue(r.Context(), contextKey{}, identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (a *Auth) Require(permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := CurrentIdentity(r.Context())
		if !ok {
			platform.Error(w, 401, "unauthorized", "Authentication required")
			return
		}

		codes, err := a.permissions(r.Context(), identity)
		if err != nil {
			platform.Error(w, 500, "internal_error",
				"Unable to check permissions")
			return
		}

		if !codes[permission] {
			platform.Error(w, 403, "forbidden", "Permission denied")
			return
		}

		next.ServeHTTP(w, r)
	})
}
