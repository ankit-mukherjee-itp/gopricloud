package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"gopricloud/gopricloud/internal/delivery/http/dto"
	"gopricloud/gopricloud/internal/delivery/http/httpx"
	"gopricloud/gopricloud/internal/delivery/http/middleware"
	"gopricloud/gopricloud/internal/domain"
	"gopricloud/gopricloud/internal/usecase"
)

type ComputeHandler struct {
	compute *usecase.ComputeUsecase
}

func NewComputeHandler(compute *usecase.ComputeUsecase) *ComputeHandler {
	return &ComputeHandler{compute: compute}
}

// Create handles POST /instances: boots a new server via Nova.
func (h *ComputeHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r)
	if !ok {
		return
	}

	var req dto.CreateComputeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.ImageRef == "" || req.FlavorRef == "" {
		httpx.WriteError(w, http.StatusBadRequest, "name, image_ref and flavor_ref are required")
		return
	}

	record, err := h.compute.Create(r.Context(), userID, domain.ComputeCreateParams{
		Name:      req.Name,
		ImageRef:  req.ImageRef,
		FlavorRef: req.FlavorRef,
		NetworkID: req.NetworkID,
	})
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "could not provision instance: "+err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, dto.NewComputeResponse(record))
}

// List handles GET /instances: the instances userID has provisioned, as
// tracked in our DB.
func (h *ComputeHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r)
	if !ok {
		return
	}

	records, err := h.compute.List(r.Context(), userID)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not list instances")
		return
	}

	httpx.WriteJSON(w, http.StatusOK, dto.NewComputeListResponse(records))
}

// Get handles GET /instances/{id}: live state from Nova for one of
// userID's instances, keyed by compute_service_id.
func (h *ComputeHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r)
	if !ok {
		return
	}

	serviceID := r.PathValue("id")
	server, err := h.compute.Get(r.Context(), userID, serviceID)
	if err != nil {
		if errors.Is(err, domain.ErrComputeNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "instance not found")
			return
		}
		httpx.WriteError(w, http.StatusBadGateway, "could not fetch instance: "+err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, dto.NewComputeServerResponse(server))
}

// Delete handles DELETE /instances/{id}: destroys the server in Nova and
// drops its record.
func (h *ComputeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r)
	if !ok {
		return
	}

	serviceID := r.PathValue("id")
	if err := h.compute.Delete(r.Context(), userID, serviceID); err != nil {
		if errors.Is(err, domain.ErrComputeNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "instance not found")
			return
		}
		httpx.WriteError(w, http.StatusBadGateway, "could not delete instance: "+err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// userIDFromRequest reads the user id stashed by middleware.Auth and parses
// it as a UUID, writing a 401 itself if that's ever not the case.
func userIDFromRequest(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	raw, _ := r.Context().Value(middleware.UserIDContextKey).(string)
	userID, err := uuid.Parse(raw)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid access token")
		return uuid.UUID{}, false
	}
	return userID, true
}
