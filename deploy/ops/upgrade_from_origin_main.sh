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
REPO_ROOT="${REPO_ROOT:-$(cd -- "$SCRIPT_DIR/../.." && pwd)}"
COMPOSE_FILE="${COMPOSE_FILE:-$REPO_ROOT/deploy/docker-compose.yml}"
SERVICE="${SERVICE:-sub2api}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:8080/health}"
HEALTH_RETRIES="${HEALTH_RETRIES:-30}"
HEALTH_SLEEP_SECONDS="${HEALTH_SLEEP_SECONDS:-2}"
TARGET_INPUT="${1:-main}"

START_COMMIT=""
TARGET_COMMIT=""
TARGET_LABEL=""
ROLLBACK_TAG=""
ROLLBACK_REQUIRED=0
DEPLOY_STARTED=0

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  SUDO="sudo"
fi

compose_cmd() {
  if docker compose version >/dev/null 2>&1; then
    if [ -n "$SUDO" ]; then
      "$SUDO" docker compose "$@"
    else
      docker compose "$@"
    fi
  elif command -v docker-compose >/dev/null 2>&1; then
    if [ -n "$SUDO" ]; then
      "$SUDO" docker-compose "$@"
    else
      docker-compose "$@"
    fi
  else
    fail "neither docker compose nor docker-compose is available"
  fi
}

resolve_version_conflict() {
  local conflict_file ours theirs resolved suffix
  conflict_file="backend/cmd/server/VERSION"

  git -C "$REPO_ROOT" diff --name-only --diff-filter=U | grep -qx "$conflict_file" || return 1
  ours="$(git -C "$REPO_ROOT" show ":2:$conflict_file" 2>/dev/null || true)"
  theirs="$(git -C "$REPO_ROOT" show ":3:$conflict_file" 2>/dev/null || true)"
  [ -n "$ours" ] || return 1
  [ -n "$theirs" ] || return 1

  resolved="$theirs"
  if [[ "$ours" == *-* ]]; then
    suffix="${ours#*-}"
    resolved="${theirs}-${suffix}"
  fi

  printf '%s\n' "$resolved" > "$REPO_ROOT/$conflict_file"
  git -C "$REPO_ROOT" add "$conflict_file"
  log "resolved VERSION conflict: ours=$ours theirs=$theirs final=$resolved"
}

update_build_metadata() {
  local version short_sha build_date
  version="$(tr -d '\r\n' < "$REPO_ROOT/backend/cmd/server/VERSION")"
  short_sha="$(git -C "$REPO_ROOT" rev-parse --short=12 HEAD)"
  build_date="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"

  if grep -q '^APP_VERSION=' "$REPO_ROOT/deploy/.env"; then
    sed -i "s/^APP_VERSION=.*/APP_VERSION=${version}/" "$REPO_ROOT/deploy/.env"
  else
    printf '\nAPP_VERSION=%s\n' "$version" >> "$REPO_ROOT/deploy/.env"
  fi

  if grep -q '^APP_COMMIT=' "$REPO_ROOT/deploy/.env"; then
    sed -i "s/^APP_COMMIT=.*/APP_COMMIT=${short_sha}/" "$REPO_ROOT/deploy/.env"
  else
    printf 'APP_COMMIT=%s\n' "$short_sha" >> "$REPO_ROOT/deploy/.env"
  fi

  if grep -q '^APP_DATE=' "$REPO_ROOT/deploy/.env"; then
    sed -i "s/^APP_DATE=.*/APP_DATE=${build_date}/" "$REPO_ROOT/deploy/.env"
  else
    printf 'APP_DATE=%s\n' "$build_date" >> "$REPO_ROOT/deploy/.env"
  fi
}

wait_for_health() {
  local attempt=1
  while [ "$attempt" -le "$HEALTH_RETRIES" ]; do
    if curl -fsS "$HEALTH_URL" >/dev/null 2>&1; then
      log "health ok: $HEALTH_URL"
      return 0
    fi
    sleep "$HEALTH_SLEEP_SECONDS"
    attempt=$((attempt + 1))
  done
  return 1
}

rollback() {
  set +e
  log "rolling back to $START_COMMIT"
  git -C "$REPO_ROOT" merge --abort >/dev/null 2>&1 || true
  git -C "$REPO_ROOT" reset --hard "$START_COMMIT" >/dev/null 2>&1 || true
  update_build_metadata
  if [ "$DEPLOY_STARTED" -eq 1 ]; then
    compose_cmd -f "$COMPOSE_FILE" up -d --build
    wait_for_health >/dev/null 2>&1 || true
  fi
  set -e
}

on_error() {
  local exit_code="$1"
  local line="$2"
  log "command failed on line $line"
  if [ "$ROLLBACK_REQUIRED" -eq 1 ]; then
    rollback
  fi
  exit "$exit_code"
}

trap 'on_error $? $LINENO' ERR

usage() {
  cat <<USAGE
Usage: $(basename "$0") [main|<commit>]

Examples:
  $(basename "$0")
  $(basename "$0") 0236b97d496e8d5a4bd56b73ad2fa29aa56fba10
USAGE
}

if [ "$TARGET_INPUT" = "-h" ] || [ "$TARGET_INPUT" = "--help" ]; then
  usage
  exit 0
fi

require_cmd git
require_cmd curl
require_cmd docker

cd "$REPO_ROOT"
START_COMMIT="$(git rev-parse HEAD)"
ROLLBACK_TAG="pre-upgrade-$(date -u +'%Y%m%dT%H%M%SZ')"
git tag -f "$ROLLBACK_TAG" "$START_COMMIT" >/dev/null

if [ -n "$(git status --porcelain)" ]; then
  fail "repo working tree is not clean"
fi

log "fetching origin/main"
git fetch origin --prune
git fetch origin main

if [ "$TARGET_INPUT" = "main" ]; then
  TARGET_COMMIT="$(git rev-parse FETCH_HEAD)"
  TARGET_LABEL="origin/main@$TARGET_COMMIT"
else
  git cat-file -e "$TARGET_INPUT^{commit}" 2>/dev/null || fail "unknown commit: $TARGET_INPUT"
  TARGET_COMMIT="$TARGET_INPUT"
  TARGET_LABEL="$TARGET_INPUT"
fi

if [ "$TARGET_COMMIT" = "$START_COMMIT" ]; then
  log "already at target commit: $TARGET_LABEL"
  exit 0
fi

ROLLBACK_REQUIRED=1
log "merging $TARGET_LABEL into $(git branch --show-current)"
if ! git merge --no-ff --no-edit "$TARGET_COMMIT"; then
  resolve_version_conflict || fail "merge conflict requires manual resolution"
  if [ -n "$(git diff --name-only --diff-filter=U)" ]; then
    fail "merge conflict requires manual resolution"
  fi
  git commit --no-edit
fi

update_build_metadata
DEPLOY_STARTED=1
log "rebuilding $SERVICE"
compose_cmd -f "$COMPOSE_FILE" up -d --build
wait_for_health || fail "health check failed after deploy"

container_image="$($SUDO docker inspect -f '{{.Config.Image}}' "$SERVICE" 2>/dev/null || true)"
[ "$container_image" = "sub2api-local:latest" ] || fail "unexpected running image: $container_image"

log "version: $($SUDO docker exec "$SERVICE" /app/sub2api --version 2>/dev/null | tail -n 1)"
log "upgrade completed"
