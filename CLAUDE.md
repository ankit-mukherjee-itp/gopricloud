# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

GoPriCloud: users sign up / sign in, then from a dashboard provision,
inspect, and destroy OpenStack Nova compute instances under their own
account. Two independent projects in one repo:

- **Backend** (`backend/`) — a Go REST API (clean/ports-and-adapters
  architecture), backed by Postgres for its own state and by an OpenStack
  cloud (via [gophercloud](https://github.com/gophercloud/gophercloud))
  for the actual compute instances.
- **Frontend** (`frontend/`) — React (Vite, **TypeScript**) + Tailwind CSS
  + shadcn/ui, calling the backend over HTTP.

Detailed, function-by-function write-ups already exist — read these
instead of re-deriving the call chain from scratch:

- `Docs/API_Calls.md` — every HTTP endpoint traced through router →
  middleware → handler → usecase → repository/provider.
- `Docs/Provisioning.md` — the create/list/get/delete-instance flows in
  full, down to each OpenStack (gophercloud) and Postgres call.
- `README.md` — user-facing overview and run instructions.

> ⚠️ **Both `Docs/*.md` files and `README.md` still describe the
> pre-refactor layout** (a `gopricloud/gopricloud/` source tree and
> doubled `gopricloud/gopricloud/...` import paths). That layout no
> longer exists — see below. Their *descriptions of behaviour* (call
> chains, auth flow, provisioning order) are still accurate, because the
> refactor changed only structure; their *paths and import strings* are
> not. Trust this file over them for layout.

## Repo layout

The backend is a **self-contained Go module rooted at `backend/`** — that
is where `go.mod` lives, and the module is named `backend`:

```
gopricloud/                      <- repo root (no go.mod here)
├── CLAUDE.md / README.md / orchestrator.md
├── Docs/
├── frontend/                    <- separate npm project (TypeScript)
└── backend/                     <- Go module root; module `backend`
    ├── go.mod / go.sum
    ├── .env / clouds.yaml       <- runtime config
    ├── .go-arch-lint.yml / Makefile
    ├── cmd/main.go              <- composition root
    ├── cmd/server/server.go
    ├── configs/configs.go
    ├── internal/core/...        <- domain, ports, services, token
    ├── internal/adapters/...    <- handlers, repositories, providers, api
    └── tools/tools.go
```

Import paths are therefore `backend/internal/core/...`,
`backend/internal/adapters/...`, `backend/configs`, `backend/tools`.

## ⚠️ Config-discovery gotcha (read this first)

`.env` and `clouds.yaml` both live in `backend/`, but they are found by
**two different mechanisms**, and only one of them walks up the tree:

- **`.env` resolves from any subdirectory.** `configs.LoadEnv()` calls
  `tools.FindRootDir()`, which walks up until it finds the directory
  holding `go.mod`, then loads the `.env` beside it. So `go run` from
  `backend/internal/core/...` still picks up the environment.
- **`clouds.yaml` does NOT.** It is read by gophercloud's
  `clientconfig`, which searches the **process's current working
  directory**, then `~/.config/openstack`, then `/etc/openstack`. It
  never walks up to `go.mod`.

**Consequence: always run the backend with its CWD set to `backend/`.**
If you don't, the API still starts and auth endpoints work fine, but
every `/instances` call fails — because OpenStack auth is lazy (see
"Compute provisioning design"), the missing `clouds.yaml` only surfaces on
first use, not at startup. A silent, delayed failure mode.

Note also that `LoadEnv()` deliberately does **not** fail on a missing
`.env` (a deployed binary has no source tree; real environment variables
take precedence). `Load()` still rejects a missing `DATABASE_URL` /
`JWT_SECRET`, so nothing starts silently misconfigured.

## Commands

**Backend** — all from `backend/`, so `clouds.yaml` resolves (see the
gotcha above):

```
cd backend
go run ./cmd      # note: ./cmd, not ./cmd/api
go build ./...
go vet ./...
gofmt -l .        # list unformatted files
gofmt -w .        # fix them
go mod tidy
```

> **`make` is not installed on this machine** (checked in both Git Bash
> and PowerShell: no `make`, `mingw32-make`, `gmake`, or `nmake`). A
> `backend/Makefile` exists and wraps the commands above as `run`,
> `build`, `check`, `fmt`, `vet`, `tidy`, `test`, `arch`, `arch-check` —
> but it is currently **unusable here** and none of its targets have been
> run. Use the direct `go` commands above, or install make first.

Architecture-boundary checking needs a separate tool install
(`go install github.com/fe3dback/go-arch-lint@latest`), then
`go-arch-lint check` from `backend/`.

Required env vars (see `backend/.env`): `DATABASE_URL` (Postgres DSN),
`JWT_SECRET`. Optional: `PORT` (default 8080), `JWT_ISSUER`,
`OS_CLOUD_NAME` (cloud entry name in `clouds.yaml`, default `openstack`).

There is no automated test suite yet (no `_test.go` files) — verification
so far has been manual (curl / browser) against a live Postgres and
OpenStack instance. `go test ./...` runs clean because it matches nothing.

**Frontend** (run from `frontend/`):

```
npm install
npm run dev        # Vite dev server on :5173, proxies API calls to :8080
npm run build      # production build
npm run typecheck  # tsc --noEmit
npm run lint       # oxlint
npm run preview    # serve the production build locally
```

The dev server proxies `/signup`, `/signin`, `/refresh`, `/test`, and
`/instances` to `http://localhost:8080` (see `frontend/vite.config.ts`),
so the backend must be running on port 8080 for the frontend to work in
dev, and no CORS handling is needed (or present) on the backend. Note
`/signup` is proxied with a method-aware `bypass`: it's both a client-side
route (the signup page) and a backend POST endpoint, so a plain GET
navigation must fall through to the SPA instead of hitting the backend.
**This bypass is load-bearing** — remove it and loading the signup page
returns 405 from the POST-only backend endpoint.

## Backend architecture

Ports-and-adapters / clean architecture, strictly layered by dependency
direction — `domain` has no dependencies; everything else depends inward.
`backend/.go-arch-lint.yml` encodes this mechanically (`go-arch-lint check`):

```
internal/core/domain       Entities: User, RefreshToken, Compute,
                            ComputeServer, ComputeCreateParams, domain errors.
                            No dependencies on anything else in the repo.

internal/core/ports        Ports (interfaces) only, one file each:
                              user-repository.interface.go     (Postgres)
                              token-repository.interface.go    (Postgres)
                              compute-repository.interface.go  (Postgres)
                              compute-provider.interface.go    (OpenStack)

internal/core/services     Business logic, depends only on the ports above:
                              auth.service.go     — AuthUsecase    (signup/signin/refresh)
                              compute.service.go  — ComputeUsecase (create/list/get/delete)

internal/core/token        JWT signing/verification (JWTManager) and opaque
                            refresh-token generation/hashing.

internal/adapters/repositories/postgres
                            Implements the three repository ports. Also owns
                            schema bootstrap (creates tables on startup — no
                            migration tool).

internal/adapters/providers/openstack
                            Implements ComputeProvider on top of gophercloud/Nova.

internal/adapters/handlers/rest
                            Inbound adapter: routes.go (stdlib http.ServeMux,
                            Go 1.22+ method+path patterns), handlers, plus
                            dto/ and middleware/ subpackages.

internal/adapters/api      Shared HTTP response/error helpers.

configs/                   Env loading (LoadEnv) + Config struct (Load).
tools/                     FindRootDir — stdlib-only leaf, no deps.

cmd/main.go                The only place concrete types get wired together.
cmd/server/server.go       http.Server setup, listen, graceful shutdown.
```

Two naming notes that trip people up:

- **The service types kept their original names.** They live in
  `internal/core/services/` but are still called `AuthUsecase` and
  `ComputeUsecase`, constructed by `NewAuthUsecase` / `NewComputeUsecase`.
  The refactor moved code without renaming symbols.
- **`internal/core/token` is a core package, not an adapter**, even though
  it wraps a third-party JWT library.

### About `.go-arch-lint.yml` — what a green run does and does not mean

The reference architecture this was modelled on grants the core no vendor
access at all, making a pure core a protected invariant. **That invariant
does not hold here**, and the config does not pretend otherwise: 11 vendor
import lines sit inside core packages (`google/uuid` in domain/ports/
services, `bcrypt` in services, `golang-jwt` + `uuid` in token). Those
components carry explicit `anyVendorDeps: true` grants.

So a green `go-arch-lint check` means **dependency *direction* is intact** —
nothing under `internal/core/**` imports an adapter, a handler, or `cmd`.
It does *not* mean the core is dependency-free. Fix violations by moving
code to the correct layer, never by widening a rule in that file.

`deepScan` is deliberately `false`: it traces DI data-flow and
misattributes the composition root's wiring (cmd builds a concrete adapter
and injects it through a port) as a phantom adapter→service edge.

### Auth design

- **Access token**: JWT (HS256), 5 min TTL, self-contained (user ID +
  email as claims), never persisted server-side.
- **Refresh token**: random 32-byte opaque string, 7 day TTL. Only its
  SHA-256 hash is stored; the raw value is returned to the client once.
  **Rotated on every use** — using a refresh token revokes it and issues
  a new one. If an already-revoked refresh token is presented again
  (replay), every outstanding refresh token for that user is revoked,
  since reuse is treated as a compromise signal.

### Compute provisioning design

`ComputeUsecase` is the only thing that talks to both `ComputeRepository`
(the `compute` Postgres table: `id`, `user_id`, `compute_service_id`,
`name`, `status`, `created_at`) and `ComputeProvider` (actual Nova calls),
and always keeps them in sync — every mutating operation touches both.

- **Ownership is enforced at the repository query**, not in application
  logic: `GetByServiceIDAndUserID` / `DeleteByServiceIDAndUserID` filter
  by `compute_service_id AND user_id` together, so a row only comes back
  (or gets deleted) if both match. `Get` and `Delete` in `ComputeUsecase`
  always check ownership this way *before* touching OpenStack.
- **Delete order matters**: ownership check → delete in Nova → delete the
  Postgres row, in that order, so a failure partway through never leaves
  a "deleted" DB row pointing at a VM that's still running.
- **Create doesn't trust Nova's response for name/status**: Nova's create
  response only contains the new server's ID, so the `compute` row is
  seeded from the request params and a constant initial status
  (`domain.ComputeStatusBuilding`, `"BUILD"`) rather than fields Nova
  doesn't actually return. Live status comes from a later `Get`.
- **OpenStack auth is lazy**: the `Provider` in
  `internal/adapters/providers/openstack` doesn't authenticate at startup
  — it authenticates on first use and caches the client (mutex-guarded,
  **not** `sync.Once`, so a failed attempt isn't cached forever). This
  means an unreachable/misconfigured OpenStack cloud only breaks
  `/instances` endpoints; auth endpoints are unaffected. It is also why a
  missing `clouds.yaml` surfaces late rather than at startup.

See `Docs/Provisioning.md` for the exact code of every function involved.

## Frontend architecture

React + **TypeScript** + Vite, Tailwind CSS, shadcn/ui components
(`frontend/src/components/ui/`), React Router for `/login`, `/signup`,
`/dashboard`. TS config is split: `tsconfig.json` (app) +
`tsconfig.node.json` (Vite config); `npm run typecheck` covers the app.

- `src/lib/types.ts` — shared request/response and domain types used
  across the API layer and components.
- `src/lib/tokens.ts` — localStorage read/write for access token, refresh
  token, and the signed-in user object.
- `src/lib/api.ts` — the only place `fetch` is called. Wraps every request
  with the stored access token; on a 401 it transparently calls `/refresh`
  once (de-duped across concurrent 401s via a shared in-flight promise)
  and retries the original request, or clears the session and dispatches
  a `window` `auth:unauthorized` event if refresh itself fails.
- `src/context/AuthContext.tsx` — listens for that `auth:unauthorized`
  event to log the user out from anywhere, independent of `api.ts` having
  any React awareness.
- `src/components/ProtectedRoute.tsx` — `ProtectedRoute` /
  `PublicOnlyRoute` gate `/dashboard` vs `/login`+`/signup` based on
  `AuthContext`.

## Working in this repo

- **OneDrive syncs this directory, and it will fight you.** It has
  silently created conflict copies (`client-ANKIT-MUKHERJEE.go`) *and*
  reverted the originals mid-session, producing a Go build error about two
  packages in one directory after a build that had just passed. After any
  bulk file operation, check with
  `find . -not -path "./.git/*" -not -path "*/node_modules/*" -name "*-ANKIT-MUKHERJEE*"`
  before trusting a green build.
- **Paths here contain spaces and a comma** ("Intuitive Technology
  Partners, Inc"). Never split command output on whitespace when
  processing file lists — split on newlines, and quote every path.
- `jq` is **not** installed; Node is. Scripts needing JSON should use
  `node`.
