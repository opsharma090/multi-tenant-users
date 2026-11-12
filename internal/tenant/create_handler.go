package tenant

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"multi-tenant-users/internal/middleware"
	"multi-tenant-users/internal/platform"
)

var ErrCreateValidation = errors.New(
	"name required; code must be 2–50 lowercase letters, digits or hyphens and start with a letter or digit",
)

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	identity, ok := middleware.CurrentIdentity(r.Context())
	if !ok {
		platform.Error(w, 401, "unauthorized", "Authentication required")
		return
	}

	var input struct {
		Name string `json:"name"`
		Code string `json:"code"`
	}

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
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

	result, err := h.service.Create(
		r.Context(), identity.TenantID, identity.UserID,
		input.Name, input.Code,
	)

	switch {
	case errors.Is(err, ErrCreateValidation):
		platform.Error(w, 400, "validation_error", err.Error())
		return
	case errors.Is(err, ErrCodeConflict):
		platform.Error(w, 409, "conflict", "Organization code already exists")
		return
	case err != nil:
		platform.Error(w, 500, "internal_error", "Unable to create organization")
		return
	}

	platform.JSON(w, 201, map[string]any{"data": result})
}
