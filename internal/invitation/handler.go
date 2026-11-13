package invitation

import (
	"encoding/json"
	"errors"
	"io"
	"log"
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

func decode(
	w http.ResponseWriter,
	r *http.Request,
	input any,
) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(input); err != nil {
		platform.Error(w, 400, "invalid_request", "Invalid JSON body")
		return false
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		platform.Error(w, 400, "invalid_request", "Expected one JSON object")
		return false
	}

	return true
}

func respondError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrValidation):
		platform.Error(
			w, 400, "validation_error",
			"Valid email and invitation input required",
		)

	case errors.Is(err, ErrCredentials):
		platform.Error(
			w, 401, "unauthorized",
			"Invalid credentials",
		)

	case errors.Is(err, ErrInvalid):
		platform.Error(
			w, 401, "invalid_invitation",
			"Invalid or unavailable invitation",
		)

	case errors.Is(err, ErrForbidden):
		platform.Error(
			w, 403, "forbidden",
			"Permission denied",
		)

	case errors.Is(err, ErrAlreadyMember):
		platform.Error(
			w, 409, "conflict",
			"User is already an active member",
		)

	default:
		log.Printf("invitation operation failed: %v", err)

		platform.Error(
			w, 500, "internal_error",
			"Unable to process invitation",
		)
	}
}

func (h *Handler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	identity, ok := middleware.CurrentIdentity(r.Context())
	if !ok {
		platform.Error(
			w, 401, "unauthorized",
			"Authentication required",
		)
		return
	}

	var input struct {
		Email string `json:"email"`
	}

	if !decode(w, r, &input) {
		return
	}

	result, err := h.service.Create(
		r.Context(),
		identity.TenantID,
		identity.UserID,
		input.Email,
	)
	if err != nil {
		respondError(w, err)
		return
	}

	w.Header().Set("Cache-Control", "no-store")

	platform.JSON(w, http.StatusCreated, map[string]any{
		"data": result,
	})
}

func (h *Handler) Accept(
	w http.ResponseWriter,
	r *http.Request,
) {
	var input struct {
		Token    string `json:"invitation_token"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if !decode(w, r, &input) {
		return
	}

	result, err := h.service.Accept(
		r.Context(),
		input.Token,
		input.Email,
		input.Password,
	)
	if err != nil {
		respondError(w, err)
		return
	}

	w.Header().Set("Cache-Control", "no-store")

	platform.JSON(w, http.StatusOK, map[string]any{
		"data": result,
	})
}
