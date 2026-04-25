# CLAUDE.md — Sub2API

> AI API Gateway Platform for Subscription Quota Distribution

## Project Structure

```
sub2api/
├── backend/           # Go 1.26+ API server (Gin + Ent ORM + Wire DI)
├── frontend/          # Vue 3.4 SPA (Vite + Pinia + TailwindCSS + TypeScript)
├── deploy/            # Docker Compose, install scripts, config examples
├── docs/              # Payment integration guides
├── tools/             # Utility scripts
└── assets/            # Static assets (logos, partner images)
```

## Common Commands

### Build
```bash
make build              # Build backend + frontend
make build-backend       # Go binary only
make build-frontend      # Vue production build only
```

### Backend
```bash
cd backend
make generate           # Regenerate Ent schemas + Wire DI (required after schema changes)
make test               # All tests + linter
make test-unit           # Unit tests only (build tag: unit)
make test-integration    # Integration tests (build tag: integration)
make test-e2e            # E2E tests (build tag: e2e)
golangci-lint run ./...  # Lint (config: backend/.golangci.yml)
```

### Frontend
```bash
cd frontend
pnpm install            # MUST use pnpm, not npm
pnpm dev                # Dev server with HMR
pnpm build              # Production build
pnpm lint               # ESLint with auto-fix
pnpm typecheck           # TypeScript type checking
pnpm test               # Vitest unit tests
```

## Tech Stack

- **Backend**: Go 1.26, Gin, Ent ORM, Wire DI, PostgreSQL, Redis, JWT auth
- **Frontend**: Vue 3, Vite 5, TypeScript, Pinia, TailwindCSS, Axios
- **Payments**: Stripe, WeChat Pay, Alipay
- **Infra**: Docker Compose, Nginx/Caddy reverse proxy

## Key Conventions

- Frontend package manager is **pnpm only** — never use npm or yarn
- After modifying `backend/ent/schema/*.go`, run `make generate` and commit generated code
- After adding methods to interfaces, update all test stubs/mocks implementing that interface
- Backend can embed frontend into a single binary: `go build -tags embed`
- Tests use build tags: `unit`, `integration`, `e2e`
- Simple mode (`RUN_MODE=simple`) hides SaaS features and skips billing

## Architecture

- **Backend** follows clean architecture: `handler → service → repository → ent/schema`
- **Frontend** uses Vue 3 Composition API with `<script setup>` syntax
- Payment integrations live in `backend/internal/payment/`
- i18n translations are in `frontend/src/i18n/`
- API calls are centralized in `frontend/src/api/`

## CI

- **backend-ci.yml**: Go tests + golangci-lint on push/PR
- **security-scan.yml**: govulncheck, gosec, pnpm audit
- **release.yml**: Multi-platform builds on tag (v*)
