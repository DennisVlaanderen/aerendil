# Contributing to Aerendil

Thanks for your interest in Aerendil! This guide covers how to set up a
development environment, the conventions the codebase follows, and what to
expect when you open a pull request.

> [!NOTE]
> Aerendil is pre-v1 and not yet a working product. Expect things to move:
> APIs, storage formats, and the FQDP protocol can still change without a
> migration path.

## Ways to contribute

- **Report a bug** with the
  [bug report template](./.github/ISSUE_TEMPLATE/bug_report.md).
- **Suggest a feature** with the
  [feature request template](./.github/ISSUE_TEMPLATE/feature_request.md),
  or, when you can name the user, the action, and the acceptance criteria,
  the [user story template](./.github/ISSUE_TEMPLATE/user-story.md).
- **Pick up an existing issue.** Leave a comment saying you're working on
  it so nobody else duplicates the effort.
- **Improve the docs**, including the FQDP spec in
  [docs/fqdp.md](./docs/fqdp.md).

For anything larger than a small fix (a new endpoint, a change to the Raft
command/snapshot format, a protocol change, a new top-level package), open
or comment on an issue first so the approach can be agreed before you write
the code.

**Security issues** must not be reported in public issues. Follow
[SECURITY.md](./SECURITY.md) instead.

## Project layout

| Path       | What lives there                                                      |
| ---------- | --------------------------------------------------------------------- |
| `backend/` | Go server: Raft store, HTTP API, auth, and the FQDP protocol server   |
| `ui/`      | SvelteKit admin webapp (TypeScript, Tailwind, pnpm)                   |
| `proxy/`   | nginx config that fronts the backend and UI in `docker-compose.yml`   |
| `docs/`    | FQDP specification and the HTTP API standards                         |

[AGENTS.md](./AGENTS.md) has the detailed architecture walkthrough for both
the backend and the UI. It's written for AI coding agents, but it is the
most complete map of the codebase for humans too. Read the section for the
side you're about to change.

## Development setup

### Prerequisites

- **Go** at the version in [go.mod](./go.mod) (currently 1.27) or newer.
- **Node.js 22** and **pnpm**. The pnpm version is pinned through
  `packageManager` in [ui/package.json](./ui/package.json); running
  `corepack enable` once picks it up automatically.
- **Docker** with Compose, if you want to run the full stack.

### Running the full stack

```sh
docker compose up --build
```

This builds and starts the backend, the UI, and the nginx proxy. Open
<http://localhost:8090> and sign in with the development admin account
(`admin` / `admin123`). The defaults come from
[docker-compose.yml](./docker-compose.yml); to override them, copy
[.env.example](./.env.example) to `.env` and edit it. Never commit a `.env`
file or real secrets.

Raft data persists in the `aerendil-raft-data` volume. If a change alters
the stored command or snapshot format, wipe it with `docker compose down -v`
(see [Backend](#backend) below).

### Running each side on its own

For faster iteration, run the backend and UI directly:

```sh
# Backend, from the repo root: HTTP API on :8080, FQDP on :9001,
# Raft data in ./data (git-ignored).
go run ./backend/cmd/server

# UI, from ui/ in a second terminal. It talks to the backend at
# http://127.0.0.1:8080 unless AERENDIL_API_ORIGIN says otherwise.
cd ui
pnpm install
pnpm dev
```

The UI's browser-mode tests need Chromium. Install it once with
`pnpm exec playwright install chromium` from `ui/`.

## Making changes

### Branches

Branch off `main` and name the branch after the kind of change and, where
there is one, the issue number:

```text
feat/#112-consolidate-profile-menu
fix/create-flag-overwrites-existing-flags
docs/#30-expand-fqdp-docs
```

### Commit messages

Commits follow [Conventional Commits](https://www.conventionalcommits.org/):
`type(scope): summary`, written in the imperative.

- **Types:** `feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `ci`.
- **Scopes** name the area you touched, for example `api`, `store`, `auth`,
  `fqdp`, `ui`, or a resource such as `environments`. Leave the scope out
  when nothing fits.

```text
fix(store): close the BoltDB log store in Store.Close and on newRaft errors
docs(fqdp): add decision table for error handling and reject scenarios
```

Keep each commit focused on one change, and put the *why* in the body when
the summary alone doesn't explain it.

### Backend

- Keep new code inside the existing packages under `backend/internal/`.
  Add a new package only for a genuinely new concern.
- Comments follow [Go doc comment conventions](https://go.dev/doc/comment):
  doc comments on packages and exported identifiers start with the name
  they describe. Keep them short and explain *why*, not what.
- HTTP endpoints follow
  [docs/REST_API_Standards.md](./docs/REST_API_Standards.md), with the
  project-wide deviations listed under *Conventions* in
  [AGENTS.md](./AGENTS.md) (no version segment, `{error, code}` error
  bodies, unix-second timestamps, the shared pagination shape).
- Mutating routes are wrapped as
  `requirePermission(perm, withAudit(cfg, handler))`. Keep that order.
- The Raft command and snapshot format may change in breaking ways before
  v1. Upgrades wipe the Raft data directory rather than migrating it, so
  don't add migration code; call out the breaking change in your PR
  instead.
- `backend/internal/fqdp` is a stub. Treat [docs/fqdp.md](./docs/fqdp.md)
  as the target spec, not as a description of current behavior.

### UI

- Match the existing Svelte 5 and TypeScript idioms in `ui/src`.
- The UI only talks to the backend from server-side code
  (`+page.server.ts`, `src/lib/server/`, `src/routes/bff/`), never from the
  browser.
- User-facing strings go in `ui/messages/en.json` **and**
  `ui/messages/nl-nl.json`. Never edit `src/lib/paraglide/` by hand; it is
  generated.
- Formatting is Prettier's job: run `pnpm run format` before committing.

### Tests

Add or update tests alongside the change:

- **Go:** `*_test.go` next to the code under test.
- **UI logic and server code:** `*.test.ts` next to the module, run in Node.
- **Svelte components:** `*.svelte.test.ts` next to the component, run in a
  real Chromium through Vitest's browser mode.

A bug fix should come with a test that fails without the fix.

### Checks to run before pushing

CI is path-filtered, so run the checks for the side you touched. Run both
when a change spans them.

| Side    | Commands                                                                  |
| ------- | ------------------------------------------------------------------------- |
| Backend | `go build ./...`, `go vet ./...`, `go test -race ./...`, `go mod tidy`    |
| UI      | `pnpm run check`, `pnpm run lint`, `pnpm run build`, `pnpm test` (in `ui/`) |

CI also runs [golangci-lint](./.golangci.yml) on new code and `govulncheck`
for the backend. If you have them installed locally, `golangci-lint run`
and `govulncheck ./...` catch those failures before CI does.

Markdown is linted with markdownlint (see
[.markdownlint.jsonc](./.markdownlint.jsonc)): wrap prose at 80 columns.

## Pull requests

1. Open the PR against `main`. Keep it to one logical change; unrelated
   refactors belong in their own PR.
2. Link the issue it addresses (`Closes #123`) and describe what changed
   and why. Include screenshots for visible UI changes, and call out
   anything breaking (API shape, stored data format, protocol).
3. Update the docs your change affects: [docs/fqdp.md](./docs/fqdp.md) for
   protocol work, [.env.example](./.env.example) for new environment
   variables, and [AGENTS.md](./AGENTS.md) when you change the architecture
   or a convention it describes.
4. Make sure CI is green. Every PR runs:
   - **Go**: build, race-enabled tests, `go mod tidy` check, `go vet`,
     golangci-lint, and `govulncheck` (backend changes only).
   - **Node.js CI**: type-check, lint, build, and tests (UI changes only).
   - **Docker**: builds the affected image and scans it with Trivy.
   - **CodeQL**: static analysis across the whole repo.
5. To bring in changes from `main` before a review has started, either
   rebase your branch onto `main` or merge `main` into it, whichever you
   prefer. Once a review has started, respond to feedback by pushing new
   commits and merge `main` instead of rebasing, so reviewers can see what
   changed since they last looked.

Use a draft PR for work in progress. Open PRs with no activity for 30 days
are marked stale and closed 7 days later; drafts are exempt, and any push
or comment resets the timer.

Dependency updates are handled by Dependabot, so there's no need to open
PRs that only bump versions.

## AI-assisted contributions

Using AI coding tools is fine. [AGENTS.md](./AGENTS.md) is the single
source of instructions for them (`CLAUDE.md`, `.cursor/rules/`, and
`.github/copilot-instructions.md` all point to it), so update AGENTS.md
rather than adding tool-specific copies. Whatever tool wrote the code, you
are responsible for it: review it, understand it, and make sure it passes
the checks above before asking for a review.

## License

Aerendil is licensed under the [GNU AGPL-3.0](./LICENSE). By submitting a
contribution, you agree that it is licensed under the same terms.
