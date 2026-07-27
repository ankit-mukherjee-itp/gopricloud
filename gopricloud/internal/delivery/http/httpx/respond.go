// Package httpx holds small JSON response helpers shared by handlers and
// middleware, kept separate to avoid a net/http <-> http package name clash.
package httpx

import (
	"encoding/json"
	"net/http"

	"gopricloud/gopricloud/internal/delivery/http/dto"
)

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, dto.ErrorResponse{Error: message})
}
