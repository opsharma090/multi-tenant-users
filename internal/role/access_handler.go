package role

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"multi-tenant-users/internal/middleware"
	"multi-tenant-users/internal/platform"
)

func accessError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		platform.Error(w, 404, "not_found", "Resource not found")
	case errors.Is(err, ErrPermissions):
		platform.Error(w, 400, "validation_error",
			"Provide an array of valid permission codes")
	default:
		platform.Error(w, 500, "internal_error", "Unable to save changes")
	}
}

func (h *Handler) Permissions(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Permissions(r.Context())
	if err != nil {
		platform.Error(w, 500, "internal_error", "Unable to list permissions")
		return
	}
	platform.JSON(w, 200, map[string]any{"data": result})
}

func (h *Handler) SetPermissions(w http.ResponseWriter, r *http.Request) {
	identity, ok := middleware.CurrentIdentity(r.Context())
	if !ok {
		platform.Error(w, 401, "unauthorized", "Authentication required")
		return
	}

	roleID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || roleID <= 0 {
		platform.Error(w, 400, "validation_error", "Invalid role ID")
		return
	}

	var input struct {
		Permissions []string `json:"permissions"`
	}

	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&input); err != nil {
		platform.Error(w, 400, "invalid_request", "Invalid JSON body")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		platform.Error(w, 400, "invalid_request", "Expected one JSON object")
		return
	}

	err = h.service.SetPermissions(
		r.Context(), identity.TenantID, identity.UserID,
		roleID, input.Permissions,
	)
	if err != nil {
		accessError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Assign(w http.ResponseWriter, r *http.Request) {
	identity, ok := middleware.CurrentIdentity(r.Context())
	if !ok {
		platform.Error(w, 401, "unauthorized", "Authentication required")
		return
	}

	userID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || userID <= 0 {
		platform.Error(w, 400, "validation_error", "Invalid user ID")
		return
	}

	roleID, err := strconv.ParseInt(r.PathValue("roleId"), 10, 64)
	if err != nil || roleID <= 0 {
		platform.Error(w, 400, "validation_error", "Invalid role ID")
		return
	}

	err = h.service.Assign(
		r.Context(), identity.TenantID, identity.UserID, userID, roleID,
	)
	if err != nil {
		accessError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
