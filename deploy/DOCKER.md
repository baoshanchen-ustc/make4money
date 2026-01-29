# Sub2API Docker Image

Sub2API is an AI API Gateway Platform for distributing and managing AI product subscription API quotas.

## Quick Start

```bash
docker run -d \
  --name sub2api \
  -p 8080:8080 \
  -e DATABASE_URL="postgres://user:pass@host:5432/sub2api" \
  -e REDIS_URL="redis://host:6379" \
  weishaw/sub2api:latest
```

## Docker Compose

```yaml
version: '3.8'

services:
  sub2api:
    image: weishaw/sub2api:latest
    ports:
      - "8080:8080"
    environment:
      - DATABASE_URL=postgres://postgres:postgres@db:5432/sub2api?sslmode=disable
      - REDIS_URL=redis://redis:6379
    depends_on:
      - db
      - redis

  db:
    image: postgres:15-alpine
    environment:
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
      - POSTGRES_DB=sub2api
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data

volumes:
  postgres_data:
  redis_data:
```

### Local build tips

- The build override (`deploy/docker-compose.build.yml`) builds the image from this repo and now passes Go module proxy args to avoid regional DNS issues:
  ```yaml
  build:
    context: ..
    dockerfile: Dockerfile
    args:
      GOPROXY: https://proxy.golang.org,direct
      GOSUMDB: sum.golang.org
  ```
- By default the build override no longer mounts `deploy/config.yaml`; the app writes its generated config into the `sub2api_data` volume. Mount a custom config only when you intentionally want to override the generated one.

## Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `DATABASE_URL` | PostgreSQL connection string | Yes | - |
| `REDIS_URL` | Redis connection string | Yes | - |
| `PORT` | Server port | No | `8080` |
| `GIN_MODE` | Gin framework mode (`debug`/`release`) | No | `release` |

## Supported Architectures

- `linux/amd64`
- `linux/arm64`

## Tags

- `latest` - Latest stable release
- `x.y.z` - Specific version
- `x.y` - Latest patch of minor version
- `x` - Latest minor of major version

## Maintenance

### Viewing Configuration

Since the application writes its configuration to a volume, you can view the active configuration by executing a command within the running container:

```bash
docker compose exec sub2api cat /app/data/config.yaml
```

### Retrieving Admin Password

If you did not provide an `ADMIN_PASSWORD` in your environment, the system auto-generates one on the first run. You can retrieve it using:

```bash
docker compose exec sub2api cat /app/data/.initial_admin_password
```

## Links

- [GitHub Repository](https://github.com/weishaw/sub2api)
- [Documentation](https://github.com/weishaw/sub2api#readme)
