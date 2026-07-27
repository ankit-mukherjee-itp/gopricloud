# API Calls — Implementation Walkthrough

This document traces **every** HTTP endpoint exposed by the backend down
through the exact functions invoked, in the order they run, from the
router to the database/OpenStack call and back. Code references are
relative to `gopricloud/gopricloud/`.

For the deeper mechanics of instance provisioning specifically (what
`ComputeUsecase`, `ComputeProvider`, and `ComputeRepository` do internally),
see [`Provisioning.md`](Provisioning.md) — this document covers the HTTP
layer and the auth flow in full, and only summarizes the compute calls
before pointing there.

## Routing

`internal/delivery/http/router.go` builds a single stdlib `http.ServeMux`
(Go 1.22+ method+path patterns) in `NewRouter(authHandler, testHandler,
computeHandler, jwtManager)`:

```go
mux.HandleFunc("POST /signup", authHandler.Signup)
mux.HandleFunc("POST /signin", authHandler.Signin)
mux.HandleFunc("POST /refresh", authHandler.Refresh)

requireAuth := middleware.Auth(jwtManager)
mux.Handle("GET /test", requireAuth(http.HandlerFunc(testHandler.Test)))

mux.Handle("POST /instances", requireAuth(http.HandlerFunc(computeHandler.Create)))
mux.Handle("GET /instances", requireAuth(http.HandlerFunc(computeHandler.List)))
mux.Handle("GET /instances/{id}", requireAuth(http.HandlerFunc(computeHandler.Get)))
mux.Handle("DELETE /instances/{id}", requireAuth(http.HandlerFunc(computeHandler.Delete)))
```

`requireAuth` is `middleware.Auth(jwtManager)` — every route except the
three auth endpoints is wrapped in it, so the handler function only ever
runs once a valid access token has been verified.

## Shared helpers used by every endpoint

### `httpx.WriteJSON` / `httpx.WriteError` (`internal/delivery/http/httpx/respond.go`)

Every handler response goes through one of these two functions:

- `WriteJSON(w, status, body)` — sets `Content-Type: application/json`,
  writes the status code, and (if `body` isn't nil) JSON-encodes it to the
  response.
- `WriteError(w, status, message)` — calls `WriteJSON` with a
  `dto.ErrorResponse{Error: message}`, so every error response has the same
  `{"error": "..."}` shape.

### `middleware.Auth` (`internal/delivery/http/middleware/auth_middleware.go`)

Wraps a handler and runs before it on every protected route:

1. Reads the `Authorization` header, requires the `Bearer ` prefix —
   `httpx.WriteError(w, 401, "missing bearer token")` if absent.
2. Calls `jwtManager.ParseAccessToken(raw)` (`internal/token/jwt.go`), which:
   - Parses the JWT via `jwt.ParseWithClaims`, checking it was signed with
     HMAC and validating the signature against the server's secret.
   - Returns `token.ErrExpiredAccessToken` if `jwt.ErrTokenExpired` was the
     underlying cause, or `token.ErrInvalidAccessToken` otherwise.
3. On failure: `httpx.WriteError(w, 401, "access token expired")` or
   `"invalid access token"`.
4. On success: stashes the token's `Subject` (user ID) and `Email` claims
   into the request context under `middleware.UserIDContextKey` /
   `UserEmailContextKey`, then calls the wrapped handler.

### `userIDFromRequest` (`internal/delivery/http/handler/compute_handler.go`)

Used by every compute handler to recover the authenticated user:

```go
func userIDFromRequest(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
    raw, _ := r.Context().Value(middleware.UserIDContextKey).(string)
    userID, err := uuid.Parse(raw)
    if err != nil {
        httpx.WriteError(w, http.StatusUnauthorized, "invalid access token")
        return uuid.UUID{}, false
    }
    return userID, true
}
```

Reads the user ID that `middleware.Auth` put in the context (as a string)
and parses it into a `uuid.UUID`. If a handler gets this `false` back, it
returns immediately — the error response was already written.

---

## `POST /signup`

**Handler:** `AuthHandler.Signup` (`internal/delivery/http/handler/auth_handler.go`)

1. `json.NewDecoder(r.Body).Decode(&req)` into `dto.SignupRequest{Name, Email, Password}`.
2. Trims `Name`, trims+lowercases `Email`.
3. Validates all three fields are non-empty and `len(Password) >= 8`,
   writing a 400 via `httpx.WriteError` otherwise.
4. Calls `h.auth.Signup(r.Context(), req.Name, req.Email, req.Password)`.

**Usecase:** `AuthUsecase.Signup` (`internal/usecase/auth_usecase.go`)

1. `bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)` to hash the password.
2. `domain.NewUser(name, email, string(hash))` — builds a `domain.User`
   with a fresh `uuid.New()` ID and `CreatedAt`/`UpdatedAt` set to now.
3. `u.users.Create(ctx, user)` — the `UserRepository` port, implemented by
   `postgres.userRepository.Create` (`internal/infrastructure/postgres/user_repo.go`):
   runs an `INSERT INTO users (...)`; if Postgres returns a unique-violation
   (`pgErr.Code == "23505"`, i.e. the email already exists), it's mapped to
   `domain.ErrUserAlreadyExists`.
4. `u.issueTokens(ctx, user)` (see **Token issuance** below).

**Handler (continued):** on `domain.ErrUserAlreadyExists` → 409; any other
error → 500; success → `httpx.WriteJSON(w, 201, dto.NewAuthResponse(result))`.

`dto.NewAuthResponse` (`internal/delivery/http/dto/auth_dto.go`) maps the
usecase's `*usecase.AuthResult` onto the wire shape: nested `user` object
(`id`, `name`, `email`) plus `access_token`, `access_token_expires_at`,
`refresh_token`, `refresh_token_expires_at`.

> 📸 *Screenshot: `POST /signup` request + response*
>
> _(space reserved — paste screenshot here)_

---

## `POST /signin`

**Handler:** `AuthHandler.Signin`

1. Decodes `dto.SigninRequest{Email, Password}`.
2. Trims+lowercases `Email`; 400 if either field is empty.
3. Calls `h.auth.Signin(r.Context(), req.Email, req.Password)`.

**Usecase:** `AuthUsecase.Signin`

1. `u.users.GetByEmail(ctx, email)` → `postgres.userRepository.GetByEmail`
   runs `SELECT ... FROM users WHERE email = $1` and scans into a
   `domain.User` via the shared `scanUser` helper. `sql.ErrNoRows` is
   mapped to `domain.ErrUserNotFound`, which the usecase turns into the
   deliberately-vague `domain.ErrInvalidCredentials` (so the API doesn't
   reveal whether the email exists).
2. `bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))`
   — any mismatch also becomes `domain.ErrInvalidCredentials`.
3. `u.issueTokens(ctx, user)`.

**Handler (continued):** `domain.ErrInvalidCredentials` → 401; success →
200 with the same `dto.NewAuthResponse(result)` shape as signup.

> 📸 *Screenshot: `POST /signin` request + response*
>
> _(space reserved — paste screenshot here)_

---

## `POST /refresh`

**Handler:** `AuthHandler.Refresh`

1. Decodes `dto.RefreshRequest{RefreshToken}`; 400 if blank.
2. Calls `h.auth.Refresh(r.Context(), req.RefreshToken)`.

**Usecase:** `AuthUsecase.Refresh`

1. `token.HashRefreshToken(rawRefreshToken)` (`internal/token/refresh.go`)
   — SHA-256 hex digest of the raw token, since only the hash is stored.
2. `u.tokens.GetByHash(ctx, hash)` → `postgres.refreshTokenRepository.GetByHash`
   (`internal/infrastructure/postgres/token_repo.go`) runs
   `SELECT ... FROM refresh_tokens WHERE token_hash = $1`; no match →
   `domain.ErrInvalidRefreshToken`.
3. If `stored.Revoked` is already `true` (someone is replaying a token that
   was already exchanged): `u.tokens.RevokeAllForUser(ctx, stored.UserID)`
   revokes every outstanding refresh token for that user, and the call
   fails with `domain.ErrInvalidRefreshToken` — reuse of a dead token is
   treated as a compromise signal.
4. `stored.IsValid(time.Now().UTC())` (`domain.RefreshToken.IsValid`,
   `internal/domain/refresh_token.go`) — `!Revoked && now.Before(ExpiresAt)`.
   Fails the same way if expired.
5. `u.users.GetByID(ctx, stored.UserID)` → `postgres.userRepository.GetByID`
   (same `scanUser` helper as above, keyed by primary key this time).
6. `u.tokens.Revoke(ctx, stored.ID)` → `postgres.refreshTokenRepository.Revoke`
   runs `UPDATE refresh_tokens SET revoked = TRUE WHERE id = $1` —
   the token just used is burned so it can't be replayed.
7. `u.issueTokens(ctx, user)` — issues and stores a **new** access/refresh
   pair (rotation).

**Handler (continued):** `domain.ErrInvalidRefreshToken` → 401; success →
200 with a fresh `dto.NewAuthResponse(result)`.

> 📸 *Screenshot: `POST /refresh` request + response*
>
> _(space reserved — paste screenshot here)_

---

## Token issuance — `AuthUsecase.issueTokens` (shared by signup/signin/refresh)

1. `u.jwt.GenerateAccessToken(user.ID, user.Email)` (`internal/token/jwt.go`)
   — builds `AccessClaims{RegisteredClaims{Subject: userID, Issuer, IssuedAt,
   ExpiresAt: now + 5m}, Email}` and signs it HS256 with the server's
   `JWT_SECRET`.
2. `token.GenerateRefreshToken()` (`internal/token/refresh.go`) — 32 random
   bytes from `crypto/rand`, base64 (URL-safe, unpadded) encoded.
3. `token.HashRefreshToken(rawRefreshToken)` — SHA-256 hex digest, stored
   instead of the raw value.
4. Builds a `domain.RefreshToken{ID: uuid.New(), UserID, TokenHash,
   ExpiresAt: now + 7d, Revoked: false, CreatedAt: now}`.
5. `u.tokens.Create(ctx, record)` → `postgres.refreshTokenRepository.Create`
   runs `INSERT INTO refresh_tokens (...)`.
6. Returns `*usecase.AuthResult` with the user, the raw access token +
   expiry, and the raw refresh token + expiry (the raw refresh token is
   only ever available here — after this call only its hash exists).

---

## `GET /test`

**Handler:** `TestHandler.Test` (`internal/delivery/http/handler/test_handler.go`)

Runs only after `middleware.Auth` has already validated the access token.
The handler itself does nothing but confirm that: `httpx.WriteJSON(w, 200,
struct{}{})` — an empty JSON object, proving the caller holds a valid
token without exposing any resource.

> 📸 *Screenshot: `GET /test` request + response (with and without a token)*
>
> _(space reserved — paste screenshot here)_

---

## `POST /instances`, `GET /instances`, `GET /instances/{id}`, `DELETE /instances/{id}`

All four are handled by `ComputeHandler` (`internal/delivery/http/handler/compute_handler.go`)
and all follow the same shape: `userIDFromRequest` → decode/validate
(create only) → call the matching `ComputeUsecase` method → map domain
errors to HTTP status codes → `httpx.WriteJSON`/`WriteError`.

| Route | Handler method | Usecase method | Success status |
|---|---|---|---|
| `POST /instances` | `Create` | `ComputeUsecase.Create` | 201 |
| `GET /instances` | `List` | `ComputeUsecase.List` | 200 |
| `GET /instances/{id}` | `Get` | `ComputeUsecase.Get` | 200 |
| `DELETE /instances/{id}` | `Delete` | `ComputeUsecase.Delete` | 204 |

`{id}` is read via `r.PathValue("id")` and is the Nova server ID
(`compute_service_id`), not the internal `compute` row's own `id`.

Error mapping used by `Get` and `Delete`: `domain.ErrComputeNotFound` → 404
(this covers both "no such row for this user" and "Nova returned 404" —
see `Provisioning.md`); any other error from the usecase → 502, since it
means the OpenStack call itself failed.

`Create`'s request body (`dto.CreateComputeRequest{Name, ImageRef,
FlavorRef, NetworkID}`) requires `Name`, `ImageRef`, `FlavorRef` (400 if
any are blank); `NetworkID` is optional. Responses are shaped by
`dto.NewComputeResponse` (for `Create`/each item in `List`, via
`dto.NewComputeListResponse`) and `dto.NewComputeServerResponse` (for
`Get`) — see `Provisioning.md` for exactly what data populates them and
what happens inside `ComputeUsecase`, `ComputeProvider`, and
`ComputeRepository`.

> 📸 *Screenshot: `POST /instances` request + response*
>
> _(space reserved — paste screenshot here)_

> 📸 *Screenshot: `GET /instances` request + response*
>
> _(space reserved — paste screenshot here)_

> 📸 *Screenshot: `GET /instances/{id}` request + response*
>
> _(space reserved — paste screenshot here)_

> 📸 *Screenshot: `DELETE /instances/{id}` request + response*
>
> _(space reserved — paste screenshot here)_
