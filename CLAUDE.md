# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project overview

Aerendil is a distributed feature-flagging toolkit — **a work in progress, not
yet a working product**. Core premise: a Raft-replicated cluster of servers
stores flags and serves them over both a custom binary TCP protocol (FQDP)
and an HTTP API, so applications get near-real-time flag updates without
polling. See [README.md](./README.md), [IDEA.md](./IDEA.md),
[docs/premise.md](./docs/premise.md) (design intent) and
[docs/fqdp.md](./docs/fqdp.md) (wire protocol spec) for the full picture.
[docs/user-stories.md](./docs/user-stories.md) is the working backlog and is
useful for understanding *why* a piece of the API/UI looks the way it does,
including places where the current code is intentionally behind the spec.

The repo has three parts, each with its own toolchain:

- `backend/` — Go server. Module root is the repo root (module `aerendil`), entry
  point `backend/cmd/server/main.go`.
- `ui/` — SvelteKit 5 admin webapp (TypeScript, Tailwind, pnpm).
- `proxy/` — nginx config fronting backend + UI (see `docker-compose.yml`).

## Commands

Go backend (run from repo root):

```sh
go build -v ./...
go test -v ./...
go test -v ./backend/internal/store/...   # single package
go test -v -run TestName ./backend/internal/auth/...  # single test
```

UI (run from `ui/`):

```sh
pnpm install
pnpm run build
pnpm run check   # svelte-kit sync + svelte-check (types)
pnpm run lint    # prettier --check + eslint
pnpm run format  # prettier --write
pnpm test        # vitest --run, includes Playwright browser-mode tests
pnpm run test:unit -- path/to/file.spec.ts   # single test file, watch mode
```

CI mirrors these exactly and is path-filtered per side —
[.github/workflows/go.yml](./.github/workflows/go.yml) (build/test with
`-race`, `go vet` + golangci-lint, `govulncheck`) only triggers on
`backend/**`, [.github/workflows/node.js.yml](./.github/workflows/node.js.yml)
(`pnpm run check`/`lint`/build/test) only on `ui/**`. **Only run the suite
for the side you actually touched** unless the change is cross-cutting.
Two more workflows scan the whole repo instead of being path-filtered:
[.github/workflows/codeql.yml](./.github/workflows/codeql.yml) (CodeQL, on
every PR plus weekly) and
[.github/workflows/docker.yml](./.github/workflows/docker.yml) (build-
validates both Dockerfiles + Trivy scan — this is what would have caught a
`backend/Dockerfile` Go version drifting behind `go.mod`).

Full local stack: `docker-compose.yml` at the repo root (backend + UI + nginx
proxy). Backend env vars are documented in
[.env.example](./.env.example) (Raft node identity/storage, JWT secret,
bootstrap admin credentials) — copy to `.env` only to override the
docker-compose defaults; never commit real secrets.

## Backend architecture (`backend/internal/`)

`main.go` wires three independent pieces together and does not otherwise
contain logic: it opens the `store.Store`, starts the FQDP TCP listener in a
goroutine, and starts the HTTP API — all sharing the same underlying flag
store.

- **`store/`** — the Raft-replicated data layer, built directly on
  `hashicorp/raft`:
  - `cluster.go` (`newRaft`) sets up a Raft node backed by BoltDB
    (`raft-boltdb`) for the log/stable store and a file snapshot store. On
    first run with `Bootstrap: true` it bootstraps a single-member cluster;
    on restart it rejoins existing on-disk state instead of re-bootstrapping.
  - `fsm.go` is the Raft FSM: one `command{Op, Entity, Flag, User, Group,
    AuditEntry, Environment}` envelope dispatched by `Entity`, backing one
    in-memory map per entity (`flags`, `users`, `groups`, `auditEntries`,
    `environments`), all guarded by a single `sync.RWMutex`. A single
    composite `snapshotDoc` covers every entity for Raft snapshot/restore.
    There is deliberately no separate embedded DB for application data —
    Raft's replicated log plus FSM snapshot/restore is the durability
    mechanism. Project stance (pre-v1, no real deployments to migrate):
    schema-breaking changes to this command/snapshot format require wiping
    the Raft data dir on upgrade rather than migrating it — see the comment
    on `command` in `fsm.go`.
  - `store.go` (`Store`) is the public API: `Get`/`List` read the FSM
    directly; `Set`/`Delete` only succeed when the local node is Raft leader
    (they encode a `command` and call `raft.Apply`) — with today's
    single-node bootstrap that's always true, but multi-node behavior
    depends on this. Entity-specific operations are grouped into
    repositories — `Store.Flags()`, `Store.Users()`, `Store.Groups()`,
    `Store.Audits()`, `Store.Environments()` — rather than living directly
    on `Store`.
  - `flag.go` defines `Flag{Key, Enabled, Value, Version}`; `environment.go`
    defines `Environment{ID, Name, Order, Version}` (the deployment targets
    flags/permissions will be scoped to — see Epic 6 in
    `docs/user-stories.md`). `Version` on every entity is set from the Raft
    log index on apply, not a separately tracked counter.

- **`api/`** — `net/http` (stdlib `ServeMux` with Go 1.22+ method patterns,
  e.g. `"GET /api/flags"`), no framework. `RegisterRoutes` wires routes for
  flags, auth, users, groups, audits, environments, and application
  credentials (plus the unauthenticated `POST /api/oauth/token` OAuth2
  client-credentials endpoint those credentials authenticate against) —
  check `api.go` directly for the exact list rather than assuming it here,
  since it grows over time. Every mutating route follows the same
  `requirePermission(perm, withAudit(cfg, handler))` nesting (order matters
  — `withAudit` needs the principal `requirePermission` already attached to
  the request context). `middleware.go`'s `requirePermission` wraps a
  handler with bearer-token parsing plus a single permission-string check
  via the `auth` package. Route handlers currently do minimal validation —
  check the relevant `api/*.go` file directly rather than assuming REST
  conventions beyond what's there.

- **`auth/`** — `Service` in `service.go` issues/parses HS256 JWTs
  (`golang-jwt/jwt`) and hashes passwords with bcrypt. A human login token's
  claims are just `sub`/`exp`/`iat`; a service token issued via
  `GenerateServiceToken` for an application credential's OAuth2
  client-credentials exchange additionally carries a `typ: "service"` claim,
  which `AuthenticateToken` uses to resolve the subject against the
  application-credential store instead of `Users` — either way nothing
  beyond that is baked in, since `Service.Resolve`/`AuthenticateToken`
  re-fetch the live record and re-resolve permissions on every request.
  There is one seeded account: `bootstrap.go`'s `SeedAdminGroupAndUser`
  creates a configurable admin user (`AERENDIL_ADMIN_USERNAME` /
  `AERENDIL_ADMIN_PASSWORD`, bcrypt-hashed at startup) and an immutable,
  `System: true` Admin `Group` (`store.AdminGroupID = "admin"`) whose
  membership bypasses every permission check. All other access is
  group-based: `permissions.go` holds a flat catalog of permission strings
  (`PermFlagsRead/Write`, `PermUsersRead/Create/Update/Delete`,
  `PermGroupsRead/Create/Update/Delete`, `PermAuditsRead`,
  `PermEnvironmentsRead/Create/Update/Delete`,
  `PermApplicationCredentialsRead/Create/Update/Delete`) attached to
  `store.Group.Permissions` and resolved live — no caching, nothing baked
  into the JWT — via `Service.Resolve`, so group/permission changes and
  deactivation take effect on a user's very next request. An application
  credential is never an admin and its permission set is exactly its own
  `Scopes` (restricted to `auth.CredentialScopes`, currently
  `flags:read`/`flags:write` only) rather than anything resolved through
  group membership — see `authenticateServicePrincipal`. There is no
  `RoleAdmin`/`RoleUser` constant anywhere in the codebase; don't assume
  that model (see Epic 7 in `docs/user-stories.md` for where identity is
  headed next).

- **`fqdp/`** — the FQDP binary protocol server described in
  `docs/fqdp.md`. **Currently a stub**: `server.go` implements the
  length-prefixed framing (`readFrame`) and accepts connections, but only the
  Handshake message type is even parsed, and it always responds with an
  "not implemented" `Error` frame (`ErrHandshakeNotImplemented`) — no session,
  Query, Subscribe, or Update handling exists yet. `message_types.go` /
  `errors.go` hold the wire constants. Treat `docs/fqdp.md` as the target
  spec, not a description of current behavior, when working in this package.

## UI architecture (`ui/src/`)

SvelteKit with server-side rendering; the app never talks to FQDP (browsers
can't hold raw TCP sockets) — it only calls the backend's HTTP API, and only
from server-side code (`+page.server.ts` / `lib/server/` / `routes/bff/`),
never directly from the browser.

- **`lib/server/*.ts`** are the only bridge to the backend API
  (`AERENDIL_API_ORIGIN` env var, default `http://127.0.0.1:8080`) — one
  file per resource (`auth.ts`, `flags.ts`, `groups.ts`, `users.ts`,
  `auditLog.ts`, `environments.ts`), each following the same shape: an
  `API_ORIGIN` const, bearer-token `fetch` calls, and (for mutations) a
  `{ x } | { error, status }` result union that mirrors the backend's real
  HTTP status verbatim. `auth.ts`'s `getSession()` reads the
  `aerendil.auth` httpOnly cookie and validates it against `/api/auth/me`
  on every call (no local session cache) — the JWT itself lives only in
  that cookie. `setAuthCookie`/`clearAuthCookie` manage it (`secure` in
  non-dev).
- **`routes/bff/**`** is the proxy layer client components' `apiRequest()`
  calls hit (see `lib/client/api.ts`) — each `+server.ts` there checks
  `hasPermission` and then calls the matching `lib/server/*.ts` bridge
  function, returning the backend's real HTTP status instead of SvelteKit
  form actions' embedded-status convention.
- **`lib/permissions.ts`**'s `hasPermission(session, perm)` is the shared
  gate used both server-side (`+page.server.ts` load functions, throwing
  `error(403, ...)`) and client-side (`Sidebar.svelte`'s nav visibility) —
  there is no dedicated `dashboard/admin/` route prefix; permission gating
  happens per-page.
- **`routes/login/+page.server.ts`** performs the login action;
  **`routes/dashboard/+layout.server.ts`** shows the pattern for protecting
  a route: call `getSession`, `redirect(303, '/login')` if absent, then
  spread the session plus any layout-wide data into `page.data`. Follow
  this same pattern for any new authenticated route rather than inventing a
  different guard.
- `Sidebar.svelte` groups related admin pages into a collapsible dropdown
  (`User management`, `Application Settings`) that's hidden entirely when
  none of its children are visible to the current user — follow this
  pattern for grouping new settings-style pages rather than adding flat
  top-level links.
- **`lib/paraglide/`** is generated i18n code (from `project.inlang` /
  `@inlang/paraglide-js`) — don't hand-edit files under here; add new
  strings to `messages/en.json` / `messages/nl-nl.json` instead.

## Conventions (from AGENTS.md — keep in sync if that file changes)

- Backend Go code stays inside the existing `backend/internal/*` package
  layout; don't add new top-level packages unless a new concern truly
  warrants one.
- UI code is Prettier-formatted and ESLint-linted; match existing Svelte 5 +
  TypeScript idioms already in `ui/src`.
- `docker-compose.yml` is the reference way to run the full stack locally.
