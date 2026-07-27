# Provisioning — Implementation Walkthrough

This document covers everything involved in creating, listing, inspecting,
and deleting compute instances: the `ComputeUsecase` orchestration layer,
the `ComputeProvider` port and its OpenStack/gophercloud implementation
(`openstack/compute/provider.go`), and the `ComputeRepository` port and its
Postgres implementation (`internal/infrastructure/postgres/compute_repo.go`).
For how the HTTP request reaches `ComputeUsecase` in the first place
(routing, middleware, request decoding), see
[`API_Calls.md`](API_Calls.md).

Code references are relative to `gopricloud/gopricloud/`.

## The two ports `ComputeUsecase` depends on

```go
// internal/repository/compute_repository.go
type ComputeRepository interface {
    Create(ctx context.Context, c *domain.Compute) error
    ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Compute, error)
    GetByServiceIDAndUserID(ctx context.Context, serviceID string, userID uuid.UUID) (*domain.Compute, error)
    DeleteByServiceIDAndUserID(ctx context.Context, serviceID string, userID uuid.UUID) error
}
```

```go
// internal/repository/compute_provider.go
type ComputeProvider interface {
    CreateServer(ctx context.Context, params domain.ComputeCreateParams) (*domain.ComputeServer, error)
    GetServer(ctx context.Context, serviceID string) (*domain.ComputeServer, error)
    DeleteServer(ctx context.Context, serviceID string) error
}
```

`ComputeRepository` is GoPriCloud's own bookkeeping (the `compute` Postgres
table: which Nova server belongs to which user). `ComputeProvider` is the
actual OpenStack Nova calls. `ComputeUsecase` is the only thing that talks
to both, and it always keeps them in sync — a Nova call and a Postgres
call happen together for every mutating operation, never one without the
other.

`domain.Compute` (the DB-tracked record) and `domain.ComputeServer` (Nova's
live view of a server) are different types on purpose:

```go
// internal/domain/compute.go
type Compute struct {
    ID               uuid.UUID
    UserID           uuid.UUID
    ComputeServiceID string // Nova server ID
    Name             string
    Status           string
    CreatedAt        time.Time
}

type ComputeCreateParams struct {
    Name      string
    ImageRef  string
    FlavorRef string
    NetworkID string // optional
}

type ComputeServer struct {
    ID        string
    Name      string
    Status    string
    Addresses map[string]any
}
```

---

## Creating an instance

**`ComputeUsecase.Create`** (`internal/usecase/compute_usecase.go`)

```go
func (u *ComputeUsecase) Create(ctx context.Context, userID uuid.UUID, params domain.ComputeCreateParams) (*domain.Compute, error) {
    server, err := u.provider.CreateServer(ctx, params)
    ...
    record := &domain.Compute{
        ID:               uuid.New(),
        UserID:           userID,
        ComputeServiceID: server.ID,
        Name:             params.Name,
        Status:           domain.ComputeStatusBuilding,
        CreatedAt:        time.Now().UTC(),
    }
    u.computes.Create(ctx, record)
    return record, nil
}
```

1. `u.provider.CreateServer(ctx, params)` — calls into OpenStack first (see
   below). If this fails, nothing is written to Postgres.
2. Builds the `domain.Compute` row using **`params.Name`** and the constant
   **`domain.ComputeStatusBuilding` ("BUILD")** rather than `server.Name` /
   `server.Status`. This is deliberate: Nova's create response only
   contains the new server's ID (plus a few housekeeping fields) — not its
   name or status — so those are seeded from what the caller asked for and
   the known initial state instead. Live status is fetched later via `Get`.
3. `u.computes.Create(ctx, record)` — persists the row.

**`Provider.CreateServer`** (`openstack/compute/provider.go`)

```go
func (p *Provider) CreateServer(ctx context.Context, params domain.ComputeCreateParams) (*domain.ComputeServer, error) {
    client, err := p.ensureClient(ctx)
    ...
    opts := servers.CreateOpts{
        Name:      params.Name,
        ImageRef:  params.ImageRef,
        FlavorRef: params.FlavorRef,
    }
    if params.NetworkID != "" {
        opts.Networks = []servers.Network{{UUID: params.NetworkID}}
    }
    server, err := servers.Create(ctx, client, opts, nil).Extract()
    ...
    return toDomainServer(server), nil
}
```

1. `p.ensureClient(ctx)` — see **Lazy authentication** below.
2. Builds a gophercloud `servers.CreateOpts{Name, ImageRef, FlavorRef}`.
   `NetworkID` is only attached (`opts.Networks = []servers.Network{{UUID: ...}}`)
   if the caller supplied one — otherwise Nova falls back to its own
   default network behavior.
3. `servers.Create(ctx, client, opts, nil)` — the actual Nova API call
   (`POST /servers`); the trailing `nil` is scheduler hints, unused here.
   `.Extract()` decodes the response into a `*servers.Server`.
4. `toDomainServer(server)` maps the gophercloud type onto
   `domain.ComputeServer`.

**Postgres — `computeRepository.Create`** (`internal/infrastructure/postgres/compute_repo.go`)

```go
func (r *computeRepository) Create(ctx context.Context, c *domain.Compute) error {
    const q = `
        INSERT INTO compute (id, user_id, compute_service_id, name, status, created_at)
        VALUES ($1, $2, $3, $4, $5, $6)
    `
    _, err := r.db.ExecContext(ctx, q, c.ID, c.UserID, c.ComputeServiceID, c.Name, c.Status, c.CreatedAt)
    ...
}
```

A single `INSERT` with the six columns of the `compute` table.

> 📸 *Screenshot: creating an instance end-to-end (API call + resulting row/instance)*
>
> _(space reserved — paste screenshot here)_

---

## Listing instances

**`ComputeUsecase.List`**

```go
func (u *ComputeUsecase) List(ctx context.Context, userID uuid.UUID) ([]domain.Compute, error) {
    return u.computes.ListByUserID(ctx, userID)
}
```

A pure pass-through to the repository — no OpenStack call. Listing is
answered entirely from GoPriCloud's own records, which is what makes a
user's dashboard populate instantly (and why the `compute` table needs to
exist at all, rather than always asking Nova directly).

**Postgres — `computeRepository.ListByUserID`**

```go
func (r *computeRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]domain.Compute, error) {
    const q = `
        SELECT id, user_id, compute_service_id, name, status, created_at
        FROM compute WHERE user_id = $1 ORDER BY created_at DESC
    `
    rows, err := r.db.QueryContext(ctx, q, userID)
    ...
    for rows.Next() {
        var c domain.Compute
        rows.Scan(&c.ID, &c.UserID, &c.ComputeServiceID, &c.Name, &c.Status, &c.CreatedAt)
        out = append(out, c)
    }
    ...
}
```

`SELECT ... WHERE user_id = $1`, newest first, scanned row by row into
`domain.Compute` values.

> 📸 *Screenshot: listing a user's instances*
>
> _(space reserved — paste screenshot here)_

---

## Getting instance detail

**`ComputeUsecase.Get`**

```go
func (u *ComputeUsecase) Get(ctx context.Context, userID uuid.UUID, serviceID string) (*domain.ComputeServer, error) {
    if _, err := u.computes.GetByServiceIDAndUserID(ctx, serviceID, userID); err != nil {
        return nil, err
    }
    return u.provider.GetServer(ctx, serviceID)
}
```

1. **Ownership check first:** `u.computes.GetByServiceIDAndUserID(ctx,
   serviceID, userID)` — if this `serviceID` isn't in the `compute` table
   under this `userID`, the call fails with `domain.ErrComputeNotFound`
   *before ever contacting OpenStack*. This is what stops one user from
   probing another user's instances by guessing/reusing a server ID.
2. Only if that succeeds: `u.provider.GetServer(ctx, serviceID)` fetches
   the live state from Nova.

**Postgres — `computeRepository.GetByServiceIDAndUserID`**

```go
func (r *computeRepository) GetByServiceIDAndUserID(ctx context.Context, serviceID string, userID uuid.UUID) (*domain.Compute, error) {
    const q = `
        SELECT id, user_id, compute_service_id, name, status, created_at
        FROM compute WHERE compute_service_id = $1 AND user_id = $2
    `
    var c domain.Compute
    err := r.db.QueryRowContext(ctx, q, serviceID, userID).Scan(...)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, domain.ErrComputeNotFound
    }
    ...
}
```

The `WHERE compute_service_id = $1 AND user_id = $2` clause **is** the
authorization check — a row only comes back if both match. `sql.ErrNoRows`
(no match) is translated to `domain.ErrComputeNotFound`, which the HTTP
layer maps to a 404 (see `API_Calls.md`).

**`Provider.GetServer`**

```go
func (p *Provider) GetServer(ctx context.Context, serviceID string) (*domain.ComputeServer, error) {
    client, err := p.ensureClient(ctx)
    ...
    server, err := servers.Get(ctx, client, serviceID).Extract()
    if err != nil {
        if gophercloud.ResponseCodeIs(err, http.StatusNotFound) {
            return nil, domain.ErrComputeNotFound
        }
        return nil, fmt.Errorf("get server: %w", err)
    }
    return toDomainServer(server), nil
}
```

1. `p.ensureClient(ctx)` — lazy auth (below).
2. `servers.Get(ctx, client, serviceID)` — Nova `GET /servers/{id}`,
   `.Extract()` decodes it.
3. If Nova itself returns 404 (e.g. the server was deleted directly in
   OpenStack, outside GoPriCloud), `gophercloud.ResponseCodeIs(err,
   http.StatusNotFound)` catches it and it's mapped to the same
   `domain.ErrComputeNotFound` as the ownership-check case — the caller
   can't tell the difference, which is fine since both mean "no such
   instance for you."
4. `toDomainServer(server)` — same mapping helper used by `CreateServer`,
   this time carrying Nova's live `Name`, `Status`, and `Addresses`
   (network name → list of `{addr, version, ...}` entries).

> 📸 *Screenshot: fetching a single instance's detail (live Nova state)*
>
> _(space reserved — paste screenshot here)_

---

## Deleting an instance

**`ComputeUsecase.Delete`**

```go
func (u *ComputeUsecase) Delete(ctx context.Context, userID uuid.UUID, serviceID string) error {
    if _, err := u.computes.GetByServiceIDAndUserID(ctx, serviceID, userID); err != nil {
        return err
    }
    if err := u.provider.DeleteServer(ctx, serviceID); err != nil {
        return err
    }
    return u.computes.DeleteByServiceIDAndUserID(ctx, serviceID, userID)
}
```

Three steps, in order, each gating the next:

1. **Ownership check** — same `GetByServiceIDAndUserID` call as `Get`.
   Nothing is deleted anywhere if this fails.
2. **`u.provider.DeleteServer(ctx, serviceID)`** — the real Nova server is
   destroyed first.
3. **`u.computes.DeleteByServiceIDAndUserID(ctx, serviceID, userID)`** —
   only once OpenStack has confirmed the delete does the tracking row get
   removed. This ordering means a failure between steps 2 and 3 leaves a
   `compute` row with no backing server rather than the reverse (a
   "deleted" row while the real VM is still running and unaccounted for).

**`Provider.DeleteServer`**

```go
func (p *Provider) DeleteServer(ctx context.Context, serviceID string) error {
    client, err := p.ensureClient(ctx)
    ...
    if err := servers.Delete(ctx, client, serviceID).ExtractErr(); err != nil {
        if gophercloud.ResponseCodeIs(err, http.StatusNotFound) {
            return domain.ErrComputeNotFound
        }
        return fmt.Errorf("delete server: %w", err)
    }
    return nil
}
```

`servers.Delete(ctx, client, serviceID)` — Nova `DELETE /servers/{id}`;
`.ExtractErr()` just checks for an error (there's no response body to
decode). A 404 from Nova is mapped to `domain.ErrComputeNotFound` the same
way as `GetServer`.

**Postgres — `computeRepository.DeleteByServiceIDAndUserID`**

```go
func (r *computeRepository) DeleteByServiceIDAndUserID(ctx context.Context, serviceID string, userID uuid.UUID) error {
    const q = `DELETE FROM compute WHERE compute_service_id = $1 AND user_id = $2`
    res, err := r.db.ExecContext(ctx, q, serviceID, userID)
    ...
    n, err := res.RowsAffected()
    ...
    if n == 0 {
        return domain.ErrComputeNotFound
    }
    return nil
}
```

`DELETE ... WHERE compute_service_id = $1 AND user_id = $2` — again, the
`user_id` condition is the authorization boundary. `RowsAffected()` is
checked explicitly: if it's `0` (row already gone, or never belonged to
this user), that's surfaced as `domain.ErrComputeNotFound` rather than
silently succeeding.

> 📸 *Screenshot: deleting an instance (before/after)*
>
> _(space reserved — paste screenshot here)_

---

## Lazy authentication — `Provider.ensureClient`

Every `Provider` method (`CreateServer`, `GetServer`, `DeleteServer`) calls
this first:

```go
func (p *Provider) ensureClient(ctx context.Context) (*gophercloud.ServiceClient, error) {
    p.mu.Lock()
    defer p.mu.Unlock()

    if p.client != nil {
        return p.client, nil
    }

    client, err := NewClient(ctx, p.cloudName)
    if err != nil {
        return nil, err
    }
    p.client = client
    return client, nil
}
```

The `Provider` doesn't authenticate to OpenStack at startup — it holds
just a `cloudName` until the first compute call, then authenticates once
and caches the resulting `*gophercloud.ServiceClient` behind a mutex (a
plain `sync.Mutex`, not `sync.Once`, specifically so a *failed* attempt
isn't cached forever — the next call will just retry `NewClient`). This
means an unreachable or misconfigured OpenStack cloud only breaks the
`/instances` endpoints; signup/signin/refresh/test, which don't touch
`Provider` at all, keep working.

`NewClient` itself (`openstack/compute/client.go`):

```go
func NewClient(ctx context.Context, cloudName string) (*gophercloud.ServiceClient, error) {
    client, err := clientconfig.NewServiceClient(ctx, "compute", &clientconfig.ClientOpts{
        Cloud: cloudName,
    })
    ...
    return client, nil
}
```

Delegates to `gophercloud/utils/v2/openstack/clientconfig`, which reads
the named cloud entry (`cloudName`, from `config.OSCloudName`, i.e.
`OS_CLOUD_NAME` env var, default `"openstack"`) out of `clouds.yaml` —
searched for in the process's working directory first — authenticates
against Keystone, and returns a Nova v2 service client.
