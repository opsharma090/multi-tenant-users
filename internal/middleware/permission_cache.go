package middleware

import (
	"context"
	"sync"
	"time"
)

type permissionKey struct {
	TenantID int64
	UserID   int64
	Version  int64
}

type permissionEntry struct {
	Codes     map[string]bool
	ExpiresAt time.Time
}

type PermissionCache struct {
	mu      sync.Mutex
	entries map[permissionKey]permissionEntry
}

func NewPermissionCache() *PermissionCache {
	return &PermissionCache{
		entries: make(map[permissionKey]permissionEntry),
	}
}

func (c *PermissionCache) get(
	key permissionKey,
) (map[string]bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	if !time.Now().Before(entry.ExpiresAt) {
		delete(c.entries, key)
		return nil, false
	}

	return entry.Codes, true
}

func (c *PermissionCache) put(
	key permissionKey,
	codes map[string]bool,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Bound memory usage for this assignment.
	if len(c.entries) >= 1000 {
		clear(c.entries)
	}

	c.entries[key] = permissionEntry{
		Codes:     codes,
		ExpiresAt: time.Now().Add(time.Minute),
	}
}

func (a *Auth) permissions(
	ctx context.Context,
	identity Identity,
) (map[string]bool, error) {
	var version int64

	err := a.db.QueryRow(ctx, `
		SELECT permission_version
		FROM tenants WHERE id = $1
	`, identity.TenantID).Scan(&version)
	if err != nil {
		return nil, err
	}

	key := permissionKey{
		TenantID: identity.TenantID,
		UserID:   identity.UserID,
		Version:  version,
	}

	if codes, ok := a.cache.get(key); ok {
		return codes, nil
	}

	rows, err := a.db.Query(ctx, `
		SELECT DISTINCT p.code
		FROM user_roles ur
		JOIN role_permissions rp
		  ON rp.tenant_id = ur.tenant_id
		 AND rp.role_id = ur.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE ur.tenant_id = $1 AND ur.user_id = $2
	`, identity.TenantID, identity.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	codes := make(map[string]bool)

	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		codes[code] = true
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Maps are read-only after insertion into the cache.
	a.cache.put(key, codes)
	return codes, nil
}
