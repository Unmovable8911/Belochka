## What to build

Configure the production build to embed the compiled frontend assets into the Go binary using `go:embed`, producing a single self-contained binary that serves both the API and the frontend.

The `make build` target first builds the React frontend (`npm run build` in `web/`), then compiles the Go binary with the `web/dist` directory embedded. In production mode, the Go server serves the embedded static files for all non-API routes, with proper fallback to `index.html` for client-side routing.

During development, the embedded files are not used — the Vite dev server handles frontend serving.

## Acceptance criteria

- [ ] `make build` produces a single binary in `./bin/belochka`
- [ ] Build sequence: frontend build → Go compile with embed
- [ ] `go:embed` directive includes `web/dist/*` in the binary
- [ ] Production mode: non-API routes serve embedded static files
- [ ] Client-side routing works: unknown paths serve `index.html`
- [ ] API routes (`/api/*`) are not affected by static file serving
- [ ] Binary runs standalone without external files (except data directory)
- [ ] Development mode: embedded files not served (Vite dev server used instead)
- [ ] Binary size is reasonable (frontend assets gzipped or minified)

## Blocked by

- #1 Project Scaffold
