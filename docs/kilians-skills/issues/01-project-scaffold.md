## What to build

Set up the project foundation: a Go backend serving a React frontend, with a development workflow that supports independent hot-reloading of both.

The Go side initializes a chi router with a `/api/health` endpoint. The React side is a Vite app with TypeScript, Tailwind CSS (dark theme only via `class="dark"` on root), and React Router with two placeholder routes: `/` (dashboard) and `/server/:id` (detail). The Vite dev server runs on port 53137, proxying `/api` and WebSocket traffic to the Go backend on port 53136.

A Makefile provides three targets: `make dev-backend` (runs Go with hot reload or manual restart), `make dev-frontend` (runs Vite dev server), and `make build` (placeholder for production build).

Install shadcn/ui and configure the component library with the following components available for later slices: button, card, dialog, input, select, label, table, badge, progress, sonner, alert.

## Acceptance criteria

- [ ] `go mod init belochka` with chi dependency, `cmd/server/main.go` entry point
- [ ] `GET /api/health` returns 200 with JSON response
- [ ] Go server listens on port 53136
- [ ] Vite React app with TypeScript on port 53137
- [ ] Vite proxy forwards `/api` and WebSocket to port 53136
- [ ] Tailwind CSS configured, dark theme applied globally
- [ ] React Router with `/` and `/server/:id` routes rendering placeholder content
- [ ] shadcn/ui installed and configured
- [ ] Makefile with `dev-backend`, `dev-frontend`, and `build` targets
- [ ] `internal/` directory structure created: config, api, model, ssh, monitor, hub, store
- [ ] slog text-format logging to stdout configured

## Blocked by

None - can start immediately.
