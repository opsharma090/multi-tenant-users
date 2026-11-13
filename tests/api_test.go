package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type userResponse struct {
	Data struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"data"`
}

func TestAPIIntegration(t *testing.T) {
	baseURL := strings.TrimRight(os.Getenv("TEST_API_URL"), "/")
	if baseURL == "" {
		t.Skip("set TEST_API_URL to run integration tests")
	}

	client := &http.Client{Timeout: 10 * time.Second}

	call := func(
		method, path, token string,
		body any,
		expected int,
		output any,
	) {
		t.Helper()

		var encoded []byte
		var err error

		if body != nil {
			encoded, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}

		request, err := http.NewRequest(
			method, baseURL+path, bytes.NewReader(encoded),
		)
		if err != nil {
			t.Fatal(err)
		}

		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}

		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()

		data, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}

		if response.StatusCode != expected {
			// Avoid printing token responses.
			t.Fatalf("%s %s: expected %d, got %d",
				method, path, expected, response.StatusCode)
		}

		if output != nil {
			if err := json.Unmarshal(data, output); err != nil {
				t.Fatalf("invalid JSON response: %v", err)
			}
		}
	}

	login := func(email, password, tenant string) tokenResponse {
		t.Helper()

		var result tokenResponse
		call("POST", "/auth/login", "", map[string]string{
			"email":       email,
			"password":    password,
			"tenant_code": tenant,
		}, 200, &result)

		if result.AccessToken == "" || result.RefreshToken == "" {
			t.Fatal("login returned empty tokens")
		}
		return result
	}

	alpha := login("admin@alpha.com", "Admin@123", "alpha")
	beta := login("admin@beta.com", "Admin@123", "beta")

	t.Run("missing token returns 401", func(t *testing.T) {
		call("GET", "/users", "", nil, 401, nil)
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		call("GET", "/users", "invalid-token", nil, 401, nil)
	})

	t.Run("invalid login returns 401", func(t *testing.T) {
		call("POST", "/auth/login", "", map[string]string{
			"email":       "admin@alpha.com",
			"password":    "wrong-password",
			"tenant_code": "alpha",
		}, 401, nil)
	})

	// Create a fresh user with no roles for deterministic RBAC checks.
	email := fmt.Sprintf("integration-%d@example.com", time.Now().UnixNano())

	var created userResponse
	call("POST", "/users", alpha.AccessToken, map[string]string{
		"name":     "Integration User",
		"email":    email,
		"password": "TestUser@123",
	}, 201, &created)

	if created.Data.ID <= 0 {
		t.Fatal("create returned invalid user ID")
	}

	path := fmt.Sprintf("/users/%d", created.Data.ID)
	deleted := false

	t.Cleanup(func() {
		if !deleted {
			call("DELETE", path, alpha.AccessToken, nil, 204, nil)
		}
	})

	unprivileged := login(email, "TestUser@123", "alpha")

	t.Run("missing permission returns 403", func(t *testing.T) {
		call("GET", "/permission-check",
			unprivileged.AccessToken, nil, 403, nil)
	})

	t.Run("own tenant can read user", func(t *testing.T) {
		call("GET", path, alpha.AccessToken, nil, 200, nil)
	})

	t.Run("other tenant cannot read update or delete user", func(t *testing.T) {
		call("GET", path, beta.AccessToken, nil, 404, nil)

		call("PATCH", path, beta.AccessToken,
			map[string]string{"name": "Forbidden Change"}, 404, nil)

		call("DELETE", path, beta.AccessToken, nil, 404, nil)
	})

	t.Run("request tenant override is ignored", func(t *testing.T) {
		call("GET", path+"?tenant_id=1",
			beta.AccessToken, nil, 404, nil)
	})

	t.Run("other tenant role cannot be assigned", func(t *testing.T) {
		var roles struct {
			Data []struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}

		call("GET", "/roles", beta.AccessToken, nil, 200, &roles)
		if len(roles.Data) == 0 {
			t.Fatal("Beta seed roles missing")
		}

		assignmentPath := fmt.Sprintf(
			"%s/roles/%d", path, roles.Data[0].ID,
		)

		call("PUT", assignmentPath,
			alpha.AccessToken, nil, 404, nil)
	})

	t.Run("refresh rotates and rejects reuse", func(t *testing.T) {
		var refreshed tokenResponse

		body := map[string]string{
			"refresh_token": unprivileged.RefreshToken,
		}

		call("POST", "/auth/refresh", "", body, 200, &refreshed)

		if refreshed.RefreshToken == "" ||
			refreshed.RefreshToken == unprivileged.RefreshToken {
			t.Fatal("refresh token was not rotated")
		}

		call("GET", "/me", refreshed.AccessToken, nil, 200, nil)
		call("POST", "/auth/refresh", "", body, 401, nil)

		unprivileged = refreshed
	})

	t.Run("own tenant can update user", func(t *testing.T) {
		var updated userResponse

		call("PATCH", path, alpha.AccessToken,
			map[string]string{"name": "Updated Integration User"},
			200, &updated)

		if updated.Data.Name != "Updated Integration User" {
			t.Fatal("updated name does not match")
		}
	})

	t.Run("permission cache follows grants and revocations", func(t *testing.T) {
		var role struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}

		call("POST", "/roles", alpha.AccessToken,
			map[string]string{
				"name": fmt.Sprintf("cache-test-%d", time.Now().UnixNano()),
			}, 201, &role)

		permissionPath := fmt.Sprintf(
			"/roles/%d/permissions", role.Data.ID,
		)

		call("PUT", permissionPath, alpha.AccessToken,
			map[string]any{
				"permissions": []string{"user:create"},
			}, 204, nil)

		// Populate the user's cache before assigning the new role.
		call("GET", "/permission-check",
			unprivileged.AccessToken, nil, 403, nil)

		assignmentPath := fmt.Sprintf(
			"%s/roles/%d", path, role.Data.ID,
		)

		call("PUT", assignmentPath, alpha.AccessToken, nil, 204, nil)

		// The existing token must see the grant immediately.
		call("GET", "/permission-check",
			unprivileged.AccessToken, nil, 200, nil)

		call("PUT", permissionPath, alpha.AccessToken,
			map[string]any{
				"permissions": []string{},
			}, 204, nil)

		// The existing token must also see the revocation.
		call("GET", "/permission-check",
			unprivileged.AccessToken, nil, 403, nil)
	})
	t.Run("existing user joins another tenant through invitation", func(t *testing.T) {
		var invitation struct {
			Data struct {
				Token    string `json:"invitation_token"`
				TenantID int64  `json:"tenant_id"`
			} `json:"data"`
		}

		// Fresh test user Alpha mein already exist karta hai.
		// Beta admin use Beta mein invite karega.
		call(
			"POST",
			"/invitations",
			beta.AccessToken,
			map[string]string{"email": email},
			201,
			&invitation,
		)

		if invitation.Data.Token == "" {
			t.Fatal("invitation token missing")
		}

		acceptBody := map[string]string{
			"invitation_token": invitation.Data.Token,
			"email":            email,
			"password":         "WrongPassword",
		}

		// Wrong password invitation consume nahi karna chahiye.
		call(
			"POST", "/invitations/accept",
			"", acceptBody, 401, nil,
		)

		acceptBody["password"] = "TestUser@123"

		var joined struct {
			Data struct {
				TenantID int64 `json:"tenant_id"`
				UserID   int64 `json:"user_id"`
			} `json:"data"`
		}

		call(
			"POST", "/invitations/accept",
			"", acceptBody, 200, &joined,
		)

		if joined.Data.UserID != created.Data.ID {
			t.Fatal("acceptance must reuse the existing global user")
		}

		if joined.Data.TenantID != invitation.Data.TenantID {
			t.Fatal("membership created in the wrong tenant")
		}

		// Accepted invitation cannot be reused.
		call(
			"POST", "/invitations/accept",
			"", acceptBody, 401, nil,
		)

		betaMember := login(email, "TestUser@123", "beta")

		call(
			"GET", "/me",
			betaMember.AccessToken, nil, 200, nil,
		)

		// Invitation grants membership, not roles.
		call(
			"GET", "/users",
			betaMember.AccessToken, nil, 403, nil,
		)

		// Alpha membership remains accessible.
		call(
			"GET", "/me",
			unprivileged.AccessToken, nil, 200, nil,
		)

		// Clean up only the newly added Beta membership.
		call(
			"DELETE", path,
			beta.AccessToken, nil, 204, nil,
		)

		call(
			"GET", "/me",
			betaMember.AccessToken, nil, 401, nil,
		)
	})
	t.Run("soft delete removes access", func(t *testing.T) {
		call("DELETE", path, alpha.AccessToken, nil, 204, nil)
		deleted = true

		call("GET", path, alpha.AccessToken, nil, 404, nil)
		call("GET", "/me", unprivileged.AccessToken, nil, 401, nil)

		call("POST", "/auth/refresh", "", map[string]string{
			"refresh_token": unprivileged.RefreshToken,
		}, 401, nil)
	})
}
