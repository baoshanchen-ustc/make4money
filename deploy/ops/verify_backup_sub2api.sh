#!/usr/bin/env bash
set -Eeuo pipefail

log() {
  printf '[%s] %s\n' "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$*"
}

fail() {
  log "ERROR: $*"
  exit 1
}

BACKUP_ROOT="${BACKUP_ROOT:-/home/ubuntu/backups/sub2api}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-sub2api-postgres}"

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

pick_backup_dir() {
  local selector="${1:-latest}"
  if [ "$selector" = "latest" ]; then
    find "$BACKUP_ROOT" -mindepth 1 -maxdepth 1 -type d | sort | tail -1
  else
    printf '%s\n' "$selector"
  fi
}

backup_dir="$(pick_backup_dir "${1:-latest}")"
[ -n "$backup_dir" ] || fail "no backup directory found"
[ -d "$backup_dir" ] || fail "backup directory does not exist: $backup_dir"

for required_file in postgres.dump sub2api_data.tgz redis_data.tgz .env Caddyfile.oracle-live manifest.txt; do
  if [ ! -s "$backup_dir/$required_file" ]; then
    fail "backup file missing or empty: $backup_dir/$required_file"
  fi
  log "OK: found $required_file"
done

if [ -f "$backup_dir/SHA256SUMS" ]; then
  (
    cd "$backup_dir"
    sha256sum -c SHA256SUMS >/dev/null
  )
  log "OK: SHA256SUMS verified"
else
  log "INFO: SHA256SUMS not present; skipping checksum verification"
fi

if docker_cmd inspect "$POSTGRES_CONTAINER" >/dev/null 2>&1; then
  tmp_name="/tmp/sub2api-backup-verify-$$.dump"
  cat "$backup_dir/postgres.dump" | docker_cmd exec -i "$POSTGRES_CONTAINER" sh -lc "cat > '$tmp_name' && pg_restore --list '$tmp_name' >/dev/null && rm -f '$tmp_name'"
  log "OK: postgres dump can be listed by pg_restore"
else
  log "INFO: postgres container not available; skipped pg_restore validation"
fi

sub2api_tgz_size="$(du -h "$backup_dir/sub2api_data.tgz" | awk '{print $1}')"
redis_tgz_size="$(du -h "$backup_dir/redis_data.tgz" | awk '{print $1}')"
pg_dump_size="$(du -h "$backup_dir/postgres.dump" | awk '{print $1}')"

cat <<EOF
Backup verification summary
  directory: $backup_dir
  postgres.dump: $pg_dump_size
  sub2api_data.tgz: $sub2api_tgz_size
  redis_data.tgz: $redis_tgz_size
EOF

log "backup verification passed"
