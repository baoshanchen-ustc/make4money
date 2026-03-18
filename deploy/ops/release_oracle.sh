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
PRECHECK_SCRIPT="${PRECHECK_SCRIPT:-$SCRIPT_DIR/preflight_oracle.sh}"
BACKUP_SCRIPT="${BACKUP_SCRIPT:-$SCRIPT_DIR/backup_sub2api.sh}"
REMOTE="${REMOTE:-fork}"
BRANCH="${BRANCH:-fix/openai-system-message-lifting}"
SERVICE="${SERVICE:-sub2api}"
COMPOSE_FILE="${COMPOSE_FILE:-$REPO_ROOT/deploy/docker-compose.yml}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:8080/healthz}"
BACKUP_BEFORE_DEPLOY="${BACKUP_BEFORE_DEPLOY:-1}"
DEPLOY_IF_UP_TO_DATE="${DEPLOY_IF_UP_TO_DATE:-0}"
HEALTH_RETRIES="${HEALTH_RETRIES:-30}"
HEALTH_SLEEP_SECONDS="${HEALTH_SLEEP_SECONDS:-2}"
DIRTY_MODE="${DIRTY_MODE:-fail}"
DIRTY_STASH_NAME=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    --stash-dirty)
      DIRTY_MODE="stash"
      ;;
    --fail-on-dirty)
      DIRTY_MODE="fail"
      ;;
    *)
      fail "unknown argument: $1"
      ;;
  esac
  shift
done

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

require_source_build_compose() {
  if grep -Eq '^[[:space:]]+build:' "$COMPOSE_FILE" && \
     grep -Eq '^[[:space:]]+context:[[:space:]]+\.\.' "$COMPOSE_FILE" && \
     grep -Eq '^[[:space:]]+dockerfile:[[:space:]]+deploy/Dockerfile' "$COMPOSE_FILE" && \
     grep -Eq '^[[:space:]]+image:[[:space:]]+sub2api-local:latest' "$COMPOSE_FILE"; then
    log "compose file is pinned to repo source build"
  else
    fail "compose file is not pinned to repo source build; refusing release because official image fallback would bypass local fixes"
  fi
}

wait_for_health() {
  local attempt=1
  while [ "$attempt" -le "$HEALTH_RETRIES" ]; do
    if curl -fsS "$HEALTH_URL" >/dev/null; then
      log "health endpoint reachable: $HEALTH_URL"
      return 0
    fi
    sleep "$HEALTH_SLEEP_SECONDS"
    attempt=$((attempt + 1))
  done
  fail "health endpoint did not become healthy: $HEALTH_URL"
}

log_dirty_stash_notice() {
  if [ -n "$DIRTY_STASH_NAME" ]; then
    log "local changes were saved to git stash: $DIRTY_STASH_NAME"
  fi
}

ensure_repo_ready() {
  if [ -z "$(git status --porcelain)" ]; then
    return 0
  fi

  case "$DIRTY_MODE" in
    fail)
      fail "repo working tree is not clean (rerun with --stash-dirty to save local changes before release)"
      ;;
    stash)
      DIRTY_STASH_NAME="sub2api-release-$(date -u +'%Y%m%dT%H%M%SZ')"
      log "repo working tree is not clean; stashing local changes as $DIRTY_STASH_NAME"
      git stash push -u -m "$DIRTY_STASH_NAME" >/dev/null || fail "failed to stash local changes"
      ;;
    *)
      fail "unsupported DIRTY_MODE: $DIRTY_MODE"
      ;;
  esac
}

require_cmd git
require_cmd curl
require_cmd docker
[ -x "$PRECHECK_SCRIPT" ] || fail "missing precheck script: $PRECHECK_SCRIPT"
[ -x "$BACKUP_SCRIPT" ] || fail "missing backup script: $BACKUP_SCRIPT"

cd "$REPO_ROOT"

current_branch="$(git branch --show-current)"
[ "$current_branch" = "$BRANCH" ] || fail "current branch is $current_branch, expected $BRANCH"
ensure_repo_ready
require_source_build_compose

bash "$PRECHECK_SCRIPT"

log "fetching $REMOTE/$BRANCH"
git fetch "$REMOTE" "$BRANCH"
upstream_ref="refs/remotes/$REMOTE/$BRANCH"
git rev-parse --verify "$upstream_ref" >/dev/null 2>&1 || fail "missing upstream ref: $upstream_ref"

counts="$(git rev-list --left-right --count HEAD...$upstream_ref)"
ahead="$(printf '%s\n' "$counts" | awk '{print $1}')"
behind="$(printf '%s\n' "$counts" | awk '{print $2}')"

if [ "$ahead" -ne 0 ]; then
  fail "local branch has $ahead commit(s) not on $upstream_ref; reconcile before release"
fi

if [ "$behind" -eq 0 ] && [ "$DEPLOY_IF_UP_TO_DATE" != "1" ]; then
  log_dirty_stash_notice
  log "branch already matches $upstream_ref; skipping deploy"
  exit 0
fi

if [ "$BACKUP_BEFORE_DEPLOY" = "1" ]; then
  log "running backup before deploy"
  bash "$BACKUP_SCRIPT"
fi

if [ "$behind" -gt 0 ]; then
  log "fast-forwarding to $upstream_ref"
  git merge --ff-only "$upstream_ref"
fi

log "rebuilding and restarting $SERVICE"
compose_cmd -f "$COMPOSE_FILE" up -d --build --no-deps "$SERVICE"

wait_for_health
running_image="$($SUDO docker inspect -f '{{.Config.Image}}' "$SERVICE" 2>/dev/null || true)"
[ "$running_image" = "sub2api-local:latest" ] || fail "running image is $running_image, expected sub2api-local:latest"
bash "$PRECHECK_SCRIPT"
log_dirty_stash_notice
log "release completed"
