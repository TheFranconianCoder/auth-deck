package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/TheFranconianCoder/auth-deck/internal/core/entities"
	"github.com/TheFranconianCoder/auth-deck/internal/types"
)

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

func writeToken(w http.ResponseWriter, token *entities.Token) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	fmt.Fprintf(w, `{"access_token":"%s","token_type":"%s","expires_in":%d}`,
		token.AccessToken, token.TokenType, token.ExpiresIn())
}
