package dto

import (
	"time"

	"gopricloud/gopricloud/internal/domain"
)

type CreateComputeRequest struct {
	Name      string `json:"name"`
	ImageRef  string `json:"image_ref"`
	FlavorRef string `json:"flavor_ref"`
	NetworkID string `json:"network_id,omitempty"`
}

// ComputeResponse describes a provisioned instance as tracked in our DB.
type ComputeResponse struct {
	ID               string    `json:"id"`
	ComputeServiceID string    `json:"compute_service_id"`
	Name             string    `json:"name"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
}

func NewComputeResponse(c *domain.Compute) ComputeResponse {
	return ComputeResponse{
		ID:               c.ID.String(),
		ComputeServiceID: c.ComputeServiceID,
		Name:             c.Name,
		Status:           c.Status,
		CreatedAt:        c.CreatedAt,
	}
}

func NewComputeListResponse(cs []domain.Compute) []ComputeResponse {
	out := make([]ComputeResponse, 0, len(cs))
	for i := range cs {
		out = append(out, NewComputeResponse(&cs[i]))
	}
	return out
}

// ComputeServerResponse describes an instance's live state as reported by
// Nova.
type ComputeServerResponse struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Status    string         `json:"status"`
	Addresses map[string]any `json:"addresses"`
}

func NewComputeServerResponse(s *domain.ComputeServer) ComputeServerResponse {
	return ComputeServerResponse{
		ID:        s.ID,
		Name:      s.Name,
		Status:    s.Status,
		Addresses: s.Addresses,
	}
}
