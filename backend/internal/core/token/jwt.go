package token

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidAccessToken = errors.New("invalid access token")
	ErrExpiredAccessToken = errors.New("access token expired")
)

// AccessClaims are the JWT claims carried by an access token.
type AccessClaims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
}

// JWTManager issues and verifies short-lived, self-contained access tokens.
// Refresh tokens are handled separately (see refresh.go) since they are
// opaque, server-side-tracked values rather than JWTs.
type JWTManager struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
}

func NewJWTManager(secret, issuer string, accessTTL time.Duration) *JWTManager {
	return &JWTManager{
		secret:    []byte(secret),
		issuer:    issuer,
		accessTTL: accessTTL,
	}
}

// GenerateAccessToken returns a signed access token for the given user along
// with its expiry.
func (m *JWTManager) GenerateAccessToken(userID uuid.UUID, email string) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(m.accessTTL)

	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
		Email: email,
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

// ParseAccessToken validates the signature and expiry of tokenStr and
// returns its claims.
func (m *JWTManager) ParseAccessToken(tokenStr string) (*AccessClaims, error) {
	claims := &AccessClaims{}

	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidAccessToken
		}
		return m.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredAccessToken
		}
		return nil, ErrInvalidAccessToken
	}

	return claims, nil
}
