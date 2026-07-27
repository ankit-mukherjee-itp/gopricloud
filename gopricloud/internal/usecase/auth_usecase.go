package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"gopricloud/gopricloud/internal/domain"
	"gopricloud/gopricloud/internal/repository"
	"gopricloud/gopricloud/internal/token"
)

// AuthResult is returned by every auth flow that hands the caller a fresh
// token pair.
type AuthResult struct {
	User                  *domain.User
	AccessToken           string
	AccessTokenExpiresAt  time.Time
	RefreshToken          string
	RefreshTokenExpiresAt time.Time
}

type AuthUsecase struct {
	users      repository.UserRepository
	tokens     repository.RefreshTokenRepository
	jwt        *token.JWTManager
	refreshTTL time.Duration
}

func NewAuthUsecase(
	users repository.UserRepository,
	tokens repository.RefreshTokenRepository,
	jwt *token.JWTManager,
	refreshTTL time.Duration,
) *AuthUsecase {
	return &AuthUsecase{
		users:      users,
		tokens:     tokens,
		jwt:        jwt,
		refreshTTL: refreshTTL,
	}
}

// Signup creates a new user and returns an initial token pair.
func (u *AuthUsecase) Signup(ctx context.Context, name, email, password string) (*AuthResult, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := domain.NewUser(name, email, string(hash))
	if err := u.users.Create(ctx, user); err != nil {
		return nil, err
	}

	return u.issueTokens(ctx, user)
}

// Signin verifies credentials and returns a fresh token pair.
func (u *AuthUsecase) Signin(ctx context.Context, email, password string) (*AuthResult, error) {
	user, err := u.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	return u.issueTokens(ctx, user)
}

// Refresh exchanges a valid, unused refresh token for a new access token
// and rotates the refresh token. If a revoked (already-used) refresh token
// is presented, every outstanding refresh token for that user is revoked,
// since reuse indicates the token was compromised.
func (u *AuthUsecase) Refresh(ctx context.Context, rawRefreshToken string) (*AuthResult, error) {
	hash := token.HashRefreshToken(rawRefreshToken)

	stored, err := u.tokens.GetByHash(ctx, hash)
	if err != nil {
		return nil, err
	}

	if stored.Revoked {
		_ = u.tokens.RevokeAllForUser(ctx, stored.UserID)
		return nil, domain.ErrInvalidRefreshToken
	}
	if !stored.IsValid(time.Now().UTC()) {
		return nil, domain.ErrInvalidRefreshToken
	}

	user, err := u.users.GetByID(ctx, stored.UserID)
	if err != nil {
		return nil, err
	}

	if err := u.tokens.Revoke(ctx, stored.ID); err != nil {
		return nil, err
	}

	return u.issueTokens(ctx, user)
}

func (u *AuthUsecase) issueTokens(ctx context.Context, user *domain.User) (*AuthResult, error) {
	accessToken, accessExpiresAt, err := u.jwt.GenerateAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}

	rawRefreshToken, err := token.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	now := time.Now().UTC()
	refreshExpiresAt := now.Add(u.refreshTTL)

	record := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: token.HashRefreshToken(rawRefreshToken),
		ExpiresAt: refreshExpiresAt,
		Revoked:   false,
		CreatedAt: now,
	}
	if err := u.tokens.Create(ctx, record); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &AuthResult{
		User:                  user,
		AccessToken:           accessToken,
		AccessTokenExpiresAt:  accessExpiresAt,
		RefreshToken:          rawRefreshToken,
		RefreshTokenExpiresAt: refreshExpiresAt,
	}, nil
}
