package role

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"multi-tenant-users/internal/middleware"
	"multi-tenant-users/internal/platform"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	identity, ok := middleware.CurrentIdentity(r.Context())
	if !ok {
		platform.Error(w, 401, "unauthorized", "Authentication required")
		return
	}

	var input struct {
		Name string `json:"name"`
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
		r.Context(), identity.TenantID, identity.UserID, input.Name,
	)

	switch {
	case errors.Is(err, ErrValidation):
		platform.Error(w, 400, "validation_error", err.Error())
		return
	case errors.Is(err, ErrConflict):
		platform.Error(w, 409, "conflict", "Role name already exists")
		return
	case err != nil:
		platform.Error(w, 500, "internal_error", "Unable to create role")
		return
	}

	platform.JSON(w, 201, map[string]any{"data": result})
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
		if err != nil || n < 1 || n > 1000000 {
			platform.Error(w, 400, "validation_error", "Invalid page")
			return
		}
		page = n
	}

	if value := r.URL.Query().Get("limit"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 100 {
			platform.Error(w, 400, "validation_error",
				"limit must be between 1 and 100")
			return
		}
		limit = n
	}

	search := strings.TrimSpace(r.URL.Query().Get("search"))
	if len(search) > 100 {
		platform.Error(w, 400, "validation_error", "Search is too long")
		return
	}

	result, err := h.service.List(
		r.Context(), identity.TenantID,
		search, limit, (page-1)*limit,
	)
	if err != nil {
		platform.Error(w, 500, "internal_error", "Unable to list roles")
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
