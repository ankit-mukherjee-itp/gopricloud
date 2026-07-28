package domain

import (
	"time"

	"github.com/google/uuid"
)

// ComputeStatusBuilding is the status recorded immediately after Nova
// accepts a create request; Nova's create response doesn't echo back the
// server's actual status, so this stands in until the caller polls Get.
const ComputeStatusBuilding = "BUILD"

// Compute is the persisted record tying a provisioned OpenStack server to
// the user who owns it.
type Compute struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	ComputeServiceID string // Nova server ID
	Name             string
	Status           string
	CreatedAt        time.Time
}

// ComputeCreateParams are the inputs needed to boot a new server in Nova.
type ComputeCreateParams struct {
	Name      string
	ImageRef  string
	FlavorRef string
	NetworkID string // optional
}

// ComputeServer is the live state of a server as reported by Nova.
type ComputeServer struct {
	ID        string
	Name      string
	Status    string
	Addresses map[string]any
}
