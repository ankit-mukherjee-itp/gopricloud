package rest

import (
	"net/http"

	"backend/internal/adapters/api"
)

type TestHandler struct{}

func NewTestHandler() *TestHandler {
	return &TestHandler{}
}

// Test handles GET /test. It sits behind the auth middleware and simply
// confirms the caller holds a valid access token by returning an empty
// object.
func (h *TestHandler) Test(w http.ResponseWriter, r *http.Request) {
	api.WriteJSON(w, http.StatusOK, struct{}{})
}
