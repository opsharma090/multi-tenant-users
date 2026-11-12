package auth

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/mail"
	"strings"

	"multi-tenant-users/internal/platform"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email      string `json:"email"`
		Password   string `json:"password"`
		TenantCode string `json:"tenant_code"`
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

	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.TenantCode = strings.TrimSpace(input.TenantCode)

	address, err := mail.ParseAddress(input.Email)
	if err != nil || address.Address != input.Email ||
		len(input.Email) > 255 ||
		input.Password == "" || len(input.Password) > 72 ||
		input.TenantCode == "" || len(input.TenantCode) > 50 {
		platform.Error(w, 400, "validation_error",
			"Valid email, password and tenant_code are required")
		return
	}

	tokens, err := h.service.Login(
		r.Context(),
		input.Email,
		input.Password,
		input.TenantCode,
	)

	if errors.Is(err, ErrCredentials) {
		platform.Error(w, 401, "invalid_credentials",
			"Invalid credentials or tenant")
		return
	}

	if err != nil {
		log.Printf("login failed: %v", err)
		platform.Error(w, 500, "internal_error", "Unable to login")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	platform.JSON(w, http.StatusOK, tokens)
}
