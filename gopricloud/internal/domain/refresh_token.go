package domain

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken is the server-side record of an issued refresh token.
// Only TokenHash is ever persisted; the raw token is returned to the
// client once and never stored.
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	Revoked   bool
	CreatedAt time.Time
}

func (r *RefreshToken) IsValid(now time.Time) bool {
	return !r.Revoked && now.Before(r.ExpiresAt)
}
