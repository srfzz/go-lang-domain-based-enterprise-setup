# Enterprise API Starter — Go Backend Template

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen)](https://github.com/yourorg/enterprise-api/pulls)

A **production-ready Go REST API starter kit** with authentication (RS256 JWT), role-based access control (RBAC), session enforcement, audit logging, rate limiting, and pluggable file storage. Built so you can **clone, configure, and extend** instead of starting from scratch.

---

## What Is This Template?

This is a **feature-complete backend foundation** for any application that needs:

- **User authentication & session management** — register, login, refresh, logout with JWT tokens
- **Role-based access control** — granular permissions (action + resource), roles, and user-role assignments
- **Admin panel** — built-in admin user, user/role/permission CRUD APIs
- **Security** — RS256 signed JWTs, refresh token rotation, JWT blacklisting, session limits, rate limiting, throttling
- **Observability** — daily rotating structured logs (JSON), activity audit trail in PostgreSQL
- **File handling** — pluggable storage interface (local disk, S3, Cloudflare R2, MinIO)
- **Production readiness** — graceful shutdown, Docker Compose, embedded migrations, Nginx config

---

## Where Can It Be Used?

| Use Case | Why This Template Fits |
|----------|----------------------|
| **SaaS backend** | Auth, RBAC, multi-tenant ready, rate limiting, audit logs |
| **Internal tool API** | Admin panel, role management, session control |
| **Incident/Issue tracker** | Comes with an incident module; extend with your domain |
| **File-sharing platform** | Pluggable storage (local → S3/R2 with zero code changes) |
| **Microservice base** | Feature-based architecture — rip out what you don't need |

---

## Features

| Feature | Details |
|---------|---------|
| **RS256 JWT** | RSA key pair for access & refresh tokens |
| **Refresh Token Rotation** | New tokens issued on each refresh, old pair revoked |
| **Session Limits** | Configurable max active sessions per user (default 2); oldest auto-evicted |
| **JWT Blacklist** | Logged-out tokens blocked via Redis until expiry |
| **RBAC** | Action + resource permissions, roles with categories, user-role mapping |
| **Admin API** | Built-in admin: manage users, roles, permissions out of the box |
| **Default Admin Seed** | Auto-creates admin user + admin role + all permissions on first run |
| **Rate Limiting** | Per IP + device ID sliding window via Redis |
| **Token Bucket Throttling** | In-process rate limiter (`golang.org/x/time/rate`) |
| **Activity Audit Log** | Async writes to PostgreSQL with IP, user agent, device ID |
| **Daily Log Rotation** | JSON log files split by day: `app-2026-07-11.log` |
| **Structured Logging** | Zap with console + daily JSON file output |
| **Pluggable File Storage** | `Storage` interface — local, S3, Cloudflare R2, MinIO |
| **Auto Migrations** | Embedded SQL (8 migrations) run on startup |
| **Graceful Shutdown** | Handles SIGINT/SIGTERM with configurable timeout |
| **Docker Compose** | Single command to start API + PostgreSQL + Redis |

---

## Quick Start (5 minutes)

### Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Docker](https://docs.docker.com/compose/install/) (for PostgreSQL & Redis)

### 1. Clone

```bash
git clone https://github.com/yourorg/enterprise-api.git
cd enterprise-api
```

### 2. Generate RSA keys

```bash
make keys
```

### 3. Configure

```bash
cp .env.example .env
```

The defaults work with Docker Compose. Only change if needed.

### 4. Start databases

```bash
docker compose -f deployments/docker-compose.yml up -d postgres redis
```

### 5. Run

```bash
make run
```

The API starts at **http://localhost:8080**. Health check: `GET /health`

---

## Default Admin

On first run, the API **automatically seeds** an admin user with full access.

| Credential | Default Value |
|------------|---------------|
| Email | `admin@enterprise.com` |
| Password | `Admin123!` |
| Name | `System Admin` |

Login immediately and start creating users with specific roles.

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -H "X-Device-ID: my-device" \
  -d '{"email":"admin@enterprise.com","password":"Admin123!"}'
```

---

## API Reference

### Authentication `/api/v1/auth`

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/register` | No | Create account |
| `POST` | `/login` | No | Returns access + refresh tokens |
| `POST` | `/refresh` | No | Rotate refresh token |
| `POST` | `/logout` | Yes | Revoke tokens, end session |

### Admin Panel `/api/v1/admin` (requires `manage:admin` permission)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/users` | List all users with roles |
| `GET` | `/users/:id` | Get user details |
| `POST` | `/users` | Create user (with optional role IDs) |
| `PUT` | `/users/:id/roles` | Assign/replace roles |
| `GET` | `/roles` | List all roles with permissions |
| `POST` | `/roles` | Create role |
| `PUT` | `/roles/:id` | Update role |
| `DELETE` | `/roles/:id` | Delete role |
| `PUT` | `/roles/:id/permissions` | Assign permissions to role |
| `GET` | `/permissions` | List all permissions |
| `POST` | `/permissions` | Create permission |
| `PUT` | `/permissions/:id` | Update permission |
| `DELETE` | `/permissions/:id` | Delete permission |

### Incidents `/api/v1/incidents` (RBAC protected)

| Method | Path | Permission | Description |
|--------|------|------------|-------------|
| `POST` | `/` | `create:incident` | Report incident |
| `GET` | `/` | `read:incident` | List incidents |

### Examples

```bash
# Register a new user
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"securepass123","full_name":"John Doe"}'

# Admin creates a user with roles
curl -X POST http://localhost:8080/api/v1/admin/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <admin_access_token>" \
  -d '{"email":"operator@example.com","password":"securepass123","full_name":"Operator User","role_ids":["<role-uuid>"]}'

# Authenticated request
curl http://localhost:8080/api/v1/incidents \
  -H "Authorization: Bearer <access_token>"
```

---

## Project Structure

```
enterprise-api/
├── cmd/api/main.go              # Entry point
├── internal/
│   ├── config/config.go         # Environment-based configuration
│   ├── database/                # PostgreSQL pool, Redis, embedded migrations
│   │   ├── migrations/          # 8 migration files
│   │   ├── postgres.go
│   │   ├── redis.go
│   │   └── migrate.go
│   ├── middleware/              # Auth, CORS, logger, rate limiter, throttler, RBAC
│   ├── modules/                 # Feature modules (add yours here)
│   │   ├── admin/               # Admin panel: users, roles, permissions CRUD
│   │   ├── auth/                # Authentication: register, login, logout, sessions
│   │   ├── incident/            # Incident management (example domain module)
│   │   └── websocket/           # Placeholder for WebSocket support
│   ├── router/router.go         # Gin router, global middleware wiring
│   └── shared/
│       ├── logger/              # Daily rotating JSON + console logs
│       ├── storage/             # Pluggable storage interface + backends
│       └── utils/               # JWT helpers, response helpers, validator
├── deployments/                 # Dockerfile, Docker Compose, Nginx, Prometheus
├── keys/                        # RSA key pair (gitignored)
├── .env.example
├── go.mod
└── Makefile
```

To add a new feature module:
1. Create `internal/modules/<name>/` with `domain/`, `dto/`, `repository/`, `service/`, `handler/`, `routes.go`
2. Register routes in `internal/router/router.go`
3. Add any new tables to `internal/database/migrations/`

---

## Configuration

Key `.env` variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `APP_PORT` | `8080` | HTTP port |
| `JWT_ACCESS_EXPIRY_MINUTES` | `15` | Access token lifetime |
| `JWT_REFRESH_EXPIRY_DAYS` | `7` | Refresh token lifetime |
| `MAX_ACTIVE_SESSIONS` | `2` | Concurrent sessions per user |
| `RATE_LIMIT_REQUESTS` | `100` | Requests per window per IP+device |
| `STORAGE_DRIVER` | `local` | `local`, `s3`, or `r2` |
| `ADMIN_EMAIL` | `admin@enterprise.com` | Default admin email |
| `ADMIN_PASSWORD` | `Admin123!` | Default admin password |

Full list in `.env.example`.

---

## Deployment

```bash
# Build binary
make build

# Full stack with Docker Compose
docker compose -f deployments/docker-compose.yml up --build
```

Includes production-grade Nginx reverse proxy config and Prometheus scrape config under `deployments/`.

---

## Extending

- **New domain module** — copy `internal/modules/incident/` as a template, rename, add your logic
- **New storage backend** — implement the `Storage` interface, add a case in `factory.go`
- **New middleware** — add to `internal/middleware/`, wire in `router.go`
- **New migration** — add `000009_*.up.sql` / `*.down.sql` to `internal/database/migrations/`

---

## License

MIT
