package user

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"strconv"
	"strings"

	"multi-tenant-users/internal/middleware"
	"multi-tenant-users/internal/platform"
)

func decodeBody(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()

	if err := d.Decode(target); err != nil {
		platform.Error(w, 400, "invalid_request", "Invalid JSON body")
		return false
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		platform.Error(w, 400, "invalid_request", "Expected one JSON object")
		return false
	}
	return true
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		platform.Error(w, 404, "not_found", "User not found")
	case errors.Is(err, ErrConflict):
		platform.Error(w, 409, "conflict", "Email already registered")
	case errors.Is(err, ErrSelfDelete):
		platform.Error(w, 409, "conflict", "Cannot delete your own membership")
	default:
		platform.Error(w, 500, "internal_error", "Unable to save user changes")
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	identity, ok := middleware.CurrentIdentity(r.Context())
	if !ok {
		platform.Error(w, 401, "unauthorized", "Authentication required")
		return
	}

	var input struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeBody(w, r, &input) {
		return
	}

	input.Name = strings.TrimSpace(input.Name)
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	address, err := mail.ParseAddress(input.Email)

	if input.Name == "" || len(input.Name) > 150 ||
		err != nil || address.Address != input.Email ||
		len(input.Email) > 255 ||
		len(input.Password) < 8 || len(input.Password) > 72 {
		platform.Error(w, 400, "validation_error",
			"Valid name, email and password of 8–72 bytes required")
		return
	}

	u, err := h.service.Create(
		r.Context(), identity.TenantID, identity.UserID,
		input.Name, input.Email, input.Password,
	)
	if err != nil {
		writeError(w, err)
		return
	}

	w.Header().Set("Location", "/users/"+strconv.FormatInt(u.ID, 10))
	platform.JSON(w, 201, map[string]any{"data": u})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	identity, ok := middleware.CurrentIdentity(r.Context())
	if !ok {
		platform.Error(w, 401, "unauthorized", "Authentication required")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		platform.Error(w, 400, "validation_error", "Invalid user ID")
		return
	}

	var input struct {
		Name string `json:"name"`
	}
	if !decodeBody(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(input.Name) > 150 {
		platform.Error(w, 400, "validation_error", "Valid name required")
		return
	}

	u, err := h.service.Update(
		r.Context(), identity.TenantID, identity.UserID, id, input.Name,
	)
	if err != nil {
		writeError(w, err)
		return
	}

	platform.JSON(w, 200, map[string]any{"data": u})
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	identity, ok := middleware.CurrentIdentity(r.Context())
	if !ok {
		platform.Error(w, 401, "unauthorized", "Authentication required")
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		platform.Error(w, 400, "validation_error", "Invalid user ID")
		return
	}

	if err := h.service.Delete(
		r.Context(), identity.TenantID, identity.UserID, id,
	); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
