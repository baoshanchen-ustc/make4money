#!/usr/bin/env bash
set -Eeuo pipefail

log() {
  printf '[%s] %s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$*"
}

fail() {
  log "ERROR: $*"
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing command: $1"
}

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
DEPLOY_DIR="$REPO_ROOT/deploy"
ENV_FILE="${ENV_FILE:-$DEPLOY_DIR/.env}"
BACKUP_ROOT="${BACKUP_ROOT:-/home/ubuntu/backups/sub2api}"
SUB2API_CONTAINER="${SUB2API_CONTAINER:-sub2api}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-sub2api-postgres}"
REDIS_CONTAINER="${REDIS_CONTAINER:-sub2api-redis}"
VERIFY_SCRIPT="${VERIFY_SCRIPT:-$SCRIPT_DIR/verify_backup_sub2api.sh}"

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

copy_file() {
  local src="$1"
  local dst="$2"
  if [ -n "$SUDO" ]; then
    "$SUDO" cp "$src" "$dst"
    "$SUDO" chown "$(id -u):$(id -g)" "$dst"
    chmod 600 "$dst"
  else
    cp "$src" "$dst"
    chmod 600 "$dst"
  fi
}

volume_source() {
  local container="$1"
  local destination="$2"
  docker_cmd inspect -f "{{range .Mounts}}{{if eq .Destination \"$destination\"}}{{.Source}}{{end}}{{end}}" "$container"
}

require_cmd git
require_cmd docker
[ -f "$ENV_FILE" ] || fail "missing env file: $ENV_FILE"

set -a
# shellcheck disable=SC1090
. "$ENV_FILE"
set +a

: "${POSTGRES_USER:=sub2api}"
: "${POSTGRES_DB:=sub2api}"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
backup_dir="$BACKUP_ROOT/$timestamp"
sub2api_data_src="$(volume_source "$SUB2API_CONTAINER" "/app/data")"
redis_data_src="$(volume_source "$REDIS_CONTAINER" "/data")"

[ -n "$sub2api_data_src" ] || fail "could not resolve /app/data mount for $SUB2API_CONTAINER"
[ -n "$redis_data_src" ] || fail "could not resolve /data mount for $REDIS_CONTAINER"

for container in "$SUB2API_CONTAINER" "$POSTGRES_CONTAINER" "$REDIS_CONTAINER"; do
  status="$(docker_cmd inspect -f '{{.State.Status}}' "$container" 2>/dev/null || true)"
  [ "$status" = "running" ] || fail "container not running: $container"
done

install -d -m 700 "$BACKUP_ROOT"
install -d -m 700 "$backup_dir"

log "creating backup at $backup_dir"
if [ -n "$SUDO" ]; then
  docker_cmd exec "$POSTGRES_CONTAINER" pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc | tee "$backup_dir/postgres.dump" >/dev/null
else
  docker_cmd exec "$POSTGRES_CONTAINER" pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc > "$backup_dir/postgres.dump"
fi

if [ -n "$SUDO" ]; then
  "$SUDO" tar -C "$sub2api_data_src" -czf "$backup_dir/sub2api_data.tgz" .
  "$SUDO" tar -C "$redis_data_src" -czf "$backup_dir/redis_data.tgz" .
else
  tar -C "$sub2api_data_src" -czf "$backup_dir/sub2api_data.tgz" .
  tar -C "$redis_data_src" -czf "$backup_dir/redis_data.tgz" .
fi

copy_file "$ENV_FILE" "$backup_dir/.env"
copy_file /etc/caddy/Caddyfile "$backup_dir/Caddyfile.oracle-live"

cat > "$backup_dir/manifest.txt" <<EOF
created_at_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)
host=$(hostname)
repo_root=$REPO_ROOT
repo_commit=$(git -C "$REPO_ROOT" rev-parse HEAD 2>/dev/null || echo unknown)
repo_branch=$(git -C "$REPO_ROOT" rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)
sub2api_status=$(docker_cmd inspect -f '{{.State.Status}}/{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$SUB2API_CONTAINER")
postgres_status=$(docker_cmd inspect -f '{{.State.Status}}/{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$POSTGRES_CONTAINER")
redis_status=$(docker_cmd inspect -f '{{.State.Status}}/{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$REDIS_CONTAINER")
sub2api_data_source=$sub2api_data_src
redis_data_source=$redis_data_src
EOF

if command -v sha256sum >/dev/null 2>&1; then
  (
    cd "$backup_dir"
    sha256sum postgres.dump sub2api_data.tgz redis_data.tgz .env Caddyfile.oracle-live manifest.txt > SHA256SUMS
  )
fi

if [ -x "$VERIFY_SCRIPT" ]; then
  "$VERIFY_SCRIPT" "$backup_dir"
fi

log "backup completed: $backup_dir"
