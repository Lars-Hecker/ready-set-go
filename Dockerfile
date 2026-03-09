# syntax=docker/dockerfile:1

# ==============================================================================
# Stage 1: Node dependencies and frontend builds
# ==============================================================================
FROM node:22-alpine AS frontend

WORKDIR /app

# Install pnpm
RUN corepack enable && corepack prepare pnpm@latest --activate

# Copy package files for dependency installation
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.json ./
COPY apps/admin/package.json apps/admin/
COPY apps/web/package.json apps/web/
COPY package/ package/

# Install dependencies
RUN pnpm install --frozen-lockfile

# Copy source files
COPY apps/admin apps/admin
COPY apps/web apps/web

# Build admin panel
RUN pnpm --filter @baseapp/admin build

# Build web frontend
RUN pnpm --filter @baseapp/web build

# ==============================================================================
# Stage 2: Go build
# ==============================================================================
FROM golang:1.25-alpine AS api

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git

# Copy go module files
COPY apps/api/go.mod apps/api/go.sum ./apps/api/

# Download dependencies
WORKDIR /app/apps/api
RUN go mod download

# Copy Go source
WORKDIR /app
COPY apps/api apps/api

# Copy built admin panel into the embed location
COPY --from=frontend /app/apps/admin/dist apps/api/infra/admin/dist

# Build all binaries
WORKDIR /app/apps/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/worker ./cmd/worker

# ==============================================================================
# Stage 3: Runtime image
# ==============================================================================
FROM alpine:3.21 AS runtime

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 app && adduser -u 1000 -G app -s /bin/sh -D app

# Copy binaries from build stage
COPY --from=api /bin/api /bin/api
COPY --from=api /bin/migrate /bin/migrate
COPY --from=api /bin/worker /bin/worker

# Copy web frontend for Caddy to serve
COPY --from=frontend /app/apps/web/dist /srv/web

# Set ownership
RUN chown -R app:app /srv

USER app

EXPOSE 8080

CMD ["/bin/api"]