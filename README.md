# Enterprise API — Go Backend with RS256 JWT, RBAC, Session Management & Pluggable Storage

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/yourorg/enterprise-api)](https://goreportcard.com/report/github.com/yourorg/enterprise-api)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen)](https://github.com/yourorg/enterprise-api/pulls)

A production-ready **Go REST API** featuring **RS256 (RSA) JWT authentication**, **refresh token rotation**, **RBAC (role-based access control)** with granular permissions, **automatic session eviction** (max N active sessions per user), **JWT blacklisting** via Redis, **rate limiting / throttling**, **structured logging**, **activity audit trail**, and a **pluggable file storage interface** supporting local filesystem, AWS S3, Cloudflare R2, MinIO, and any S3-compatible backend.

Built with **Gin**, **pgx**, **Redis**, **Zap**, and clean **feature-based architecture**.

---

## Features

| Feature | Details |
|---------|---------|
| **RS256 JWT** | RSA private/public key pair for access & refresh tokens |
| **Refresh Token Rotation** | Each refresh issues new tokens & revokes the old pair |
| **Session Limit Enforcement** | Configurable max active sessions per user (default 2); oldest auto-evicted |
| **JWT Blacklist** | Revoked tokens blocked via Redis until natural expiry |
| **RBAC** | Action + resource permissions, roles with categories, user-role assignments |
| **Rate Limiting** | Per IP + device ID sliding window via Redis |
| **Token Bucket Throttling** | In-process rate limiter using `golang.org/x/time/rate` |
| **Activity Audit Log** | Async writes to PostgreSQL with IP, user agent, device ID |
| **Pluggable File Storage** | `Storage` interface with local, S3, Cloudflare R2, MinIO backends |
| **Structured Logging** | Zap logger with console + JSON file output |
| **Auto Migrations** | Embedded SQL migrations run on startup |
| **Graceful Shutdown** | Handles SIGINT/SIGTERM with timeout |
| **Docker Compose** | One-command local dev environment |

---

## Quick Start

### Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Docker & Docker Compose](https://docs.docker.com/compose/install/) (for PostgreSQL & Redis)
- OpenSSL (optional — use `make keys`)

### 1. Clone & enter

```bash
git clone https://github.com/yourorg/enterprise-api.git
cd enterprise-api
```

### 2. Generate RSA keys

```bash
make keys
```

### 3. Configure environment

```bash
cp .env.example .env
# Edit .env if needed (defaults work with docker-compose)
```

### 4. Start PostgreSQL & Redis

```bash
docker compose -f deployments/docker-compose.yml up -d postgres redis
```

### 5. Run the API

```bash
make run
```

The server starts at **http://localhost:8080**. Health check: `GET /health`

---

## API Endpoints

### Authentication (`/api/v1/auth`)

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/register` | No | Create account |
| `POST` | `/login` | No | Login, returns tokens |
| `POST` | `/refresh` | No | Rotate refresh token |
| `POST` | `/logout` | Yes | Revoke tokens + end session |

### Incidents (`/api/v1/incidents`) — RBAC protected

| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| `POST` | `/` | `create:incident` | Report incident |
| `GET` | `/` | `read:incident` | List incidents |

### Request/Response Examples

**Register**
```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"securepass123","full_name":"John Doe"}'
```

**Login**
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -H "X-Device-ID: my-device" \
  -d '{"email":"user@example.com","password":"securepass123"}'
```

**Authenticated request**
```bash
curl http://localhost:8080/api/v1/incidents \
  -H "Authorization: Bearer <access_token>"
```

---

## Project Structure

```
enterprise-api/
├── cmd/api/main.go              # Entry point
├── internal/
│   ├── config/config.go         # Env-based configuration
│   ├── database/                # PostgreSQL pool, Redis client, migrations
│   │   ├── migrations/          # 8 SQL migration files
│   │   ├── postgres.go
│   │   ├── redis.go
│   │   └── migrate.go
│   ├── middleware/              # Auth, CORS, logger, rate limiter, throttler, RBAC
│   │   ├── auth.go
│   │   ├── cors.go
│   │   ├── logger.go
│   │   ├── rate_limiter.go
│   │   ├── throttler.go
│   │   └── rbac.go
│   ├── modules/                 # Feature modules
│   │   ├── auth/                # Registration, login, logout, refresh, sessions
│   │   │   ├── domain/          # User, RefreshToken, Session entities
│   │   │   ├── dto/             # Request & response structs
│   │   │   ├── repository/      # UserRepo, SessionRepo
│   │   │   ├── service/         # Auth business logic
│   │   │   ├── handler/         # HTTP handlers
│   │   │   └── routes.go
│   │   ├── incident/            # Incident CRUD with RBAC
│   │   └── websocket/           # Placeholder for future WS support
│   ├── router/router.go         # Gin router setup, global middleware
│   └── shared/
│       ├── logger/logger.go     # Zap-based structured logging
│       ├── storage/             # Pluggable storage interface + backends
│       │   ├── storage.go       # Storage interface
│       │   ├── local.go         # Local filesystem backend
│       │   ├── s3.go            # S3-compatible backend (AWS, R2, MinIO)
│       │   └── factory.go       # Backend selection from config
│       └── utils/               # JWT helpers, response helpers, validator
├── deployments/                 # Docker, Docker Compose, Nginx, Prometheus
├── keys/                        # RSA private.pem, public.pem (gitignored)
├── migrations/seed.sql          # Seed data for permissions, roles
├── .env.example
├── go.mod
└── Makefile
```

---

## Session Management

By default, each user can have **up to 2 active sessions simultaneously**. When a third login occurs, the **oldest session is automatically evicted** — its refresh token is revoked and the user must re-authenticate on that device.

Configure via `MAX_ACTIVE_SESSIONS` in `.env`.

---

## File Storage

The `Storage` interface in `internal/shared/storage/storage.go` supports pluggable backends:

```go
type Storage interface {
    Upload(ctx, key, reader, contentType) (*FileInfo, error)
    Download(ctx, key) (io.ReadCloser, error)
    Delete(ctx, key) error
    Exists(ctx, key) (bool, error)
    URL(ctx, key) (string, error)
}
```

Set `STORAGE_DRIVER=local` for local filesystem, or `s3`/`r2` for S3-compatible backends. Configure credentials via the `STORAGE_S3_*` env vars.

---

## Configuration (.env)

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_PORT` | `8080` | HTTP listen port |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `REDIS_HOST` | `localhost` | Redis host |
| `JWT_ACCESS_EXPIRY_MINUTES` | `15` | Access token TTL |
| `JWT_REFRESH_EXPIRY_DAYS` | `7` | Refresh token TTL |
| `RATE_LIMIT_REQUESTS` | `100` | Max requests per window |
| `MAX_ACTIVE_SESSIONS` | `2` | Max concurrent sessions per user |
| `STORAGE_DRIVER` | `local` | Storage backend: `local`, `s3`, `r2` |

See `.env.example` for the full list.

---

## Deployment

```bash
# Build binary
make build

# Full stack with Docker Compose
docker compose -f deployments/docker-compose.yml up --build
```

The `Dockerfile` produces a scratch-based minimal image. Nginx reverse proxy config and Prometheus scrape config are provided under `deployments/`.

---

## Contributing

PRs are welcome! Please open an issue first to discuss changes.

1. Fork the repo
2. Create your feature branch (`git checkout -b feat/my-feature`)
3. Commit changes (`git commit -m 'Add my feature'`)
4. Push (`git push origin feat/my-feature`)
5. Open a Pull Request

---

## License

MIT
