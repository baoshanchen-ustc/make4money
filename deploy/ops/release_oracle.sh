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

require_cmd git
require_cmd curl
require_cmd docker
[ -x "$PRECHECK_SCRIPT" ] || fail "missing precheck script: $PRECHECK_SCRIPT"
[ -x "$BACKUP_SCRIPT" ] || fail "missing backup script: $BACKUP_SCRIPT"

cd "$REPO_ROOT"

current_branch="$(git branch --show-current)"
[ "$current_branch" = "$BRANCH" ] || fail "current branch is $current_branch, expected $BRANCH"
[ -z "$(git status --porcelain)" ] || fail "repo working tree is not clean"

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
bash "$PRECHECK_SCRIPT"
log "release completed"
