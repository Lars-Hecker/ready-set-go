# BaseApp Project Context

Monorepo structure: Go API backend + React frontend packages (pnpm workspace).

## Stack
- **Backend**: Go 1.25, Connect RPC (protobuf), PostgreSQL (pgx), sqlc, goose (migrations), Casbin (RBAC), JWT auth
- **Frontend**: React 19, TypeScript, Vite, TanStack Router/Query
- **Tooling**: buf (protobuf), Task (taskfile)

## Structure
```
apps/
  api/              # Go backend
    cmd/
      api/          # HTTP server binary
      migrate/      # Migration runner binary
      worker/       # Background worker binary (placeholder)
    domain/         # Business logic (user, workspace, subscription)
    infra/
      admin/        # Admin panel embed (references apps/admin/dist)
      auth/         # JWT, middleware
      perm/         # Casbin RBAC
    sql/
      migrations/   # Goose SQL migrations
      migrate.go    # Embedded migration runner
    gen/            # Generated code (sqlc queries, protobuf)
  admin/            # Admin panel React app (embedded into API)
  web/              # Public-facing React app
package/            # Shared packages (ui, bloc, dnd, ai)
proto/              # Protobuf definitions
```

## Key Patterns
- **Multi-binary**: Separate cmd/ entrypoints for api/migrate/worker
- **Embedded admin**: Admin SPA built to dist/, embedded via infra/admin, served on all non-API routes
- **Backend**: Domain-driven (domain handlers → infra/gen), JWT middleware, public/private endpoints
- **Frontend**: Component library in `package/ui`, state with zustand/bloc
- **Codegen**: `task generate` runs buf + sqlc; migrations embedded in binary

## Dev Commands
- `task dev:api` - Run API server
- `task dev:admin` - Run admin panel dev server (proxies /api to backend)
- `pnpm dev:web` - Run web frontend
- `task migrate` - Run DB migrations
- `task build:admin` - Build admin panel for embedding
