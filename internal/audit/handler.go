package audit

import (
	"errors"
	"net/http"
	"strconv"

	"multi-tenant-users/internal/middleware"
	"multi-tenant-users/internal/platform"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	identity, ok := middleware.CurrentIdentity(r.Context())
	if !ok {
		platform.Error(w, 401, "unauthorized", "Authentication required")
		return
	}

	page, limit := 1, 20

	if value := r.URL.Query().Get("page"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil {
			platform.Error(w, 400, "validation_error", "Invalid page")
			return
		}
		page = n
	}

	if value := r.URL.Query().Get("limit"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil {
			platform.Error(w, 400, "validation_error", "Invalid limit")
			return
		}
		limit = n
	}

	result, err := h.service.List(
		r.Context(),
		identity.TenantID,
		r.URL.Query().Get("action"),
		page,
		limit,
	)

	if errors.Is(err, ErrValidation) {
		platform.Error(w, 400, "validation_error", err.Error())
		return
	}
	if err != nil {
		platform.Error(w, 500, "internal_error", "Unable to list audit logs")
		return
	}

	platform.JSON(w, 200, map[string]any{
		"data": result,
		"pagination": map[string]int{
			"page":  page,
			"limit": limit,
		},
	})
}
