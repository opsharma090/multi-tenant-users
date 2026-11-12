package user

import (
	"errors"
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
			platform.Error(w, 400, "validation_error",
				"page must be between 1 and 1000000")
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
	if len(search) > 255 {
		platform.Error(w, 400, "validation_error", "Search is too long")
		return
	}

	users, err := h.service.List(
		r.Context(),
		identity.TenantID,
		search,
		limit,
		(page-1)*limit,
	)
	if err != nil {
		platform.Error(w, 500, "internal_error", "Unable to list users")
		return
	}

	platform.JSON(w, 200, map[string]any{
		"data": users,
		"pagination": map[string]int{
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
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

	u, err := h.service.Get(r.Context(), identity.TenantID, id)
	if errors.Is(err, ErrNotFound) {
		platform.Error(w, 404, "not_found", "User not found")
		return
	}
	if err != nil {
		platform.Error(w, 500, "internal_error", "Unable to fetch user")
		return
	}

	platform.JSON(w, 200, map[string]any{"data": u})
}
