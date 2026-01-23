# CLAUDE.md - AI Assistant Guide for Sub2API

**Last Updated**: 2026-01-23
**Project**: Sub2API - AI API Gateway Platform for Subscription Quota Distribution
**Repository**: https://github.com/Wei-Shaw/sub2api

---

## Table of Contents

1. [Project Overview](#project-overview)
2. [Repository Structure](#repository-structure)
3. [Tech Stack](#tech-stack)
4. [Development Setup](#development-setup)
5. [Architecture Patterns](#architecture-patterns)
6. [Key Components](#key-components)
7. [Database Schema](#database-schema)
8. [Testing Guidelines](#testing-guidelines)
9. [Build & Deployment](#build--deployment)
10. [Code Generation & Tools](#code-generation--tools)
11. [Development Workflows](#development-workflows)
12. [Important Conventions](#important-conventions)
13. [Security Considerations](#security-considerations)
14. [Troubleshooting](#troubleshooting)

---

## Project Overview

Sub2API is an AI API gateway platform that distributes and manages API quotas from AI product subscriptions (like Claude Code $200/month). Users access upstream AI services through platform-generated API Keys, while the platform handles authentication, billing, load balancing, and request forwarding.

### Core Features

- **Multi-Account Management**: Support for OAuth, API Key, and Cookie-based upstream accounts
- **API Key Distribution**: Generate and manage API Keys for users
- **Precise Billing**: Token-level usage tracking and cost calculation
- **Smart Scheduling**: Intelligent account selection with sticky sessions
- **Concurrency Control**: Per-user and per-account limits
- **Rate Limiting**: Configurable request and token rate limits
- **Admin Dashboard**: Web interface for monitoring and management

### Supported Platforms

- **Claude** (Anthropic API)
- **OpenAI** (GPT models)
- **Gemini** (Google AI)
- **Antigravity** (Alternative Claude provider)

---

## Repository Structure

```
sub2api/
├── backend/                          # Go backend service
│   ├── cmd/
│   │   ├── server/                   # Application entry point
│   │   │   ├── main.go               # Main entry with setup wizard
│   │   │   ├── wire.go               # Wire DI configuration
│   │   │   └── wire_gen.go           # Generated Wire code
│   │   └── jwtgen/                   # JWT token generator utility
│   ├── ent/                          # Ent ORM models & generated code
│   │   ├── schema/                   # Schema definitions (source of truth)
│   │   └── *.go                      # Generated CRUD operations
│   ├── internal/
│   │   ├── config/                   # Configuration loading (Viper-based)
│   │   ├── handler/                  # HTTP handlers & routes
│   │   │   ├── admin/                # Admin API handlers
│   │   │   ├── gateway/              # Gateway proxy handlers
│   │   │   ├── user/                 # User-facing handlers
│   │   │   └── setup/                # Setup wizard handlers
│   │   ├── service/                  # Business logic layer (135+ services)
│   │   │   ├── gateway_service.go    # Core API gateway logic
│   │   │   ├── billing_service.go    # Billing & usage tracking
│   │   │   ├── token_*_service.go    # Token management & refresh
│   │   │   ├── ops_*_service.go      # Operations metrics & alerts
│   │   │   └── *_oauth_service.go    # Platform OAuth integrations
│   │   ├── repository/               # Data access & caching layer
│   │   ├── middleware/               # Auth, CORS, security headers
│   │   ├── server/                   # Router setup & server config
│   │   ├── pkg/                      # Utility packages
│   │   │   ├── claude/               # Claude API client
│   │   │   ├── openai/               # OpenAI API client
│   │   │   ├── gemini/               # Gemini API client
│   │   │   └── errors/               # Structured error types
│   │   ├── util/                     # Utility functions
│   │   ├── setup/                    # Setup wizard logic
│   │   └── web/                      # Embedded frontend dist folder
│   ├── migrations/                   # SQL migration files (19+ versions)
│   └── resources/                    # Static resources
│
├── frontend/                         # Vue 3 frontend
│   ├── src/
│   │   ├── api/                      # API client modules
│   │   │   ├── admin/                # Admin API calls
│   │   │   ├── auth.ts               # Authentication API
│   │   │   ├── keys.ts               # API key management
│   │   │   ├── usage.ts              # Usage statistics
│   │   │   └── client.ts             # Axios HTTP client
│   │   ├── router/                   # Vue Router with lazy loading
│   │   ├── stores/                   # Pinia state management
│   │   │   ├── app.ts                # Global app state
│   │   │   ├── auth.ts               # Auth state with auto-refresh
│   │   │   ├── adminSettings.ts      # Admin settings state
│   │   │   └── subscriptions.ts      # Subscription state
│   │   ├── components/               # Vue components by domain
│   │   │   ├── auth/                 # Login, register forms
│   │   │   ├── admin/                # Admin dashboard components
│   │   │   ├── account/              # Account management
│   │   │   ├── charts/               # Chart components
│   │   │   ├── layout/               # Layout wrappers
│   │   │   └── common/               # Shared components
│   │   ├── views/                    # Page components
│   │   │   ├── auth/                 # OAuth callbacks, email verify
│   │   │   ├── user/                 # Dashboard, API keys, usage
│   │   │   ├── admin/                # Admin dashboard
│   │   │   └── setup/                # Setup wizard
│   │   ├── composables/              # Vue composables
│   │   ├── i18n/                     # Internationalization (en/zh)
│   │   ├── types/                    # TypeScript type definitions
│   │   └── utils/                    # Utility functions
│   ├── public/                       # Static assets
│   └── package.json                  # Frontend dependencies
│
├── deploy/                           # Deployment configurations
│   ├── docker-compose.yml            # Full stack deployment
│   ├── .env.example                  # Environment variables template
│   ├── config.example.yaml           # Full config file template
│   └── install.sh                    # One-click installation script
│
├── .github/workflows/                # CI/CD pipelines
│   ├── backend-ci.yml                # Backend testing & linting
│   ├── release.yml                   # GoReleaser build & publish
│   └── security-scan.yml             # Security scanning
│
├── Dockerfile                        # Multi-stage Docker build
├── Makefile                          # Top-level build commands
└── README.md                         # User-facing documentation
```

---

## Tech Stack

### Backend

| Component | Technology | Version |
|-----------|------------|---------|
| Language | Go | 1.25.5 |
| Web Framework | Gin | Latest |
| ORM | Ent | Latest |
| Dependency Injection | Google Wire | Latest |
| Database | PostgreSQL | 15+ |
| Cache/Queue | Redis | 7+ |
| Config | Viper | Latest |
| Auth | JWT (golang-jwt) | Latest |

### Frontend

| Component | Technology | Version |
|-----------|------------|---------|
| Framework | Vue | 3.4+ |
| Language | TypeScript | Latest |
| Build Tool | Vite | 5+ |
| State Management | Pinia | Latest |
| Router | Vue Router | Latest |
| Styling | TailwindCSS | Latest |
| Charts | Chart.js | Latest |
| HTTP Client | Axios | Latest |
| Package Manager | pnpm | Latest |

### Infrastructure

- **Docker**: Multi-stage builds, Alpine-based images
- **Docker Compose**: Development & production stacks
- **GoReleaser**: Cross-platform binary releases
- **GitHub Actions**: CI/CD automation
- **systemd**: Service management for Linux deployments

---

## Development Setup

### Prerequisites

```bash
# Backend
Go 1.21+
PostgreSQL 15+
Redis 7+

# Frontend
Node.js 18+
pnpm (npm install -g pnpm)
```

### Quick Start

```bash
# 1. Clone repository
git clone https://github.com/Wei-Shaw/sub2api.git
cd sub2api

# 2. Install frontend dependencies
cd frontend
pnpm install

# 3. Build frontend (output to backend/internal/web/dist/)
pnpm run build

# 4. Build backend with embedded frontend
cd ../backend
go build -tags embed -o sub2api ./cmd/server

# 5. Create config file
cp ../deploy/config.example.yaml ./config.yaml
nano config.yaml  # Edit database/redis credentials

# 6. Run application
./sub2api

# Access setup wizard at http://localhost:8080
```

### Development Mode (Hot Reload)

```bash
# Terminal 1: Backend with hot reload
cd backend
go run ./cmd/server

# Terminal 2: Frontend dev server (Vite HMR)
cd frontend
pnpm run dev  # Runs on http://localhost:5173
```

**Note**: In development mode, frontend runs separately on port 5173. Backend API calls are proxied via Vite config.

---

## Architecture Patterns

### Clean Architecture Layers

```
┌─────────────────────────────────────────┐
│          HTTP Handlers                  │  ← Gin routes, request validation
├─────────────────────────────────────────┤
│          Service Layer                  │  ← Business logic, orchestration
├─────────────────────────────────────────┤
│        Repository Layer                 │  ← Data access, caching
├─────────────────────────────────────────┤
│          Ent ORM / Redis                │  ← Database & cache abstraction
└─────────────────────────────────────────┘
```

### Dependency Injection (Wire)

- **Configuration**: `backend/cmd/server/wire.go`
- **Provider Sets**: Each layer exports a `ProviderSet` (e.g., `service.ProviderSet`)
- **Generation**: Run `go generate ./cmd/server` to regenerate `wire_gen.go`
- **Cleanup**: Cleanup function provided in Wire graph for graceful shutdown

**Example Provider Set** (`internal/service/wire.go`):
```go
var ProviderSet = wire.NewSet(
    NewAuthService,
    NewUserService,
    NewGatewayService,
    // ... 135+ service constructors
)
```

### Middleware Chain

```
Request → Logger → CORS → SecurityHeaders → FrontendStatic → Router
                                                              ├─ JWTAuth (user endpoints)
                                                              ├─ AdminAuth (admin endpoints)
                                                              └─ APIKeyAuth (gateway endpoints)
```

### Caching Strategy

**Redis Caching Layers**:
- **API Key Auth Cache**: 5-minute TTL, invalidated on key updates
- **Session Cache**: Sticky sessions with 1-hour TTL
- **Token Cache**: OAuth tokens with refresh-before-expiry
- **Billing Cache**: Account quotas with invalidation on billing events

**In-Memory Caching**:
- Concurrency limit tracking (per-user, per-account)
- App settings (refreshed on updates)

**Cache Invalidation Pattern**:
```go
// Example: Invalidate API key cache on update
func (s *APIKeyService) UpdateKey(ctx context.Context, keyID int) error {
    // 1. Update database
    if err := s.repo.Update(ctx, keyID, updates); err != nil {
        return err
    }

    // 2. Invalidate cache
    return s.cache.Delete(ctx, cacheKey(keyID))
}
```

---

## Key Components

### 1. API Gateway (`internal/service/gateway_service.go`)

**Request Flow**:
```
Client → APIKeyAuth → GatewayHandler.HandleRequest
                      ↓
                ParseGatewayRequest (single-pass JSON parsing)
                      ↓
                GatewayService.RouteRequest
                      ├─ Account Selection (sticky sessions + load balancing)
                      ├─ Token Provider (OAuth refresh if needed)
                      ├─ Model Mapping & Transformation
                      ├─ Rate Limiting & Concurrency Checks
                      └─ HTTP Upstream Proxy
                      ↓
                Response Processing (streaming SSE, billing)
                      ↓
                Client Response
```

**Key Features**:
- **Sticky Sessions**: SHA256 hash of conversation context → account binding (1-hour TTL)
- **Account Switching**: Up to 10 fallback attempts on quota/rate limit errors
- **Streaming Support**: SSE data line parsing with ping/heartbeat insertion
- **Request Parsing Optimization**: Single JSON unmarshal, multiple field extractions

**File Reference**: `backend/internal/service/gateway_service.go:1-2500` (135KB)

### 2. Token Management

**Token Refresh Architecture**:
```go
// TokenRefreshService (background refresh)
backend/internal/service/token_refresh_service.go
    ├─ Periodic scan of expiring tokens (configurable interval)
    ├─ Async refresh with retry logic
    └─ Cache invalidation on success

// TokenCacheInvalidator (event-driven invalidation)
backend/internal/service/token_cache_invalidator.go
    ├─ Invalidates token cache on manual refresh
    └─ Coordinates with billing cache service
```

**Platform Token Providers**:
- `claude_token_provider.go`: Claude OAuth token refresh
- `openai_token_provider.go`: OpenAI API key management
- `gemini_oauth_service.go`: Gemini OAuth flow
- `antigravity_oauth_service.go`: Antigravity OAuth integration

### 3. Billing System

**Billing Flow**:
```
Request Start → Pre-Request Check (quota available?)
                      ↓
                Upstream API Call
                      ↓
                Response → Token Count Extraction
                      ↓
                BillingService.RecordUsage
                      ├─ User balance deduction
                      ├─ Account usage increment
                      ├─ UsageLog creation
                      └─ Cache invalidation
```

**Circuit Breaker Pattern** (`config.yaml`):
```yaml
billing:
  circuit_breaker:
    enabled: true  # Fail closed on billing errors (safer default)
```

**Files**:
- `backend/internal/service/billing_service.go`: Core billing logic
- `backend/internal/service/usage_service.go`: Usage log management
- `backend/internal/service/account_usage_service.go`: Account-level tracking

### 4. Operations Metrics (`ops_*_service.go`)

**Ops Services**:
- `ops_service.go`: Main operations service with metrics aggregation
- `ops_metrics_collector.go`: Real-time metrics collection
- `ops_aggregation_service.go`: Time-series aggregation (hourly, daily)
- `ops_alert_evaluator_service.go`: Alert threshold evaluation
- `ops_cleanup_service.go`: Old metrics cleanup
- `ops_scheduled_report_service.go`: Scheduled reporting

**Health Scoring System**:
```go
// Account health score = success_rate * availability * performance_factor
// Used for intelligent account selection
```

### 5. Setup Wizard

**First-Run Setup Flow**:
```
1. Check if .installed file exists → Skip wizard if present
2. Database Connection Test
3. Redis Connection Test
4. Admin Account Creation
5. Initial Settings Configuration
6. Create .installed marker file
```

**Files**:
- `backend/internal/setup/wizard.go`: Setup wizard logic
- `frontend/src/views/setup/`: Setup UI components

---

## Database Schema

### Core Tables (11 entities)

#### 1. User
```go
// backend/ent/schema/user.go
type User struct {
    ID              int       `json:"id"`
    Email           string    `json:"email" unique:"true"`
    PasswordHash    string    `json:"-"`
    Balance         float64   `json:"balance" decimal:"precision=20,scale=10"`
    Concurrency     int       `json:"concurrency" default:"5"`
    Role            string    `json:"role" default:"user"` // user, admin
    Status          string    `json:"status" default:"active"` // active, suspended
    IsEmailVerified bool      `json:"is_email_verified" default:"false"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
    DeletedAt       *time.Time `json:"deleted_at"` // Soft delete

    // Edges
    APIKeys         []APIKey
    Subscriptions   []UserSubscription
    UsageLogs       []UsageLog
    AllowedGroups   []UserAllowedGroup
}
```

#### 2. Account
```go
// backend/ent/schema/account.go
type Account struct {
    ID           int       `json:"id"`
    Name         string    `json:"name"`
    Platform     string    `json:"platform"` // claude, openai, gemini, antigravity
    AuthType     string    `json:"auth_type"` // api_key, oauth, cookie
    Credentials  string    `json:"credentials" sensitive:"true"` // JSONB encrypted
    Status       string    `json:"status" default:"active"` // active, inactive, error
    Concurrency  int       `json:"concurrency" default:"3"`
    ExpiresAt    *time.Time `json:"expires_at"`
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
    DeletedAt    *time.Time `json:"deleted_at"`

    // Edges
    Groups       []AccountGroup
    UsageLogs    []UsageLog
}
```

#### 3. Group
```go
// backend/ent/schema/group.go
type Group struct {
    ID              int       `json:"id"`
    Name            string    `json:"name"`
    SchedulePolicy  string    `json:"schedule_policy"` // round_robin, random, sticky
    DispatchPolicy  string    `json:"dispatch_policy"` // quota, performance
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
    DeletedAt       *time.Time `json:"deleted_at"`

    // Edges
    Accounts        []AccountGroup  // Many-to-many through AccountGroup
    AllowedUsers    []UserAllowedGroup
    APIKeys         []APIKey
}
```

#### 4. APIKey
```go
// backend/ent/schema/api_key.go
type APIKey struct {
    ID          int       `json:"id"`
    UserID      int       `json:"user_id"`
    GroupID     *int      `json:"group_id"` // Optional: restrict to group
    Key         string    `json:"key" unique:"true"` // e.g., sk-xxx
    Name        string    `json:"name"`
    Status      string    `json:"status" default:"active"`
    ExpiresAt   *time.Time `json:"expires_at"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    DeletedAt   *time.Time `json:"deleted_at"`

    // Edges
    User        User
    Group       *Group
    UsageLogs   []UsageLog
}
```

#### 5. UsageLog
```go
// backend/ent/schema/usage_log.go
type UsageLog struct {
    ID              int       `json:"id"`
    UserID          int       `json:"user_id"`
    AccountID       int       `json:"account_id"`
    APIKeyID        int       `json:"api_key_id"`
    Model           string    `json:"model"`
    InputTokens     int       `json:"input_tokens"`
    OutputTokens    int       `json:"output_tokens"`
    TotalTokens     int       `json:"total_tokens"`
    Cost            float64   `json:"cost" decimal:"precision=20,scale=10"`
    CreatedAt       time.Time `json:"created_at" index:"true"` // Indexed for range queries

    // Edges
    User            User
    Account         Account
    APIKey          APIKey
}

// UsageCleanupTask: Manages cleanup of old logs (retention policy)
```

#### 6. Setting
```go
// backend/ent/schema/setting.go
type Setting struct {
    ID        int       `json:"id"`
    Key       string    `json:"key" unique:"true"` // e.g., "site_name", "smtp_host"
    Value     string    `json:"value"` // JSONB for complex values
    Category  string    `json:"category"` // site, email, billing, etc.
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

### Schema Modifications

**When editing Ent schemas** (`backend/ent/schema/*.go`):

1. **Regenerate Ent Code**:
   ```bash
   cd backend
   go generate ./ent
   ```

2. **Regenerate Wire DI**:
   ```bash
   go generate ./cmd/server
   ```

3. **Create Migration** (if needed):
   ```bash
   # Manual migration file creation in backend/migrations/
   # Follow naming: 000_description.sql
   ```

**Key Schema Patterns**:
- **Soft Deletes**: `DeletedAt *time.Time` + partial unique indexes
- **Decimal Precision**: `Balance float64` with `decimal:"precision=20,scale=10"` tag
- **JSONB Storage**: `Credentials string` for flexible encrypted data
- **Mixins**: `TimeMixin`, `SoftDeleteMixin` for reusable fields

---

## Testing Guidelines

### Test Organization

```bash
backend/
├── internal/
│   ├── service/
│   │   ├── auth_service.go
│   │   └── auth_service_test.go           # Unit tests
│   ├── handler/
│   │   └── admin/
│   │       ├── user_handler.go
│   │       └── user_handler_test.go       # Handler tests
│   └── repository/
│       ├── user_repository.go
│       └── user_repository_integration_test.go  # Integration tests
```

### Test Tags

```go
//go:build unit
// +build unit

// Unit tests: Fast, mocked dependencies, no external services
```

```go
//go:build integration
// +build integration

// Integration tests: Real PostgreSQL/Redis via testcontainers
```

### Running Tests

```bash
# All tests (unit + integration + linting)
make test

# Backend only
make test-backend

# Frontend only (lint + typecheck)
make test-frontend

# Backend: Unit tests only (fast)
cd backend
make test-unit

# Backend: Integration tests only (slower)
make test-integration

# Backend: With coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Testing Best Practices

1. **Use Table-Driven Tests**:
   ```go
   func TestUserService_CreateUser(t *testing.T) {
       tests := []struct {
           name    string
           input   CreateUserRequest
           wantErr bool
       }{
           {"valid user", CreateUserRequest{Email: "test@example.com"}, false},
           {"duplicate email", CreateUserRequest{Email: "exist@example.com"}, true},
       }

       for _, tt := range tests {
           t.Run(tt.name, func(t *testing.T) {
               // Test implementation
           })
       }
   }
   ```

2. **Mock External Dependencies**:
   - Use interfaces for services
   - Mock HTTP clients for upstream API calls
   - Mock Redis/Database for unit tests

3. **Integration Test Setup**:
   ```go
   func setupTestDB(t *testing.T) *ent.Client {
       // Use testcontainers for PostgreSQL
       // Run migrations
       // Return connected client
   }
   ```

4. **Frontend Testing**:
   ```bash
   cd frontend
   pnpm run test        # Vitest unit tests
   pnpm run lint:check  # ESLint
   pnpm run typecheck   # TypeScript validation
   ```

---

## Build & Deployment

### Build Commands

```bash
# Full build (frontend + backend)
make build

# Backend only (no embedded frontend)
cd backend
make build

# Backend with embedded frontend (production)
cd backend
go build -tags embed -o bin/server ./cmd/server

# Frontend only (output to backend/internal/web/dist/)
cd frontend
pnpm run build
```

### Docker Build (Multi-Stage)

**Dockerfile stages**:
```dockerfile
# Stage 1: Frontend builder (Node 24-alpine)
FROM node:24-alpine AS frontend-builder
# ... build frontend → /app/dist

# Stage 2: Backend builder (Go 1.25.5-alpine)
FROM golang:1.25.5-alpine AS backend-builder
# ... copy frontend dist → backend/internal/web/dist
# ... go build -tags embed

# Stage 3: Runtime (Alpine 3.20)
FROM alpine:3.20
# ... copy binary, create non-root user, set entrypoint
```

**Build & Run**:
```bash
# Build image
docker build -t sub2api:latest .

# Run container
docker run -p 8080:8080 \
  -e DATABASE_HOST=postgres \
  -e REDIS_HOST=redis \
  -v /path/to/config.yaml:/app/config.yaml \
  sub2api:latest
```

### Docker Compose Deployment

```bash
cd deploy

# Start all services (app + postgres + redis)
docker-compose up -d

# View logs
docker-compose logs -f sub2api

# Restart app only
docker-compose restart sub2api

# Stop all
docker-compose down
```

### Binary Deployment (systemd)

```bash
# One-click installation
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/install.sh | sudo bash

# Manual systemd service
sudo systemctl start sub2api
sudo systemctl enable sub2api
sudo systemctl status sub2api

# View logs
sudo journalctl -u sub2api -f

# Restart
sudo systemctl restart sub2api
```

**Service file location**: `/etc/systemd/system/sub2api.service`

### GoReleaser (GitHub Releases)

**Configuration**: `.goreleaser.yaml`

**Triggered by**: Pushing a Git tag (e.g., `v1.0.0`)

**Artifacts**:
- Linux: amd64, arm64
- Windows: amd64
- Darwin: amd64, arm64
- Format: tar.gz archives

**Build flags injected**:
```go
// cmd/server/main.go
var (
    Version   = "dev"
    Commit    = "unknown"
    Date      = "unknown"
    BuildType = "development"
)
```

### Frontend Deployment

**Embedded Build** (production):
```bash
cd frontend
pnpm run build  # Output to ../backend/internal/web/dist/
cd ../backend
go build -tags embed -o sub2api ./cmd/server  # Embeds frontend
```

**Separate Build** (development/debugging):
```bash
cd frontend
pnpm run build
# Serve dist/ folder with nginx or other static server
```

**Vite Build Configuration** (`frontend/vite.config.ts`):
- Output: `../backend/internal/web/dist/`
- Base path: `/`
- Index fallback for SPA routing

---

## Code Generation & Tools

### 1. Ent Code Generation

**Trigger**: After editing any file in `backend/ent/schema/`

```bash
cd backend
go generate ./ent
```

**Generates**:
- `ent/*.go`: CRUD operations for all entities
- `ent/migrate/schema.go`: Migration helpers
- `ent/hook/hook.go`: Schema hooks
- `ent/predicate/predicate.go`: Query predicates

**Example Schema Edit**:
```go
// backend/ent/schema/user.go
func (User) Fields() []ent.Field {
    return []ent.Field{
        field.String("email").Unique(),
        field.String("password_hash").Sensitive(),
        // Add new field:
        field.String("phone_number").Optional(),
    }
}
```

### 2. Wire Dependency Injection

**Trigger**: After adding new services or changing DI graph

```bash
cd backend
go generate ./cmd/server
```

**Generates**: `cmd/server/wire_gen.go`

**Configuration**: `cmd/server/wire.go`

**Example: Adding a New Service**:
```go
// 1. Create service with constructor
// internal/service/my_new_service.go
type MyNewService struct {
    repo *repository.MyRepo
}

func NewMyNewService(repo *repository.MyRepo) *MyNewService {
    return &MyNewService{repo: repo}
}

// 2. Add to ProviderSet
// internal/service/wire.go
var ProviderSet = wire.NewSet(
    // ... existing services
    NewMyNewService,
)

// 3. Regenerate
go generate ./cmd/server
```

### 3. Frontend Type Generation

**API Types** (`frontend/src/types/`):
- Manually maintained TypeScript interfaces
- Should match backend Go structs

**Recommended**: Use code generation tool (e.g., `oapi-codegen`) for type safety

### 4. Database Migrations

**Manual Migrations** (`backend/migrations/`):
- Naming: `000_description.sql`
- Example: `016_add_partial_unique_indexes.sql`

**Migration Execution**:
- Automatic on server startup (via Ent `AutoMigrate`)
- Manual: `migrate.New(client.DB(), "migrations").Up(ctx)`

---

## Development Workflows

### Adding a New API Endpoint

**Example**: Add a new admin endpoint to list all groups

1. **Define Handler**:
   ```go
   // backend/internal/handler/admin/group_handler.go
   type GroupHandler struct {
       groupService *service.GroupService
   }

   func (h *GroupHandler) ListGroups(c *gin.Context) {
       groups, err := h.groupService.ListGroups(c.Request.Context())
       if err != nil {
           c.JSON(500, gin.H{"error": err.Error()})
           return
       }
       c.JSON(200, groups)
   }
   ```

2. **Register Route**:
   ```go
   // backend/internal/server/router.go
   func setupAdminRoutes(r *gin.RouterGroup, handlers *handler.Handlers) {
       admin := r.Group("/admin", middleware.AdminAuth())
       {
           admin.GET("/groups", handlers.Admin.Group.ListGroups)
       }
   }
   ```

3. **Implement Service**:
   ```go
   // backend/internal/service/group_service.go
   func (s *GroupService) ListGroups(ctx context.Context) ([]*ent.Group, error) {
       return s.repo.Query().All(ctx)
   }
   ```

4. **Add Frontend API Call**:
   ```typescript
   // frontend/src/api/admin/groups.ts
   export async function listGroups(): Promise<Group[]> {
       const response = await client.get('/api/admin/groups')
       return response.data
   }
   ```

5. **Test**:
   ```bash
   # Unit test
   cd backend
   go test ./internal/handler/admin -v

   # Integration test (if needed)
   go test -tags=integration ./internal/handler/admin -v
   ```

### Adding a New Database Model

**Example**: Add a new `Notification` entity

1. **Create Schema**:
   ```go
   // backend/ent/schema/notification.go
   package schema

   import (
       "entgo.io/ent"
       "entgo.io/ent/schema/edge"
       "entgo.io/ent/schema/field"
   )

   type Notification struct {
       ent.Schema
   }

   func (Notification) Fields() []ent.Field {
       return []ent.Field{
           field.Int("user_id"),
           field.String("message"),
           field.Bool("is_read").Default(false),
           field.Time("created_at").Default(time.Now),
       }
   }

   func (Notification) Edges() []ent.Edge {
       return []ent.Edge{
           edge.From("user", User.Type).
               Ref("notifications").
               Field("user_id").
               Unique().
               Required(),
       }
   }
   ```

2. **Update User Schema** (add reverse edge):
   ```go
   // backend/ent/schema/user.go
   func (User) Edges() []ent.Edge {
       return []ent.Edge{
           // ... existing edges
           edge.To("notifications", Notification.Type),
       }
   }
   ```

3. **Generate Ent Code**:
   ```bash
   cd backend
   go generate ./ent
   ```

4. **Create Repository**:
   ```go
   // backend/internal/repository/notification_repository.go
   type NotificationRepository struct {
       client *ent.Client
   }

   func NewNotificationRepository(client *ent.Client) *NotificationRepository {
       return &NotificationRepository{client: client}
   }
   ```

5. **Create Service**:
   ```go
   // backend/internal/service/notification_service.go
   type NotificationService struct {
       repo *repository.NotificationRepository
   }

   func NewNotificationService(repo *repository.NotificationRepository) *NotificationService {
       return &NotificationService{repo: repo}
   }
   ```

6. **Add to Wire**:
   ```go
   // internal/service/wire.go
   var ProviderSet = wire.NewSet(
       // ...
       NewNotificationService,
   )

   // Regenerate
   go generate ./cmd/server
   ```

7. **Create Migration** (if needed):
   ```sql
   -- backend/migrations/020_add_notifications.sql
   CREATE TABLE notifications (
       id SERIAL PRIMARY KEY,
       user_id INTEGER NOT NULL REFERENCES users(id),
       message TEXT NOT NULL,
       is_read BOOLEAN DEFAULT FALSE,
       created_at TIMESTAMP NOT NULL DEFAULT NOW()
   );
   CREATE INDEX idx_notifications_user_id ON notifications(user_id);
   ```

### Adding OAuth Provider Support

**Example**: Add a new OAuth provider (e.g., GitHub)

1. **Create OAuth Service**:
   ```go
   // backend/internal/service/github_oauth_service.go
   type GitHubOAuthService struct {
       config  *config.Config
       httpClient *http.Client
   }

   func (s *GitHubOAuthService) GetAuthURL() string { /* ... */ }
   func (s *GitHubOAuthService) HandleCallback(code string) (*OAuthToken, error) { /* ... */ }
   func (s *GitHubOAuthService) RefreshToken(refreshToken string) (*OAuthToken, error) { /* ... */ }
   ```

2. **Add to Account Schema**:
   ```go
   // backend/ent/schema/account.go
   // Update Platform enum to include "github"
   field.Enum("platform").Values("claude", "openai", "gemini", "antigravity", "github")
   ```

3. **Create Handler**:
   ```go
   // backend/internal/handler/oauth/github_handler.go
   func (h *GitHubOAuthHandler) Authorize(c *gin.Context) { /* ... */ }
   func (h *GitHubOAuthHandler) Callback(c *gin.Context) { /* ... */ }
   ```

4. **Register Routes**:
   ```go
   // backend/internal/server/router.go
   oauth.GET("/github/authorize", handlers.OAuth.GitHub.Authorize)
   oauth.GET("/github/callback", handlers.OAuth.GitHub.Callback)
   ```

5. **Add Frontend Integration**:
   ```typescript
   // frontend/src/api/oauth.ts
   export function initiateGitHubOAuth() {
       window.location.href = '/api/oauth/github/authorize'
   }
   ```

---

## Important Conventions

### 1. Error Handling

**Structured Errors** (`internal/pkg/errors/errors.go`):
```go
var (
    ErrNotFound          = errors.New("resource not found")
    ErrUnauthorized      = errors.New("unauthorized")
    ErrInvalidInput      = errors.New("invalid input")
    ErrInsufficientQuota = errors.New("insufficient quota")
)

// HTTP status code mapping
func HTTPStatusFromError(err error) int {
    switch {
    case errors.Is(err, ErrNotFound):
        return http.StatusNotFound
    case errors.Is(err, ErrUnauthorized):
        return http.StatusUnauthorized
    default:
        return http.StatusInternalServerError
    }
}
```

**Handler Error Response**:
```go
func (h *Handler) GetUser(c *gin.Context) {
    user, err := h.service.GetUser(c.Request.Context(), userID)
    if err != nil {
        status := errors.HTTPStatusFromError(err)
        c.JSON(status, gin.H{"error": err.Error()})
        return
    }
    c.JSON(200, user)
}
```

### 2. Request Validation

**Bind & Validate** (using Gin's binding):
```go
type CreateUserRequest struct {
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"password" binding:"required,min=8"`
}

func (h *Handler) CreateUser(c *gin.Context) {
    var req CreateUserRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    // Proceed with validated request
}
```

### 3. Logging

**Structured Logging** (using standard `log` package):
```go
log.Printf("[GatewayService] Request: model=%s, user=%d, account=%d", model, userID, accountID)
log.Printf("[ERROR] Failed to refresh token: %v", err)
```

**Best Practices**:
- Prefix with component name: `[ServiceName]`
- Use `[ERROR]`, `[WARN]`, `[INFO]` levels
- Log important events: auth failures, billing errors, API errors

### 4. Configuration

**Config Loading** (`internal/config/config.go`):
```go
// Viper-based config with env override
// Priority: env vars > config.yaml > defaults
```

**Environment Variable Override**:
```bash
# Example: Override database host
DATABASE_HOST=postgres.example.com ./sub2api

# Nested keys use underscore
SECURITY_URL_ALLOWLIST_ENABLED=false ./sub2api
```

**Config File Location**:
1. `./config.yaml` (current directory)
2. `/etc/sub2api/config.yaml` (system-wide)
3. `$DATA_DIR/config.yaml` (data directory)

### 5. API Response Format

**Success Response**:
```json
{
  "id": 123,
  "email": "user@example.com",
  "balance": 10.50
}
```

**Error Response**:
```json
{
  "error": "insufficient quota"
}
```

**List Response**:
```json
{
  "items": [...],
  "total": 100,
  "page": 1,
  "page_size": 20
}
```

### 6. Frontend State Management

**Pinia Store Pattern**:
```typescript
// stores/auth.ts
export const useAuthStore = defineStore('auth', {
    state: () => ({
        user: null as User | null,
        token: localStorage.getItem('token'),
    }),

    getters: {
        isAuthenticated: (state) => !!state.token,
    },

    actions: {
        async login(email: string, password: string) {
            const response = await authAPI.login(email, password)
            this.token = response.token
            this.user = response.user
            localStorage.setItem('token', response.token)
        },

        logout() {
            this.user = null
            this.token = null
            localStorage.removeItem('token')
        },
    },
})
```

### 7. Code Style

**Go**:
- Follow `gofmt` formatting
- Use `golangci-lint` (configured in `.golangci.yml`)
- Error checks: Always check errors, avoid `_` discard
- Receiver names: Use short, consistent names (e.g., `s *Service`, `h *Handler`)

**TypeScript**:
- Follow ESLint rules (configured in `frontend/.eslintrc.cjs`)
- Prefer `interface` over `type` for objects
- Use `async/await` over promises chains
- Component naming: PascalCase for Vue components

### 8. Git Commit Messages

**Format**:
```
type(scope): description

Examples:
feat(gateway): add Antigravity hybrid scheduling mode
fix(billing): prevent race condition in token cache
refactor(admin): extract user bulk operations to service
docs(readme): update Docker deployment instructions
test(service): add integration tests for gateway routing
```

**Types**:
- `feat`: New feature
- `fix`: Bug fix
- `refactor`: Code refactoring
- `test`: Add/update tests
- `docs`: Documentation
- `chore`: Build, CI, dependencies

---

## Security Considerations

### 1. URL Allowlist Validation

**Purpose**: Prevent SSRF attacks by restricting upstream API URLs

**Configuration** (`config.yaml`):
```yaml
security:
  url_allowlist:
    enabled: true  # Set false to disable (development only!)
    upstream_hosts:
      - api.anthropic.com
      - api.openai.com
      - generativelanguage.googleapis.com
    allow_insecure_http: false  # Reject HTTP URLs by default
    allow_private_hosts: false  # Reject private IPs (10.*, 192.168.*, etc.)
```

**Warning**: Disabling URL allowlist or allowing insecure HTTP exposes API keys to MITM attacks.

**Files**: `backend/internal/util/url_validator.go`

### 2. Response Header Filtering

**Purpose**: Prevent leaking sensitive upstream headers to clients

**Configuration** (`config.yaml`):
```yaml
security:
  response_headers:
    enabled: true  # Use default allowlist
    allowed_headers:
      - content-type
      - content-length
      - cache-control
      # ... configurable list
```

**Default Allowlist** (when `enabled: true`):
- Standard headers: `content-type`, `content-length`, `date`
- CORS headers: `access-control-*`
- Explicitly blocks: `set-cookie`, `authorization`, `x-api-key`

**Files**: `backend/internal/util/response_headers.go`

### 3. Content Security Policy (CSP)

**Configuration** (`config.yaml`):
```yaml
security:
  csp:
    enabled: true
    policy: "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline';"
```

**Applied to**: All frontend responses

**Files**: `backend/internal/middleware/security_headers.go`

### 4. CORS Configuration

**Configuration** (`config.yaml`):
```yaml
cors:
  allowed_origins:
    - https://example.com
  allow_credentials: true
  max_age: 86400
```

**Default**: Allows all origins in development, restricts in production

**Files**: `backend/internal/middleware/cors.go`

### 5. JWT Authentication

**Token Expiry**: Configurable in `config.yaml` (`jwt.expire_hour`)

**Refresh Strategy**: Frontend auto-refreshes every 60 seconds if token is valid

**Files**:
- Backend: `backend/internal/middleware/jwt_auth.go`
- Frontend: `frontend/src/stores/auth.ts`

### 6. API Key Security

**Format**: Prefix-based (default: `sk-`) + random suffix

**Storage**: Hashed in database (bcrypt)

**Cache**: Redis cache with 5-minute TTL, invalidated on updates

**Files**:
- Service: `backend/internal/service/api_key_service.go`
- Middleware: `backend/internal/middleware/api_key_auth.go`

### 7. Credential Encryption

**Account Credentials**: Stored as JSONB in PostgreSQL

**Encryption**: At-rest encryption via PostgreSQL transparent data encryption (TDE)

**Recommendation**: Use encrypted PostgreSQL backups

### 8. Rate Limiting

**Per-User Limits** (`config.yaml`):
```yaml
rate_limit:
  requests_per_minute: 60
  tokens_per_minute: 100000
```

**Per-Account Limits**: Configured per account in database

**Implementation**: Token bucket algorithm with Redis

**Files**: `backend/internal/service/rate_limit_service.go`

### 9. Simple Mode Security

**Warning**: Simple mode disables billing, intended for internal use only

**Production Confirmation Required**:
```yaml
run_mode: simple
simple_mode_confirm: true  # Must be explicitly set in production
```

**Environment Variables**:
```bash
RUN_MODE=simple
SIMPLE_MODE_CONFIRM=true
```

---

## Troubleshooting

### Common Issues

#### 1. Frontend Build Fails

**Symptom**: `go build -tags embed` fails with "embed: no matching files found"

**Cause**: Frontend not built, `backend/internal/web/dist/` is empty

**Solution**:
```bash
cd frontend
pnpm install
pnpm run build  # Outputs to ../backend/internal/web/dist/
cd ../backend
go build -tags embed -o sub2api ./cmd/server
```

**Prevention**: Keep `.keep` file in `backend/internal/web/dist/` (tracked by Git)

---

#### 2. Wire Generation Fails

**Symptom**: `go generate ./cmd/server` errors with "inject wireSet: unused"

**Cause**: Service not added to ProviderSet or circular dependency

**Solution**:
1. Ensure service constructor is in `internal/service/wire.go` ProviderSet
2. Check for circular dependencies (A depends on B, B depends on A)
3. Review `wire.go` for typos in function names

```bash
cd backend
go generate ./cmd/server  # Re-run after fixing
```

---

#### 3. Database Migration Issues

**Symptom**: Server crashes on startup with "migration failed"

**Cause**: Manual migration SQL has syntax errors or conflicts

**Solution**:
1. Check migration file syntax (`backend/migrations/*.sql`)
2. Manually apply migration to test database:
   ```bash
   psql -U postgres -d sub2api -f backend/migrations/020_migration.sql
   ```
3. If migration is broken, fix SQL and restart server

**Prevention**: Test migrations on staging database before production

---

#### 4. Redis Connection Errors

**Symptom**: `Failed to connect to Redis: dial tcp: connection refused`

**Cause**: Redis not running or wrong host/port

**Solution**:
```bash
# Check Redis status
redis-cli ping  # Should return PONG

# Start Redis (systemd)
sudo systemctl start redis

# Start Redis (Docker)
docker run -d -p 6379:6379 redis:7-alpine

# Update config.yaml with correct host/port
redis:
  host: localhost
  port: 6379
```

---

#### 5. API Key Authentication Fails

**Symptom**: Gateway requests return `401 Unauthorized`

**Cause**: API key cache invalidation issue or wrong key format

**Solution**:
1. Verify API key format: Must start with configured prefix (default: `sk-`)
2. Clear Redis cache:
   ```bash
   redis-cli FLUSHDB
   ```
3. Regenerate API key from admin panel
4. Check logs for auth middleware errors:
   ```bash
   sudo journalctl -u sub2api -f | grep "APIKeyAuth"
   ```

---

#### 6. Sticky Session Not Working

**Symptom**: Requests switch accounts mid-conversation

**Cause**: Session hash not calculated correctly or Redis cache expired

**Solution**:
1. Check session hash calculation in `gateway_service.go`:
   ```go
   // Session hash based on system prompt + first user message
   sessionHash := sha256.Sum256([]byte(sessionKey))
   ```
2. Verify Redis TTL (default: 1 hour)
3. Check account availability (all accounts may be at quota)

**Debug**:
```bash
# Check Redis session keys
redis-cli KEYS "session:*"

# Check session value
redis-cli GET "session:<hash>"
```

---

#### 7. OAuth Token Refresh Fails

**Symptom**: `Failed to refresh token: invalid_grant`

**Cause**: Refresh token expired or revoked by provider

**Solution**:
1. Re-authorize account from admin panel (OAuth flow)
2. Check account expiry date in database:
   ```sql
   SELECT id, name, platform, expires_at FROM accounts WHERE id = <account_id>;
   ```
3. Verify OAuth credentials in config:
   ```yaml
   oauth:
     client_id: "your_client_id"
     client_secret: "your_client_secret"
   ```

**Logs**:
```bash
sudo journalctl -u sub2api -f | grep "TokenRefresh"
```

---

#### 8. High Memory Usage

**Symptom**: Backend process uses excessive memory

**Possible Causes**:
- UsageLog table too large (not cleaned up)
- Ent query loading too many relations
- Redis cache bloat

**Solution**:
1. **Enable UsageCleanup Service**:
   ```yaml
   usage_cleanup:
     enabled: true
     retention_days: 90  # Keep logs for 90 days
   ```

2. **Optimize Ent Queries**:
   ```go
   // Bad: Loads all relations
   users, _ := client.User.Query().WithAPIKeys().WithUsageLogs().All(ctx)

   // Good: Load only needed fields
   users, _ := client.User.Query().Select(user.FieldEmail).All(ctx)
   ```

3. **Redis Memory Limits**:
   ```bash
   # Set max memory in redis.conf
   maxmemory 256mb
   maxmemory-policy allkeys-lru
   ```

---

#### 9. Frontend Dev Server CORS Issues

**Symptom**: API calls fail in dev mode with CORS errors

**Cause**: Vite dev server proxy misconfiguration

**Solution**: Verify Vite proxy in `frontend/vite.config.ts`:
```typescript
export default defineConfig({
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
```

**Alternative**: Set CORS to allow localhost in `config.yaml`:
```yaml
cors:
  allowed_origins:
    - http://localhost:5173  # Vite dev server
```

---

#### 10. Setup Wizard Loops Infinitely

**Symptom**: Setup wizard shows on every server restart

**Cause**: `.installed` marker file not created or deleted

**Solution**:
1. Check if `.installed` file exists:
   ```bash
   ls -la backend/.installed
   # Or in data directory
   ls -la /opt/sub2api/data/.installed
   ```

2. Manually create marker file:
   ```bash
   touch backend/.installed
   # Or
   touch /opt/sub2api/data/.installed
   ```

3. Ensure write permissions:
   ```bash
   chown sub2api:sub2api /opt/sub2api/data/.installed
   ```

---

### Debug Mode

**Enable Debug Logging**:
```yaml
# config.yaml
server:
  mode: debug  # Change from "release" to "debug"
```

**Or via environment variable**:
```bash
SERVER_MODE=debug ./sub2api
```

**Debug Features**:
- Verbose request/response logging
- Stack traces on errors
- Gin debug output

---

### Useful Debugging Commands

```bash
# Check database connections
psql -U postgres -d sub2api -c "SELECT COUNT(*) FROM users;"

# Check Redis keys
redis-cli KEYS "*"

# Monitor Redis commands
redis-cli MONITOR

# Check backend version
./sub2api --version

# Test configuration parsing
./sub2api --config config.yaml --dry-run

# View systemd service status
sudo systemctl status sub2api

# View full logs (last 100 lines)
sudo journalctl -u sub2api -n 100

# Follow logs in real-time
sudo journalctl -u sub2api -f

# Docker logs
docker logs -f sub2api

# Docker exec into container
docker exec -it sub2api sh
```

---

## Additional Resources

### Documentation

- **README.md**: User-facing setup & deployment guide
- **README_CN.md**: Chinese version of README
- **deploy/config.example.yaml**: Full configuration reference with comments
- **docs/dependency-security.md**: Dependency security guidelines

### External Links

- **Go Documentation**: https://golang.org/doc/
- **Ent Documentation**: https://entgo.io/docs/getting-started
- **Vue 3 Documentation**: https://vuejs.org/guide/
- **Gin Framework**: https://gin-gonic.com/docs/
- **Vite**: https://vitejs.dev/guide/
- **Pinia**: https://pinia.vuejs.org/

### Code References

**Key Files to Understand**:
1. `backend/cmd/server/main.go` - Application entry point
2. `backend/internal/service/gateway_service.go` - Core gateway logic
3. `backend/internal/handler/gateway/gateway_handler.go` - Gateway HTTP handler
4. `backend/ent/schema/user.go` - User schema definition
5. `frontend/src/stores/auth.ts` - Frontend auth state management
6. `frontend/src/api/client.ts` - HTTP client configuration

---

## Summary for AI Assistants

When working on this codebase:

1. **Always read files before modifying** - Understand existing patterns
2. **Follow clean architecture** - Handler → Service → Repository → Database
3. **Regenerate code after schema changes** - Run `go generate` for Ent & Wire
4. **Test before committing** - Run `make test` or specific test targets
5. **Use structured errors** - Follow `internal/pkg/errors` patterns
6. **Respect security boundaries** - Don't disable URL allowlist or bypass auth
7. **Check .gitignore** - Don't commit `config.yaml`, `dist/`, or binaries
8. **Use conventional commits** - Follow `type(scope): description` format
9. **Frontend builds to backend** - `pnpm run build` outputs to `backend/internal/web/dist/`
10. **Wire is the DI framework** - Add services to ProviderSets, regenerate wire_gen.go

**Branch Workflow**:
- Development branch: `claude/claude-md-mkqbfw60pf0sunin-XUESf`
- Always develop on this branch
- Commit with clear messages
- Push when changes are complete

**Contact**:
- GitHub Issues: https://github.com/Wei-Shaw/sub2api/issues
- Repository: https://github.com/Wei-Shaw/sub2api

---

**End of CLAUDE.md**
