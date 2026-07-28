# Provisioning — instance lifecycle, function by function

Everything involved in **creating, listing, inspecting and destroying** an
OpenStack Nova instance: `ComputeUsecase`, the `ComputeProvider` (gophercloud)
adapter, and the `ComputeRepository` (Postgres) adapter.

All paths are relative to `backend/`. For the HTTP layer (routing, middleware,
status codes) see [API_Calls.md](API_Calls.md).

---

## 1. The three pieces and the one rule

```
        ComputeUsecase  (internal/core/services/compute.service.go)
                 │
      ┌──────────┴──────────┐
      ▼                     ▼
ComputeRepository      ComputeProvider          <- ports (interfaces)
      │                     │
      ▼                     ▼
postgres.computeRepository  openstack.Provider   <- adapters (implementations)
  (the `compute` table)       (Nova via gophercloud)
```

`ComputeUsecase` is the **only** type that holds both, and it keeps them in
sync: every mutating operation touches both sides. It knows nothing about SQL or
gophercloud — only the two interfaces.

**The rule that shapes all four flows: our database is the record of ownership,
Nova is the record of truth.** The DB answers "may this user touch this
instance?"; Nova answers "what is this instance actually doing?" Neither is asked
the other's question.

---

## 2. Domain types (`internal/core/domain/compute.go`)

```go
const ComputeStatusBuilding = "BUILD"

type Compute struct {                  // our persisted ownership record
    ID               uuid.UUID         // our row id
    UserID           uuid.UUID         // owner
    ComputeServiceID string            // Nova server ID
    Name             string
    Status           string
    CreatedAt        time.Time
}

type ComputeCreateParams struct {      // inputs needed to boot a server
    Name      string
    ImageRef  string
    FlavorRef string
    NetworkID string                   // optional
}

type ComputeServer struct {            // live state as reported by Nova
    ID        string
    Name      string
    Status    string
    Addresses map[string]any
}
```

Two distinct types for two distinct things, and the distinction matters:

- `Compute` is **ours** — a row, with an owner, that exists so a user's dashboard
  can be repopulated after signing back in.
- `ComputeServer` is **Nova's** — live, unowned, never persisted.

`Addresses` is `map[string]any` because Nova's shape varies by network
configuration; it is passed through to the client verbatim rather than modelled.

The identifier used in URLs is **`compute_service_id`** (the Nova ID), not
`Compute.ID`. `Compute.ID` never appears in a request path.

---

## 3. The ports

`internal/core/ports/compute-provider.interface.go`:

```go
type ComputeProvider interface {
    CreateServer(ctx context.Context, params domain.ComputeCreateParams) (*domain.ComputeServer, error)
    GetServer(ctx context.Context, serviceID string) (*domain.ComputeServer, error)
    DeleteServer(ctx context.Context, serviceID string) error
}
```

`internal/core/ports/compute-repository.interface.go`:

```go
type ComputeRepository interface {
    Create(ctx context.Context, c *domain.Compute) error
    ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Compute, error)
    GetByServiceIDAndUserID(ctx context.Context, serviceID string, userID uuid.UUID) (*domain.Compute, error)
    DeleteByServiceIDAndUserID(ctx context.Context, serviceID string, userID uuid.UUID) error
}
```

Note the repository has **no plain `GetByServiceID`**. Every single-row lookup
requires the user id too. Ownership is not something a caller can forget to
check, because there is no unfiltered method to call.

> Stale comment warning: the doc comment on `ComputeProvider` still says the
> implementation "lives outside `internal/`, under `openstack/compute`". That was
> true before the refactor; it now lives at
> `internal/adapters/providers/openstack`.

---

## 4. The OpenStack adapter

### `NewClient` (`internal/adapters/providers/openstack/client.go`)

```go
func NewClient(ctx context.Context, cloudName string) (*gophercloud.ServiceClient, error) {
    client, err := clientconfig.NewServiceClient(ctx, "compute", &clientconfig.ClientOpts{
        Cloud: cloudName,
    })
    if err != nil {
        return nil, fmt.Errorf("create openstack compute client: %w", err)
    }
    return client, nil
}
```

One call does authentication and service-catalog lookup: `clientconfig` reads
`clouds.yaml`, authenticates against Keystone with the `auth` block, then returns
a **Nova v2** client (`"compute"`) pointed at the endpoint from the catalog
matching `region_name` and `interface`.

`cloudName` is `OS_CLOUD_NAME` (default `openstack`) and selects the entry under
`clouds:` in `clouds.yaml`.

#### ⚠️ Where `clouds.yaml` is looked up

`clientconfig` searches, in order:

1. the **process's current working directory**
2. `~/.config/openstack`
3. `/etc/openstack`

It **never walks up the directory tree** to find `go.mod`. This is different from
`.env`, which `configs.LoadEnv()` finds via `tools.FindRootDir()` from any
subdirectory. Consequences:

- **Run the backend with its CWD set to `backend/`.**
- In Docker, the image sets `WORKDIR /app` and compose bind-mounts
  `./backend/clouds.yaml → /app/clouds.yaml:ro`. The `WORKDIR` is load-bearing:
  move it and OpenStack breaks.
- A missing `clouds.yaml` produces a server that **starts perfectly and serves
  auth fine**, then fails only on the first `/instances` call — because auth is
  lazy (below).

### `Provider` and lazy authentication (`provider.go`)

```go
type Provider struct {
    cloudName string
    mu        sync.Mutex
    client    *gophercloud.ServiceClient
}

func NewProvider(cloudName string) *Provider { return &Provider{cloudName: cloudName} }

var _ ports.ComputeProvider = (*Provider)(nil)   // compile-time port conformance
```

`NewProvider` does **no I/O**. Every method begins with `ensureClient`:

```go
func (p *Provider) ensureClient(ctx context.Context) (*gophercloud.ServiceClient, error) {
    p.mu.Lock()
    defer p.mu.Unlock()
    if p.client != nil { return p.client, nil }
    client, err := NewClient(ctx, p.cloudName)
    if err != nil { return nil, err }
    p.client = client
    return client, nil
}
```

Two deliberate choices:

- **Lazy, not eager.** An unreachable or misconfigured cloud breaks only the
  `/instances` endpoints. Signup, signin, refresh and `/test` are unaffected,
  because they never touch OpenStack. Authenticating at startup would have made a
  cloud outage take down the whole API.
- **`sync.Mutex`, not `sync.Once`.** `sync.Once` would cache a *failure*
  permanently — the cloud coming back would still leave a dead provider until the
  process restarted. With a mutex, `p.client` stays `nil` on failure and the next
  request retries. The cost is that concurrent first-requests serialize on the
  mutex, and a slow Keystone briefly blocks them all. That trade is accepted: a
  retryable failure is worth more than parallel first-calls.

### `CreateServer`

```go
opts := servers.CreateOpts{ Name: params.Name, ImageRef: params.ImageRef, FlavorRef: params.FlavorRef }
if params.NetworkID != "" {
    opts.Networks = []servers.Network{{UUID: params.NetworkID}}
}
server, err := servers.Create(ctx, client, opts, nil).Extract()
if err != nil { return nil, fmt.Errorf("create server: %w", err) }
return toDomainServer(server), nil
```

- `Networks` is set **only** when `NetworkID` is non-empty — an empty slice would
  be sent as an explicit "no networks" and Nova would boot an unreachable server.
  Omitted, Nova applies its own default.
- The 4th argument to `servers.Create` (scheduler hints) is `nil`.
- `.Extract()` turns the deferred gophercloud result into `*servers.Server`.
- **Returns immediately.** Nova's create is asynchronous: the API accepts the
  request and builds the VM afterwards.

### `GetServer`

```go
server, err := servers.Get(ctx, client, serviceID).Extract()
if err != nil {
    if gophercloud.ResponseCodeIs(err, http.StatusNotFound) { return nil, domain.ErrComputeNotFound }
    return nil, fmt.Errorf("get server: %w", err)
}
return toDomainServer(server), nil
```

`gophercloud.ResponseCodeIs(err, 404)` is the translation point where a
transport-level 404 becomes `domain.ErrComputeNotFound`, which the handler maps
to an HTTP 404. Anything else stays a wrapped error → 502.

### `DeleteServer`

```go
if err := servers.Delete(ctx, client, serviceID).ExtractErr(); err != nil {
    if gophercloud.ResponseCodeIs(err, http.StatusNotFound) { return domain.ErrComputeNotFound }
    return fmt.Errorf("delete server: %w", err)
}
return nil
```

`ExtractErr()` rather than `Extract()` — delete returns no body. Also
asynchronous: success means Nova *accepted* the deletion.

### `toDomainServer`

```go
func toDomainServer(s *servers.Server) *domain.ComputeServer {
    return &domain.ComputeServer{ ID: s.ID, Name: s.Name, Status: s.Status, Addresses: s.Addresses }
}
```

The seam that keeps gophercloud types out of the core: `servers.Server` has many
more fields, and only these four cross the boundary.

---

## 5. The Postgres adapter (`postgres/compute.repository.go`)

`NewComputeRepository(db *sql.DB) ports.ComputeRepository` — returns the
interface, so callers cannot reach the concrete `computeRepository`.

### `Create`

```sql
INSERT INTO compute (id, user_id, compute_service_id, name, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
```
Errors wrapped as `create compute: %w`. `compute_service_id` is `UNIQUE`, and
`user_id` is `REFERENCES users(id) ON DELETE CASCADE` — deleting a user removes
their rows automatically.

### `ListByUserID`

```sql
SELECT id, user_id, compute_service_id, name, status, created_at
FROM compute WHERE user_id = $1 ORDER BY created_at DESC
```
Newest first. Iterates `rows.Next()`, scans into `domain.Compute`, and — easy to
miss — **checks `rows.Err()` after the loop**, so a mid-iteration failure is not
silently truncated into a short list. `defer rows.Close()`.

Returns a `nil` slice when the user has none; the DTO layer converts that to `[]`.

### `GetByServiceIDAndUserID`

```sql
SELECT id, user_id, compute_service_id, name, status, created_at
FROM compute WHERE compute_service_id = $1 AND user_id = $2
```
`sql.ErrNoRows` → `domain.ErrComputeNotFound`.

**Both predicates in one query is the ownership check.** A row comes back only if
the instance exists *and* belongs to the caller. Another user's valid instance ID
is indistinguishable from a nonexistent one — same error, same 404, so the API
never confirms that someone else's instance exists.

### `DeleteByServiceIDAndUserID`

```sql
DELETE FROM compute WHERE compute_service_id = $1 AND user_id = $2
```
Then:

```go
n, err := res.RowsAffected()
if n == 0 { return domain.ErrComputeNotFound }
```

The `RowsAffected() == 0` check is what makes the delete honest — a `DELETE` that
matches nothing is not an error in SQL, so without this the API would report
success for an instance the caller never owned.

---

## 6. Flow: create an instance

`POST /instances` → `ComputeHandler.Create` → **`ComputeUsecase.Create`**

```go
func (u *ComputeUsecase) Create(ctx context.Context, userID uuid.UUID, params domain.ComputeCreateParams) (*domain.Compute, error) {
    server, err := u.provider.CreateServer(ctx, params)      // 1. Nova first
    if err != nil { return nil, err }

    record := &domain.Compute{                                // 2. seed the row
        ID:               uuid.New(),
        UserID:           userID,
        ComputeServiceID: server.ID,
        Name:             params.Name,                        // from the REQUEST
        Status:           domain.ComputeStatusBuilding,        // constant "BUILD"
        CreatedAt:        time.Now().UTC(),
    }
    if err := u.computes.Create(ctx, record); err != nil {     // 3. persist
        return nil, fmt.Errorf("store compute record: %w", err)
    }
    return record, nil
}
```

**Order: Nova first, then the DB.** Nova assigns the server ID, and
`compute_service_id` cannot be written before it exists.

### Why `Name` and `Status` do not come from Nova's response

This was verified against a live devstack: Nova's **create** response carries the
new server's **ID and essentially nothing else** — `Name` and `Status` came back
empty. A follow-up `Get` on the same server returned the real values
(`gopricloud-smoketest` / `ACTIVE`).

So the row is seeded from what is actually known: `params.Name` from the request,
and the constant `domain.ComputeStatusBuilding` (`"BUILD"`) for status. Copying
Nova's create response would have written empty strings.

### Consequences worth knowing

- **`compute.status` is written once and never updated.** Nothing in the codebase
  issues an `UPDATE` on it. A row therefore reads `"BUILD"` forever, even when
  the VM is long since `ACTIVE`. `GET /instances` reflects the DB, so it shows
  `BUILD`; live status only comes from `GET /instances/{id}`, which asks Nova.
  Deliberate — there is no background reconciler — but it means the list view's
  status field is an initial value, not a current one.
- **A DB failure after a successful Nova create leaks a VM.** Step 1 succeeded, so
  the server is booting; step 3 failed, so no row records it. The caller gets a
  502 and the instance is orphaned — running, billable, and invisible to the API
  because listing is DB-driven. No compensating delete is attempted.

> 📸 *Screenshot: create-instance dialog in the dashboard*
>
> 📸 *Screenshot: `POST /instances` 201 response with `status: "BUILD"`*

---

## 7. Flow: list instances

`GET /instances` → `ComputeHandler.List` → **`ComputeUsecase.List`**

```go
func (u *ComputeUsecase) List(ctx context.Context, userID uuid.UUID) ([]domain.Compute, error) {
    return u.computes.ListByUserID(ctx, userID)
}
```

A straight pass-through: **no Nova call at all.** One indexed query
(`idx_compute_user_id`), so the dashboard loads at DB speed and works even while
the cloud is unreachable.

The trade-off is the one above: `status` is the stored `"BUILD"`, not live state.
`Name` is whatever was requested at create time and will not reflect a rename
made directly in OpenStack.

This is also what makes instances survive sign-out: the rows are keyed by
`user_id`, so signing back in and calling `GET /instances` repopulates the
dashboard.

> 📸 *Screenshot: Instances section listing the user's instances*

---

## 8. Flow: get one instance

`GET /instances/{id}` → `ComputeHandler.Get` (`r.PathValue("id")`) →
**`ComputeUsecase.Get`**

```go
func (u *ComputeUsecase) Get(ctx context.Context, userID uuid.UUID, serviceID string) (*domain.ComputeServer, error) {
    if _, err := u.computes.GetByServiceIDAndUserID(ctx, serviceID, userID); err != nil {
        return nil, err                      // 404 for "not yours" AND "not found"
    }
    return u.provider.GetServer(ctx, serviceID)   // live state from Nova
}
```

**Ownership is checked before Nova is contacted.** The returned `*domain.Compute`
is discarded (`_`) — it is fetched purely as an authorization probe. This is the
one place where DB and Nova are combined: the DB authorizes, Nova answers.

Both failure modes converge on `domain.ErrComputeNotFound` → **404**:

| Situation | Where it is caught |
|---|---|
| No such instance anywhere | repository, `sql.ErrNoRows` |
| Exists, owned by a **different** user | repository, `AND user_id = $2` excludes it |
| Row exists but the VM is gone from Nova | provider, `ResponseCodeIs(err, 404)` |

An unreachable cloud gives a wrapped error → **502**, not 404 — "I cannot reach
the cloud" stays distinguishable from "that instance does not exist".

> 📸 *Screenshot: instance detail dialog showing the live Nova JSON*

---

## 9. Flow: delete an instance

`DELETE /instances/{id}` → `ComputeHandler.Delete` → **`ComputeUsecase.Delete`**

```go
func (u *ComputeUsecase) Delete(ctx context.Context, userID uuid.UUID, serviceID string) error {
    if _, err := u.computes.GetByServiceIDAndUserID(ctx, serviceID, userID); err != nil {
        return err                                              // 1. authorize
    }
    if err := u.provider.DeleteServer(ctx, serviceID); err != nil {
        return err                                              // 2. destroy in Nova
    }
    return u.computes.DeleteByServiceIDAndUserID(ctx, serviceID, userID)  // 3. drop the row
}
```

### The ordering is the whole design

**ownership check → delete in Nova → delete the row.** Each step only runs if the
previous one succeeded, and the failure mode of each is deliberately the safe one:

- **Check first** — never issue a Nova delete for an instance the caller does not
  own. Without this, any authenticated user could destroy anyone's VM by ID.
- **Nova before Postgres** — if the Nova delete fails, the row survives, so the
  instance stays visible and the user can retry. The reverse order would delete
  the row first: the VM keeps running and is now **unreachable through the API**,
  because listing is DB-driven. A silent orphan.
- The remaining gap is narrow and accepted: if Nova succeeds and step 3 fails,
  the row points at a destroyed VM. The user sees a 502, and a retry returns 404
  from the provider (the VM is already gone) — so the stale row is not
  self-healing, but it *is* visible and harmless.

Success → **204 No Content**.

> 📸 *Screenshot: delete-instance confirmation dialog*
>
> 📸 *Screenshot: `DELETE /instances/{id}` 204 and the instance gone from the list*

---

## 10. Failure-mode summary

| Failure | Where | Result | State afterwards |
|---|---|---|---|
| `clouds.yaml` missing / wrong CWD | `ensureClient` → `NewClient` | 502 on `/instances` only | Auth endpoints keep working; retried on every request |
| Cloud unreachable | `ensureClient` | 502 | `p.client` stays `nil`, so recovery is automatic |
| Bad image/flavor ref | `servers.Create` | 502 `could not provision instance: ...` | Nothing created, no row |
| Nova create OK, DB insert fails | `computes.Create` | 502 | **VM orphaned** — running, no row, invisible to the API |
| Instance belongs to another user | `GetByServiceIDAndUserID` | 404 | Unchanged; existence not disclosed |
| VM deleted outside the API | `servers.Get`/`Delete` 404 | 404 | Row remains, now stale |
| Nova delete OK, row delete fails | `DeleteByServiceIDAndUserID` | 502 | Row points at a destroyed VM |
| DB unreachable at startup | `postgres.Open` → `PingContext` | Process exits | Fails fast, unlike OpenStack |

---

## 11. Quick reference — call chains

```
POST /instances
  ComputeHandler.Create
    userIDFromRequest                      -> uuid from JWT claims in ctx
    ComputeUsecase.Create
      Provider.CreateServer                -> ensureClient -> servers.Create(...).Extract()
      computeRepository.Create             -> INSERT INTO compute (...)
    dto.NewComputeResponse                 -> 201

GET /instances
  ComputeHandler.List
    ComputeUsecase.List
      computeRepository.ListByUserID       -> SELECT ... WHERE user_id = $1 ORDER BY created_at DESC
    dto.NewComputeListResponse             -> 200 (always an array)

GET /instances/{id}
  ComputeHandler.Get                       -> r.PathValue("id")
    ComputeUsecase.Get
      computeRepository.GetByServiceIDAndUserID   -> SELECT ... WHERE compute_service_id=$1 AND user_id=$2
      Provider.GetServer                   -> ensureClient -> servers.Get(...).Extract()
    dto.NewComputeServerResponse           -> 200

DELETE /instances/{id}
  ComputeHandler.Delete
    ComputeUsecase.Delete
      computeRepository.GetByServiceIDAndUserID   -> ownership probe
      Provider.DeleteServer                -> ensureClient -> servers.Delete(...).ExtractErr()
      computeRepository.DeleteByServiceIDAndUserID -> DELETE ...; RowsAffected()==0 -> 404
                                           -> 204 No Content
```

---

## 12. Trying it end to end

`image_ref` and `flavor_ref` must be real IDs from your cloud:

```bash
# On the OpenStack host
openstack image list
openstack flavor list
openstack network list      # optional network_id
```

Then, with `$TOKEN` from `POST /signin` (field `access_token`):

```bash
# Create -> 201, status "BUILD"
curl -s -X POST http://localhost:5173/instances \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"gopricloud-smoketest","image_ref":"<IMAGE_ID>","flavor_ref":"<FLAVOR_ID>"}'

# List -> 200, [] when empty
curl -s http://localhost:5173/instances -H "Authorization: Bearer $TOKEN"

# Live state -> 200; status becomes ACTIVE once Nova finishes building
curl -s http://localhost:5173/instances/<COMPUTE_SERVICE_ID> -H "Authorization: Bearer $TOKEN"

# Destroy -> 204
curl -i -X DELETE http://localhost:5173/instances/<COMPUTE_SERVICE_ID> -H "Authorization: Bearer $TOKEN"
```

Two behaviours to expect, both correct:

- Right after create, `GET /instances/{id}` reports `BUILD`, then `ACTIVE` a few
  seconds later — Nova's create is asynchronous. The **list** endpoint keeps
  saying `BUILD` regardless, because it reads the never-updated DB column.
- Calling `GET` or `DELETE` with another user's instance ID returns **404**, not
  403 — verified with two accounts.
