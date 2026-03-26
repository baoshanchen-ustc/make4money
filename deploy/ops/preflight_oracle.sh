#!/usr/bin/env bash
set -Eeuo pipefail

log() {
  printf '[%s] %s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$*"
}

warn() {
  log "WARN: $*"
}

fail() {
  log "ERROR: $*"
  exit 1
}

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
DEPLOY_DIR="$REPO_ROOT/deploy"
ENV_FILE="${ENV_FILE:-$DEPLOY_DIR/.env}"
LIVE_CADDY_FILE="${LIVE_CADDY_FILE:-/etc/caddy/Caddyfile}"
REPO_CADDY_FILE="${REPO_CADDY_FILE:-$DEPLOY_DIR/Caddyfile.oracle-a1-free}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:8080/healthz}"
BACKUP_ROOT="${BACKUP_ROOT:-/home/ubuntu/backups/sub2api}"
TIMER_NAME="${TIMER_NAME:-sub2api-backup.timer}"
SUB2API_CONTAINER="${SUB2API_CONTAINER:-sub2api}"

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  SUDO="sudo"
fi

docker_cmd() {
  if [ -n "$SUDO" ]; then
    "$SUDO" docker "$@"
  else
    docker "$@"
  fi
}

compose_cmd() {
  if docker compose version >/dev/null 2>&1; then
    docker compose "$@"
  elif command -v docker-compose >/dev/null 2>&1; then
    docker-compose "$@"
  else
    fail "neither docker compose nor docker-compose is available"
  fi
}

require_source_build_compose() {
  local compose_file="$1"
  if grep -Eq '^[[:space:]]+build:' "$compose_file" && \
     grep -Eq '^[[:space:]]+context:[[:space:]]+\.\.' "$compose_file" && \
     grep -Eq '^[[:space:]]+dockerfile:[[:space:]]+deploy/Dockerfile' "$compose_file" && \
     grep -Eq '^[[:space:]]+image:[[:space:]]+sub2api-local:latest' "$compose_file"; then
    log "OK: compose keeps sub2api on repo source build (sub2api-local:latest)"
  else
    warn "compose is not pinned to repo source build; official image fallback would bypass local compatibility fixes"
    issues=1
  fi
}

issues=0

[ -f "$ENV_FILE" ] || fail "missing env file: $ENV_FILE"
set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

for required_var in POSTGRES_PASSWORD JWT_SECRET TOTP_ENCRYPTION_KEY BIND_HOST SERVER_MODE RUN_MODE TZ; do
  if [ -z "${!required_var:-}" ]; then
    warn "env var not set: $required_var"
    issues=1
  else
    log "OK: env var set: $required_var"
  fi
done

if git -C "$REPO_ROOT" diff --quiet --ignore-submodules -- && \
   git -C "$REPO_ROOT" diff --cached --quiet --ignore-submodules -- && \
   [ -z "$(git -C "$REPO_ROOT" ls-files --others --exclude-standard)" ]; then
  log "OK: repo working tree is clean"
else
  warn "repo working tree is dirty; deploy risk is higher"
  git -C "$REPO_ROOT" status --short
  issues=1
fi

if health_state="$(docker_cmd inspect -f '{{.State.Status}}/{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$SUB2API_CONTAINER" 2>/dev/null)"; then
  log "OK: container state = $health_state"
else
  warn "failed to inspect $SUB2API_CONTAINER"
  issues=1
fi

if running_image="$(docker_cmd inspect -f '{{.Config.Image}}' "$SUB2API_CONTAINER" 2>/dev/null)"; then
  if [ "$running_image" = "sub2api-local:latest" ]; then
    log "OK: running image = $running_image"
  else
    warn "running image is $running_image; expected sub2api-local:latest"
    issues=1
  fi
fi

if curl -fsS "$HEALTH_URL" >/dev/null; then
  log "OK: health endpoint reachable at $HEALTH_URL"
else
  warn "health endpoint failed: $HEALTH_URL"
  issues=1
fi

if compose_cmd -f "$DEPLOY_DIR/docker-compose.yml" config -q >/dev/null 2>&1; then
  log "OK: docker compose config validates"
else
  warn "docker compose config validation failed"
  issues=1
fi

require_source_build_compose "$DEPLOY_DIR/docker-compose.yml"

if [ -f "$REPO_CADDY_FILE" ] && [ -f "$LIVE_CADDY_FILE" ]; then
  if [ -n "$SUDO" ]; then
    if "$SUDO" cmp -s "$REPO_CADDY_FILE" "$LIVE_CADDY_FILE"; then
      log "OK: repo Caddy snapshot matches live config"
    else
      warn "repo Caddy snapshot differs from live config"
      issues=1
    fi
  elif cmp -s "$REPO_CADDY_FILE" "$LIVE_CADDY_FILE"; then
    log "OK: repo Caddy snapshot matches live config"
  else
    warn "repo Caddy snapshot differs from live config"
    issues=1
  fi
else
  warn "missing Caddy snapshot or live Caddy file"
  issues=1
fi

if systemctl is-enabled "$TIMER_NAME" >/dev/null 2>&1; then
  log "OK: $TIMER_NAME is enabled"
else
  warn "$TIMER_NAME is not enabled"
  issues=1
fi

if systemctl is-active "$TIMER_NAME" >/dev/null 2>&1; then
  log "OK: $TIMER_NAME is active"
else
  warn "$TIMER_NAME is not active"
  issues=1
fi

latest_backup="$(find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | sort | tail -1 || true)"
if [ -n "$latest_backup" ]; then
  log "OK: latest backup directory = $latest_backup"
else
  warn "no backup directories found under $BACKUP_ROOT"
  issues=1
fi

if [ "${SECURITY_URL_ALLOWLIST_ENABLED:-}" = "true" ]; then
  log "OK: SECURITY_URL_ALLOWLIST_ENABLED=true"
  if [ -n "${SECURITY_URL_ALLOWLIST_UPSTREAM_HOSTS:-}" ]; then
    log "OK: SECURITY_URL_ALLOWLIST_UPSTREAM_HOSTS is explicitly set"
  else
    warn "SECURITY_URL_ALLOWLIST_UPSTREAM_HOSTS is empty while allowlist is enabled"
    issues=1
  fi
else
  warn "SECURITY_URL_ALLOWLIST_ENABLED is not true"
  issues=1
fi

if [ "${SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS:-}" = "false" ]; then
  log "OK: private hosts are not globally allowed"
else
  warn "SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS is not false"
  issues=1
fi

if [ "$issues" -eq 0 ]; then
  log "preflight passed"
else
  warn "preflight completed with issues"
  exit 1
fi
