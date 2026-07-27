package handler

import (
	"net/http"

	"gopricloud/gopricloud/internal/delivery/http/httpx"
)

type TestHandler struct{}

func NewTestHandler() *TestHandler {
	return &TestHandler{}
}

// Test handles GET /test. It sits behind the auth middleware and simply
// confirms the caller holds a valid access token by returning an empty
// object.
func (h *TestHandler) Test(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, struct{}{})
}
