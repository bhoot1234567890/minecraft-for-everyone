#!/usr/bin/env bash
set -euo pipefail

monitor_interval="${MONITOR_INTERVAL_SECONDS:-60}"
backup_max_age_hours="${BACKUP_MAX_AGE_HOURS:-30}"
status_dir="/monitoring/status"
alerts_file="/monitoring/alerts.log"

mkdir -p "$status_dir" "$(dirname "$alerts_file")"

dashboard_status="unknown"
velocity_status="unknown"
main_status="unknown"
limbo_status="unknown"
main_backup_status="unknown"
state_backup_status="unknown"

log_alert() {
  printf '[stack-monitor] %s %s\n' "$(date -u --iso-8601=seconds)" "$*" >> "$alerts_file"
}

check_http() {
  local url="$1"
  curl --silent --show-error --fail --max-time 5 "$url" >/dev/null
}

check_tcp() {
  local host="$1" port="$2"
  nc -z -w 5 "$host" "$port" >/dev/null 2>&1
}

latest_backup_age_ok() {
  local directory="$1" latest_file latest_epoch now_epoch max_age_seconds
  latest_file="$(find "$directory" -maxdepth 1 -type f | sort | tail -n 1 || true)"
  if [ -z "$latest_file" ]; then
    return 1
  fi

  latest_epoch="$(stat -c '%Y' "$latest_file")"
  now_epoch="$(date +%s)"
  max_age_seconds=$((backup_max_age_hours * 3600))
  [ $((now_epoch - latest_epoch)) -le "$max_age_seconds" ]
}

set_state() {
  local name="$1" new_value="$2"
  local old_value

  old_value="$(eval "printf '%s' \"\${${name}}\"")"
  if [ "$old_value" != "$new_value" ]; then
    log_alert "${name} changed from ${old_value} to ${new_value}"
    eval "${name}=\"${new_value}\""
  fi
}

write_status_snapshot() {
  local timestamp
  timestamp="$(date -u --iso-8601=seconds)"
  cat > "${status_dir}/latest.json" <<EOF
{
  "timestamp": "${timestamp}",
  "checks": {
    "dashboard": "${dashboard_status}",
    "velocity": "${velocity_status}",
    "main": "${main_status}",
    "limbo": "${limbo_status}",
    "main_backup": "${main_backup_status}",
    "state_backup": "${state_backup_status}"
  }
}
EOF
}

while true; do
  if check_http "http://whitelist-manager:3000/"; then
    set_state dashboard_status ok
  else
    set_state dashboard_status failed
  fi

  if check_tcp "velocity" "25577"; then
    set_state velocity_status ok
  else
    set_state velocity_status failed
  fi

  if check_tcp "main" "25565"; then
    set_state main_status ok
  else
    set_state main_status failed
  fi

  if check_tcp "limbo" "25565"; then
    set_state limbo_status ok
  else
    set_state limbo_status failed
  fi

  if latest_backup_age_ok "/backups/main"; then
    set_state main_backup_status ok
  else
    set_state main_backup_status stale
  fi

  if latest_backup_age_ok "/backups/state"; then
    set_state state_backup_status ok
  else
    set_state state_backup_status stale
  fi

  write_status_snapshot
  sleep "$monitor_interval"
done
