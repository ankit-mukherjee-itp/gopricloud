# GoPriCloud

GoPriCloud is a small private-cloud front end: users sign up, sign in, and
from a dashboard provision, inspect, and destroy OpenStack Nova compute
instances under their own account. It has two parts:

- **Backend** (`gopricloud/`) — a Go REST API built with clean architecture,
  backed by Postgres for its own state (users, tokens, instance records) and
  by an OpenStack cloud (via [gophercloud](https://github.com/gophercloud/gophercloud))
  for the actual compute instances.
- **Frontend** (`frontend/`) — a React (Vite, JavaScript) dashboard styled
  with Tailwind CSS and shadcn/ui, talking to the backend over HTTP.

Detailed, function-by-function write-ups live in [`Docs/`](Docs):

- [`Docs/API_Calls.md`](Docs/API_Calls.md) — every HTTP endpoint, traced
  through the router, middleware, handler, and usecase layers.
- [`Docs/Provisioning.md`](Docs/Provisioning.md) — everything involved in
  creating, listing, inspecting, and deleting compute instances, down to the
  individual OpenStack and Postgres calls.

## How it works

### Authentication

Auth is JWT-based with a short-lived access token and a longer-lived,
server-tracked refresh token:

- **Access token** — a signed JWT (HS256), 5 minutes TTL, self-contained
  (carries the user's ID and email as claims). Never stored server-side;
  verified on every request by re-checking its signature and expiry.
- **Refresh token** — a random 32-byte opaque string, 7 days TTL. The raw
  value is only ever handed to the client; the server stores its SHA-256
  hash so a leaked database can't be used to mint sessions. Refresh tokens
  are **rotated** on every use (the old one is revoked, a new one issued),
  and if an already-revoked refresh token is presented again, every
  outstanding refresh token for that user is revoked too, since reuse of a
  dead token is a strong signal it was stolen.

Endpoints: `POST /signup`, `POST /signin`, `POST /refresh`. Everything else
requires `Authorization: Bearer <access token>`.

### Compute provisioning

Each user's provisioned instances are tracked in a `compute` table
(`id`, `user_id`, `compute_service_id`, `name`, `status`, `created_at`),
keyed to the Nova server ID (`compute_service_id`). Creating, inspecting,
and deleting an instance always does two things: talk to OpenStack Nova via
gophercloud, and keep that `compute` row in sync — so a user only ever sees
and can act on instances that belong to them.

Endpoints: `POST /instances`, `GET /instances`, `GET /instances/{id}`,
`DELETE /instances/{id}` (all authenticated).

## Project layout

```
gopricloud/                        <- repo root
├── Docs/                          Implementation write-ups (this doc's siblings)
├── frontend/                      React + Vite + Tailwind + shadcn dashboard
├── gopricloud/                    Go module source (module "gopricloud")
│   ├── cmd/api/main.go            Composition root: wires config, DB, OpenStack
│   │                              client, usecases, handlers, HTTP server
│   ├── internal/
│   │   ├── domain/                Entities: User, RefreshToken, Compute,
│   │   │                          ComputeServer, ComputeCreateParams, errors
│   │   ├── repository/            Ports (interfaces): UserRepository,
│   │   │                          RefreshTokenRepository, ComputeRepository,
│   │   │                          ComputeProvider
│   │   ├── usecase/               Business logic: AuthUsecase, ComputeUsecase
│   │   ├── infrastructure/postgres/  Postgres implementations of the
│   │   │                          repository ports + schema bootstrap
│   │   ├── token/                 JWT access tokens + opaque refresh tokens
│   │   ├── config/                Env-var based configuration
│   │   └── delivery/http/         Router, middleware, handlers, DTOs
│   ├── openstack/compute/         gophercloud/Nova adapter implementing
│   │                              the ComputeProvider port
│   ├── clouds.yaml                OpenStack cloud credentials (gophercloud
│   │                              reads this from the working directory)
│   └── .env                       DATABASE_URL, JWT_SECRET, etc.
└── go.mod / go.sum
```

This follows a fairly standard ports-and-adapters layout: `domain` has no
dependencies on anything else; `repository` defines interfaces that
`usecase` depends on; `infrastructure/postgres` and `openstack/compute` are
concrete adapters implementing those interfaces; `delivery/http` is the
inbound adapter (HTTP) that depends on `usecase`. `cmd/api/main.go` is the
only place that wires concrete types together.

## Running it

**Backend** (from `gopricloud/gopricloud/`, so `.env` and `clouds.yaml` are
found in the working directory):

```
cd gopricloud/gopricloud
go run ./cmd/api
```

Required env vars (see `.env`): `DATABASE_URL` (Postgres DSN), `JWT_SECRET`.
Optional: `PORT` (default 8080), `JWT_ISSUER`, `OS_CLOUD_NAME` (cloud entry
name in `clouds.yaml`, default `openstack`).

**Frontend**:

```
cd frontend
npm install
npm run dev
```

The dev server proxies `/signup`, `/signin`, `/refresh`, `/test`, and
`/instances` to `http://localhost:8080`, so no CORS configuration is needed
in development.
