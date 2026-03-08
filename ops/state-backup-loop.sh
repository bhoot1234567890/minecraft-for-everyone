#!/usr/bin/env bash
set -euo pipefail

backup_root="/backups/state"
initial_delay="${STATE_INITIAL_DELAY:-2m}"
backup_interval="${STATE_BACKUP_INTERVAL:-6h}"
prune_days="${STATE_PRUNE_DAYS:-7}"

mkdir -p "$backup_root"

log() {
  printf '[state-backup] %s %s\n' "$(date -u --iso-8601=seconds)" "$*"
}

perform_backup() {
  local timestamp archive staging
  timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
  archive="${backup_root}/state-${timestamp}.tar.gz"
  staging="$(mktemp -d "${backup_root}/.staging-${timestamp}-XXXXXX")"

  mkdir -p "${staging}/snapshot"
  cp -a /sources/proxy-data "${staging}/snapshot/proxy-data"
  cp -a /sources/limbo-data "${staging}/snapshot/limbo-data"
  cp -a /sources/shared-whitelist "${staging}/snapshot/shared-whitelist"

  if [ -d /sources/plugins-output ]; then
    cp -a /sources/plugins-output "${staging}/snapshot/plugins-output"
  fi

  tar -C "${staging}/snapshot" -czf "$archive" .
  rm -rf "$staging"

  find "$backup_root" -maxdepth 1 -type f -name 'state-*.tar.gz' -mtime "+${prune_days}" -delete
  log "created ${archive}"
}

sleep "$initial_delay"

while true; do
  perform_backup
  sleep "$backup_interval"
done
