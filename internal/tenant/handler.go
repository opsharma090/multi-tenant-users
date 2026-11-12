package tenant

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"multi-tenant-users/internal/middleware"
	"multi-tenant-users/internal/platform"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func tenantError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		platform.Error(w, 404, "not_found", "Tenant not found")
	case errors.Is(err, ErrValidation):
		platform.Error(w, 400, "validation_error", err.Error())
	default:
		platform.Error(w, 500, "internal_error", "Unable to process tenant")
	}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	identity, ok := middleware.CurrentIdentity(r.Context())
	if !ok {
		platform.Error(w, 401, "unauthorized", "Authentication required")
		return
	}

	t, err := h.service.Get(r.Context(), identity.TenantID)
	if err != nil {
		tenantError(w, err)
		return
	}
	platform.JSON(w, 200, map[string]any{"data": t})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	identity, ok := middleware.CurrentIdentity(r.Context())
	if !ok {
		platform.Error(w, 401, "unauthorized", "Authentication required")
		return
	}

	var input struct {
		Name   string `json:"name"`
		Status string `json:"status"`
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

	t, err := h.service.Update(
		r.Context(), identity.TenantID, identity.UserID,
		input.Name, input.Status,
	)
	if err != nil {
		tenantError(w, err)
		return
	}
	platform.JSON(w, 200, map[string]any{"data": t})
}
