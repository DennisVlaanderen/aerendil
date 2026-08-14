# AGENTS.md

Instructions for AI coding agents working in this repository.

## Project overview

Aerendil is a distributed feature flagging toolkit. It is a work in progress, not
yet a working product. See [README.md](./README.md), [IDEA.md](./IDEA.md), and
[docs/premise.md](./docs/premise.md) for the project's goals, and
[docs/fqdp.md](./docs/fqdp.md) for the custom Layer 4 TCP protocol (FQDP) it
implements alongside its HTTP API.

The repo has three main parts:

- `backend/` — Go server (`cmd/server`), Raft-based distributed store
  (`internal/store`), the FQDP protocol server (`internal/fqdp`), the HTTP API
  (`internal/api`), and auth (`internal/auth`).
- `ui/` — SvelteKit admin webapp (TypeScript, Tailwind, pnpm).
- `proxy/` — nginx config used to front the backend/UI.

## Build, test, and lint

Go backend (run from the repo root, module is `aerendil`):

```sh
go build -v ./...
go test -v ./...
```

UI (run from `ui/`):

```sh
pnpm install
pnpm run build
pnpm run check   # svelte-kit sync + svelte-check
pnpm run lint    # prettier --check + eslint
pnpm test        # vitest, includes Playwright browser-mode tests
```

CI mirrors these exactly — see [.github/workflows/go.yml](./.github/workflows/go.yml)
(build/test with `-race`, `go vet` + golangci-lint, `govulncheck`) and
[.github/workflows/node.js.yml](./.github/workflows/node.js.yml) (`pnpm run
check`/`lint`/build/test). Run the relevant suite before considering backend
or UI work done. Two more workflows scan the whole repo instead of being
path-filtered: [.github/workflows/codeql.yml](./.github/workflows/codeql.yml)
(CodeQL) and [.github/workflows/docker.yml](./.github/workflows/docker.yml)
(build-validates both Dockerfiles + Trivy scan).

## Conventions

- Backend Go code follows the existing package layout under
  `backend/internal/*` — keep new code inside the appropriate package rather
  than adding new top-level packages unless a new concern truly warrants one.
- Backend Go comments follow https://go.dev/doc/comment: package and
  exported-identifier doc comments start with their own name. Keep
  comments short — explain *why*, not what, and skip anything the code
  already says.
- UI code is formatted with Prettier and linted with ESLint (`pnpm run
  format` / `pnpm run lint`); match existing Svelte 5 + TypeScript idioms in
  `ui/src`.
- Local runs are configured through environment variables documented in
  [.env.example](./.env.example) (Raft node identity/storage, JWT secret,
  bootstrap admin credentials). Never commit real secrets; the checked-in
  defaults are for local development only.
- `docker-compose.yml` is the reference way to run the full stack locally.

## Scope note

Only `backend/` has Go CI; only `ui/` has Node CI. When changing one side,
there's no need to run the other side's build/test unless the change is
cross-cutting. CodeQL and the Docker build-validation workflow run against
the whole repo regardless of which side changed.
