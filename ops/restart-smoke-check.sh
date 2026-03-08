#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

docker compose -f docker-compose.proxy.yml ps

printf '\n--- dashboard ---\n'
curl --silent --show-error --fail http://127.0.0.1:3000/ >/dev/null
echo "dashboard ok"

printf '\n--- latest backups ---\n'
for backup_dir in backups/main backups/state; do
  if find "$backup_dir" -maxdepth 1 -type f >/dev/null 2>&1 && [ -n "$(find "$backup_dir" -maxdepth 1 -type f | head -n 1)" ]; then
    latest_file="$(find "$backup_dir" -maxdepth 1 -type f | sort | tail -n 1)"
    printf '%s %s\n' "$backup_dir" "$latest_file"
  else
    printf '%s no backups yet\n' "$backup_dir"
  fi
done

printf '\n--- monitor snapshot ---\n'
if [ -f monitoring/status/latest.json ]; then
  cat monitoring/status/latest.json
  printf '\n'
else
  echo "monitor snapshot not created yet"
fi
