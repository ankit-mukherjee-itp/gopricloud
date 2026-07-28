# API Calls — every endpoint, traced function by function

How each HTTP request travels through the backend: **router → middleware →
handler → DTO → service (usecase) → repository / provider → back out as JSON**.

All paths are relative to `backend/`. Import paths are `backend/...` (the Go
module is named `backend` and `go.mod` lives in `backend/`).

For the compute/provisioning flows in full depth — including every gophercloud
and SQL call — see [Provisioning.md](Provisioning.md). This file covers the HTTP
layer and the auth flows.

---

## 1. Composition root — what exists before any request arrives

`cmd/main.go` is the only place concrete types are constructed and joined. Nothing
else in the codebase knows which database or which cloud is in use.

```
func init() { configs.LoadEnv() }        // load .env beside go.mod, if present

run():
  signal.NotifyContext(SIGINT, SIGTERM)  // ctx cancelled on shutdown signal
  configs.Load()                         // -> *configs.Config (validates required vars)
  postgres.Open(ctx, cfg.DatabaseURL)    // connect + apply schema
  ├── userRepo        = postgres.NewUserRepository(db)          // ports.UserRepository
  ├── tokenRepo       = postgres.NewRefreshTokenRepository(db)  // ports.RefreshTokenRepository
  ├── computeRepo     = postgres.NewComputeRepository(db)       // ports.ComputeRepository
  └── computeProvider = openstack.NewProvider(cfg.OSCloudName)  // ports.ComputeProvider
  ├── jwtManager      = token.NewJWTManager(secret, issuer, 5m)
  ├── authService     = services.NewAuthUsecase(userRepo, tokenRepo, jwtManager, 7d)
  └── computeService  = services.NewComputeUsecase(computeRepo, computeProvider)
  ├── authHandler     = rest.NewAuthHandler(authService)
  ├── testHandler     = rest.NewTestHandler()
  ├── computeHandler  = rest.NewComputeHandler(computeService)
  └── router          = rest.NewRouter(authHandler, testHandler, computeHandler, jwtManager)
  server.Serve(ctx, router, cfg.Port)
```

Note the direction: the repositories and the provider are **concrete structs**
returned as **port interfaces**, then injected inward. The services only ever see
interfaces, which is what keeps `internal/core/**` free of any import of an
adapter.

`cmd/server/server.go` → `Serve(ctx, handler, port)` builds the `http.Server`
(`ReadHeaderTimeout: 5s`), starts `ListenAndServe` in a goroutine reporting into
an `errCh`, logs `listening on :<port>`, and on `ctx.Done()` performs a graceful
`Shutdown` with a 10-second timeout, logging `shutting down`.

### Startup side effect: schema bootstrap

`postgres.Open` (`internal/adapters/repositories/postgres/db.go`) does three
things in order — `sql.Open("pgx", dsn)`, `db.PingContext`, then
`db.ExecContext(schema)`. The `schema` constant is idempotent
(`CREATE TABLE IF NOT EXISTS`), so there is **no migration tool**:

| Table | Columns |
|---|---|
| `users` | `id` UUID PK, `name`, `email` UNIQUE, `password_hash`, `created_at`, `updated_at` |
| `refresh_tokens` | `id` UUID PK, `user_id` → `users(id)` ON DELETE CASCADE, `token_hash` UNIQUE, `expires_at`, `revoked` DEFAULT FALSE, `created_at` |
| `compute` | `id` UUID PK, `user_id` → `users(id)` ON DELETE CASCADE, `compute_service_id` TEXT UNIQUE, `name`, `status`, `created_at` |

Plus `idx_refresh_tokens_user_id` and `idx_compute_user_id`.

Because `Ping` happens before the schema exec, an unreachable database fails at
startup — unlike OpenStack, which fails lazily (see Provisioning.md).

> 📸 *Screenshot: backend starting up — `listening on :8080`*

---

## 2. The router

`internal/adapters/handlers/rest/routes.go` → `NewRouter(...) http.Handler`.
A plain `http.ServeMux` using Go 1.22+ **method+path patterns**, so the method is
matched by the mux itself — handlers never check `r.Method`.

| Pattern | Handler | Auth |
|---|---|---|
| `POST /signup` | `authHandler.Signup` | public |
| `POST /signin` | `authHandler.Signin` | public |
| `POST /refresh` | `authHandler.Refresh` | public (the refresh token *is* the credential) |
| `GET /test` | `testHandler.Test` | `requireAuth` |
| `POST /instances` | `computeHandler.Create` | `requireAuth` |
| `GET /instances` | `computeHandler.List` | `requireAuth` |
| `GET /instances/{id}` | `computeHandler.Get` | `requireAuth` |
| `DELETE /instances/{id}` | `computeHandler.Delete` | `requireAuth` |

`requireAuth := middleware.Auth(jwtManager)` is built once and wraps each
protected handler individually via
`mux.Handle(pattern, requireAuth(http.HandlerFunc(...)))`. There is no global
middleware chain — the public auth routes use `mux.HandleFunc` and are never
wrapped.

Two consequences worth knowing:

- **`{id}` is read with `r.PathValue("id")`**, a stdlib feature, so there is no
  router library anywhere in the project.
- **A wrong method returns 405 from the mux**, before any handler runs. This is
  exactly why the frontend needs a method-aware proxy exception for `/signup`:
  that path is both an SPA route (GET) and an API endpoint (POST only).

### No CORS

There is no CORS middleware, deliberately. The browser only ever makes
**same-origin** requests: in dev, Vite proxies `/signup`, `/signin`, `/refresh`,
`/test`, `/instances` to `:8080`; in Docker, nginx reverse-proxies the same paths.

---

## 3. Shared response helpers

`internal/adapters/api/response.go` — the only place a response is serialized.

```go
func WriteJSON(w http.ResponseWriter, status int, body any)
```
Sets `Content-Type: application/json`, writes the status, then encodes `body`
unless it is `nil`. Order matters: the header must be set before `WriteHeader`.

```go
func WriteError(w http.ResponseWriter, status int, message string)
```
Delegates to `WriteJSON` with `dto.ErrorResponse{Error: message}`, so **every**
error in the API has the same shape:

```json
{ "error": "invalid email or password" }
```

The encode error is deliberately discarded (`_ =`): the status line is already
committed by then, so there is nothing useful left to do.

---

## 4. The auth middleware

`internal/adapters/handlers/rest/middleware/auth.middleware.go` →
`Auth(jwtManager) func(http.Handler) http.Handler`.

Step by step, per request:

1. Read `Authorization`. If it does not start with the literal `"Bearer "` →
   **401** `{"error":"missing bearer token"}`. (A missing header fails the same
   prefix check.)
2. `strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))` → the raw JWT.
3. `jwtManager.ParseAccessToken(raw)`:
   - `errors.Is(err, token.ErrExpiredAccessToken)` → **401**
     `{"error":"access token expired"}`
   - any other error → **401** `{"error":"invalid access token"}`
4. On success, stash two values in the request context and call the next handler
   with `r.WithContext(ctx)`:
   - `middleware.UserIDContextKey` → `claims.Subject` (the user UUID as a string)
   - `middleware.UserEmailContextKey` → `claims.Email`

The keys are a private `type contextKey string`, so no other package can collide
with them by using a bare string.

**The distinct "expired" message is load-bearing for the frontend**: it is the
signal that a refresh is worth attempting, rather than a hard logout.

### Reading the caller back out

`compute.handler.go` → `userIDFromRequest(w, r) (uuid.UUID, bool)`:

```go
raw, _ := r.Context().Value(middleware.UserIDContextKey).(string)
userID, err := uuid.Parse(raw)
if err != nil { api.WriteError(w, 401, "invalid access token"); return uuid.UUID{}, false }
```

Every compute handler starts with this and returns immediately on `!ok`. The
helper writes its own 401, so the handler does not. In practice this cannot fail
behind `requireAuth` — it is a defence-in-depth guard, not a real code path.

---

## 5. Token design (`internal/core/token`)

### Access token — `jwt.go`

- **HS256 JWT**, TTL **5 minutes** (`cfg.AccessTokenTTL`), never persisted.
- `AccessClaims` embeds `jwt.RegisteredClaims` and adds `Email string`.
- `GenerateAccessToken(userID, email) (string, time.Time, error)` sets `Subject`
  (user UUID), `Issuer` (`JWT_ISSUER`, default `gopricloud`), `IssuedAt`,
  `ExpiresAt`, then signs with the secret.
- `ParseAccessToken(tokenStr) (*AccessClaims, error)` — the keyfunc **asserts the
  algorithm is HMAC** before returning the secret:

  ```go
  if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok { return nil, ErrInvalidAccessToken }
  ```

  That check is what blocks the classic algorithm-confusion attack (a token
  claiming `alg: none`, or an RSA-signed token verified with the HMAC secret as a
  public key). It maps `jwt.ErrTokenExpired` → `ErrExpiredAccessToken` and
  everything else → `ErrInvalidAccessToken`.

### Refresh token — `refresh.go`

- `GenerateRefreshToken()` — **32 random bytes** from `crypto/rand`, encoded
  `base64.RawURLEncoding`. Opaque: it carries no claims and is not a JWT.
- `HashRefreshToken(raw)` — SHA-256, hex-encoded.
- **Only the hash is ever stored.** The raw value goes to the client exactly once,
  in the response body. A database leak therefore does not yield usable tokens.
- SHA-256 (not bcrypt) is correct here: the input is already 256 bits of entropy,
  so it is not brute-forceable, and lookup must be an indexed exact match.

---

## 6. `POST /signup`

**Chain:** `mux` → `AuthHandler.Signup` → `AuthUsecase.Signup` →
`bcrypt` + `domain.NewUser` → `userRepository.Create` → `issueTokens`

### Request

```json
{ "name": "Ada", "email": "ada@example.com", "password": "supersecret123" }
```

### `AuthHandler.Signup` (`auth.handler.go`)

1. `json.NewDecoder(r.Body).Decode(&dto.SignupRequest)` → on failure **400**
   `invalid request body`.
2. Normalize: `Name` trimmed; `Email` **trimmed and lowercased** — so emails are
   case-insensitive, which matters because the DB has a plain `UNIQUE` constraint
   on `email` rather than a functional index.
3. Any of name/email/password empty → **400**
   `name, email and password are required`.
4. `len(Password) < 8` → **400** `password must be at least 8 characters`.
   This is the only password policy, and it is enforced **only here** — the
   service does not re-check.
5. Call `h.auth.Signup(ctx, name, email, password)`.
6. `domain.ErrUserAlreadyExists` → **409** `a user with this email already exists`;
   any other error → **500** `could not create user`.
7. Success → **201** with `dto.NewAuthResponse(result)`.

### `AuthUsecase.Signup` (`internal/core/services/auth.service.go`)

1. `bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)` — wraps a
   failure as `hash password: %w`.
2. `domain.NewUser(name, email, string(hash))` — assigns `uuid.New()` and sets
   `CreatedAt = UpdatedAt = time.Now().UTC()`.
3. `u.users.Create(ctx, user)` — error returned unwrapped, so the handler's
   `errors.Is(err, domain.ErrUserAlreadyExists)` works.
4. `u.issueTokens(ctx, user)`.

### `userRepository.Create` (`postgres/user.repository.go`)

```sql
INSERT INTO users (id, name, email, password_hash, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
```

Translates the driver error into a domain error:

```go
var pgErr *pgconn.PgError
if errors.As(err, &pgErr) && pgErr.Code == "23505" { return domain.ErrUserAlreadyExists }
```

`23505` is Postgres' `unique_violation`. This is the boundary where an
infrastructure detail becomes a domain concept — the service and handler above
never mention Postgres.

### `issueTokens` — shared by signup, signin and refresh

1. `jwt.GenerateAccessToken(user.ID, user.Email)` → token + expiry.
2. `token.GenerateRefreshToken()` → raw opaque token.
3. `now := time.Now().UTC()`; `refreshExpiresAt = now.Add(refreshTTL)` (7 days).
4. Build `&domain.RefreshToken{ID: uuid.New(), UserID, TokenHash: HashRefreshToken(raw), ExpiresAt, Revoked: false, CreatedAt: now}`.
5. `u.tokens.Create(ctx, record)` → `INSERT INTO refresh_tokens (...)`.
6. Return `*AuthResult{User, AccessToken, AccessTokenExpiresAt, RefreshToken (raw), RefreshTokenExpiresAt}`.

### Response — 201

`dto.NewAuthResponse` maps `services.AuthResult` to the wire shape. Note the
**snake_case** JSON keys, and that `password_hash` is absent because
`UserResponse` only has `id`, `name`, `email`:

```json
{
  "user": { "id": "6491a0c8-...", "name": "Ada", "email": "ada@example.com" },
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "access_token_expires_at": "2026-07-28T13:37:37.439531245Z",
  "refresh_token": "Ho2J6Nov-g401weI9XC_yeAt7EZOrhDiQ3DyI8Qwlq0",
  "refresh_token_expires_at": "2026-08-04T13:32:37.439611924Z"
}
```

> 📸 *Screenshot: `POST /signup` request and 201 response*

---

## 7. `POST /signin`

**Chain:** `mux` → `AuthHandler.Signin` → `AuthUsecase.Signin` →
`userRepository.GetByEmail` → `bcrypt.CompareHashAndPassword` → `issueTokens`

1. Decode `dto.SigninRequest{email, password}`; bad JSON → **400**.
2. Email trimmed + lowercased (matching signup). Either field empty → **400**
   `email and password are required`.
3. `AuthUsecase.Signin`:
   - `u.users.GetByEmail(ctx, email)`. The repository's `scanUser` maps
     `sql.ErrNoRows` → `domain.ErrUserNotFound`; the service then **converts that
     to `domain.ErrInvalidCredentials`**.
   - `bcrypt.CompareHashAndPassword(user.PasswordHash, password)` → on mismatch,
     also `domain.ErrInvalidCredentials`.
   - `u.issueTokens(ctx, user)`.
4. `domain.ErrInvalidCredentials` → **401** `invalid email or password`; other →
   **500** `could not sign in`. Success → **200**, same `AuthResponse` shape.

**Unknown email and wrong password are deliberately indistinguishable** — same
error value, same status, same message. That prevents user enumeration.

One honest caveat: the unknown-email path returns *before* any bcrypt comparison,
so it is measurably faster. The responses are identical, but the timing is not.

> 📸 *Screenshot: `POST /signin` returning a fresh token pair*

---

## 8. `POST /refresh`

The endpoint the client calls when the 5-minute access token expires. Public,
because the refresh token itself is the credential.

**Chain:** `mux` → `AuthHandler.Refresh` → `AuthUsecase.Refresh` →
`HashRefreshToken` → `tokenRepo.GetByHash` → validity/replay checks →
`userRepo.GetByID` → `tokenRepo.Revoke` → `issueTokens`

### Handler

1. Decode `dto.RefreshRequest{refresh_token}`; bad JSON → **400**.
2. Empty after trim → **400** `refresh_token is required`.
3. `domain.ErrInvalidRefreshToken` → **401** `invalid or expired refresh token`;
   other → **500** `could not refresh session`. Success → **200** with a full new
   `AuthResponse` — **both** tokens are new.

### `AuthUsecase.Refresh` — rotation and replay detection

```go
hash := token.HashRefreshToken(rawRefreshToken)     // never look up by raw value
stored, err := u.tokens.GetByHash(ctx, hash)        // ErrNoRows -> ErrInvalidRefreshToken
if stored.Revoked {
    _ = u.tokens.RevokeAllForUser(ctx, stored.UserID)   // replay -> nuke the family
    return nil, domain.ErrInvalidRefreshToken
}
if !stored.IsValid(time.Now().UTC()) {              // !Revoked && now < ExpiresAt
    return nil, domain.ErrInvalidRefreshToken
}
user, err := u.users.GetByID(ctx, stored.UserID)
if err := u.tokens.Revoke(ctx, stored.ID); err != nil { return nil, err }  // rotate
return u.issueTokens(ctx, user)                     // brand-new pair
```

Three properties fall out of this:

1. **Rotation** — the presented token is revoked (`UPDATE refresh_tokens SET
   revoked = TRUE WHERE id = $1`) and a new one is issued. A refresh token is
   single-use.
2. **Replay detection** — presenting an *already revoked* token means either an
   attacker stole it or the legitimate client replayed it. Either way it is
   treated as compromise, and `RevokeAllForUser` (`... WHERE user_id = $1 AND
   revoked = FALSE`) invalidates every outstanding token for that user, forcing a
   full re-login.
3. **Ordering** — `Revoke` happens *before* `issueTokens`. If issuing failed
   after revocation, the client would need to log in again; the reverse order
   could leave the old token valid alongside a new one.

The `RevokeAllForUser` error is deliberately discarded (`_ =`): the request is
being rejected regardless, and the return value must stay
`ErrInvalidRefreshToken` so it maps to 401 rather than 500.

> 📸 *Screenshot: `POST /refresh` rotating the token pair*

---

## 9. `GET /test`

The simplest protected endpoint — it exists purely to confirm a token works.

**Chain:** `mux` → `requireAuth` → `TestHandler.Test`

```go
func (h *TestHandler) Test(w http.ResponseWriter, r *http.Request) {
    api.WriteJSON(w, http.StatusOK, struct{}{})
}
```

`TestHandler` is an empty struct with no dependencies. It touches no service, no
database and no cloud, so all the meaning is in whether you reach it at all:

| Condition | Result |
|---|---|
| No / malformed `Authorization` | **401** `missing bearer token` |
| Expired token | **401** `access token expired` |
| Bad signature or wrong alg | **401** `invalid access token` |
| Valid token | **200** `{}` |

---

## 10. Compute endpoints — HTTP layer

Full internals are in [Provisioning.md](Provisioning.md); this is the HTTP
contract only.

### `POST /instances` → 201

Body `dto.CreateComputeRequest`:

```json
{ "name": "web-01", "image_ref": "<image-uuid>", "flavor_ref": "<flavor-id>", "network_id": "<optional>" }
```

`name` trimmed; `name`, `image_ref`, `flavor_ref` all required → else **400**
`name, image_ref and flavor_ref are required`. `network_id` is optional. Any
provisioning error → **502** `could not provision instance: <err>`. Success →
**201** `ComputeResponse`, whose `status` is always the constant `"BUILD"`.

### `GET /instances` → 200

Reads **only our database**, never Nova. Error → **500** `could not list
instances`. Returns a JSON array; `dto.NewComputeListResponse` uses
`make([]ComputeResponse, 0, len(cs))`, so an empty result serializes as `[]`
rather than `null` — the frontend can iterate unconditionally.

### `GET /instances/{id}` → 200

`{id}` is the **`compute_service_id`** (the Nova server ID), not our row's `id`.
Ownership is checked first, then live state is fetched from Nova.
`domain.ErrComputeNotFound` → **404** `instance not found`; other → **502**
`could not fetch instance: <err>`. Returns `ComputeServerResponse`
(`id`, `name`, `status`, `addresses`).

### `DELETE /instances/{id}` → 204

`domain.ErrComputeNotFound` → **404**; other → **502** `could not delete
instance: <err>`. Success → **204 No Content** via a bare
`w.WriteHeader(http.StatusNoContent)` — the only response in the API that does
not go through `api.WriteJSON`, and correctly has no body.

> 📸 *Screenshot: dashboard listing instances*
>
> 📸 *Screenshot: `GET /instances/{id}` JSON detail in the UI*

---

## 11. Status code reference

| Status | Meaning | Raised by |
|---|---|---|
| 200 | OK | signin, refresh, `/test`, list, get |
| 201 | Created | signup, create instance |
| 204 | No Content | delete instance |
| 400 | Malformed JSON or failed field validation | all handlers |
| 401 | Bad credentials, or missing/invalid/expired access token | signin, refresh, `middleware.Auth` |
| 404 | Instance not owned by caller, or gone from Nova | get, delete |
| 405 | Wrong method for the path | the `ServeMux` itself |
| 409 | Email already registered | signup |
| 500 | Unexpected internal failure | all handlers |
| 502 | OpenStack call failed | create, get, delete |

Domain error → HTTP status mapping, all via `errors.Is` in the handlers:

| Domain error | Status |
|---|---|
| `domain.ErrUserAlreadyExists` | 409 |
| `domain.ErrInvalidCredentials` | 401 |
| `domain.ErrInvalidRefreshToken` | 401 |
| `domain.ErrComputeNotFound` | 404 |
| `token.ErrExpiredAccessToken` | 401 (`access token expired`) |
| `token.ErrInvalidAccessToken` | 401 (`invalid access token`) |
| `domain.ErrUserNotFound` | never surfaces — converted to `ErrInvalidCredentials` |

---

## 12. How the frontend consumes this

`frontend/src/lib/api.ts` is the only place `fetch` is called.

- Every request carries `Authorization: Bearer <access token>` from
  `localStorage` (see `src/lib/tokens.ts`).
- **On any 401** it calls `POST /refresh` **once** and retries the original
  request. Concurrent 401s share a single in-flight refresh promise, so a
  dashboard firing several requests at once triggers exactly one `/refresh` —
  important, because refresh tokens rotate and parallel refreshes would revoke
  each other and trip the replay detector.
- If the refresh itself fails, it clears the session and dispatches a `window`
  event `auth:unauthorized`. `src/context/AuthContext.tsx` listens for it and logs
  the user out, which is what keeps `api.ts` free of any React dependency.

---

## 13. Verified end-to-end examples

Run against the Docker stack through nginx on `:5173` (which proxies to the
backend). Against the backend directly, use `http://localhost:8080`. The status
codes below are the ones actually observed.

```bash
# 201
curl -i -X POST http://localhost:5173/signup \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ada","email":"ada@example.com","password":"supersecret123"}'

# 200 -> capture .access_token (snake_case!)
curl -s -X POST http://localhost:5173/signin \
  -H 'Content-Type: application/json' \
  -d '{"email":"ada@example.com","password":"supersecret123"}'

TOKEN=...        # the access_token value

curl -i http://localhost:5173/test                                   # 401 missing bearer token
curl -i http://localhost:5173/test -H "Authorization: Bearer $TOKEN"  # 200 {}
curl -i http://localhost:5173/instances -H "Authorization: Bearer $TOKEN"  # 200 []

# 200, and the OLD refresh token is now revoked
curl -s -X POST http://localhost:5173/refresh \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"<raw refresh token>"}'

# replay the SAME refresh token again -> 401, and every token for that user is revoked
```

A gotcha that costs real time: the JSON field is **`access_token`**, not
`accessToken`. Reading the wrong key yields an empty string and every subsequent
call returns a confusing `401 missing bearer token`.
