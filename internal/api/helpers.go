package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/TheFranconianCoder/auth-deck/internal/core/entities"
	"github.com/TheFranconianCoder/auth-deck/internal/types"
)

// refreshTokenFor returns AuthDeck's stable, opaque refresh token for a
// provider. It is not an upstream secret: the provider is already addressed by
// the request path, so the value only signals that a refresh is available.
func refreshTokenFor(provider string) string {
	return "authdeck:" + provider
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, err error) {
	var appErr *types.AppError
	if errors.As(err, &appErr) {
		respondJSON(w, appErr.Code, appErr)
		return
	}
	respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

func writeToken(w http.ResponseWriter, token *entities.Token, provider string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	payload := map[string]any{
		"access_token":  token.AccessToken,
		"token_type":    token.TokenType,
		"expires_in":    token.ExpiresIn(),
		"refresh_token": refreshTokenFor(provider),
	}
	if token.Scope != "" {
		payload["scope"] = token.Scope
	}
	json.NewEncoder(w).Encode(payload)
}
