package auth

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"multi-tenant-users/internal/platform"
)

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RefreshToken string `json:"refresh_token"`
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

	decoded, err := hex.DecodeString(input.RefreshToken)
	if err != nil || len(decoded) != 32 || len(input.RefreshToken) != 64 {
		platform.Error(w, 400, "validation_error", "Valid refresh_token required")
		return
	}

	tokens, err := h.service.Refresh(r.Context(), input.RefreshToken)

	if errors.Is(err, ErrRefreshToken) {
		platform.Error(w, 401, "unauthorized", "Invalid or expired refresh token")
		return
	}
	if err != nil {
		log.Printf("refresh failed: %v", err)
		platform.Error(w, 500, "internal_error", "Unable to refresh token")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	platform.JSON(w, 200, tokens)
}
