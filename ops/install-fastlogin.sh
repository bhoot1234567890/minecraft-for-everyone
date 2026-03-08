#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
proxy_plugins_dir="${repo_root}/plugins-output"
main_plugins_dir="${repo_root}/main-data/plugins"
proxy_fastlogin_dir="${proxy_plugins_dir}/fastlogin"
main_fastlogin_dir="${main_plugins_dir}/FastLogin"

release_json="$(curl -fsSL https://api.github.com/repos/TuxCoding/FastLogin/releases/latest)"

extract_url() {
  local asset_name="$1"
  RELEASE_JSON="${release_json}" ASSET_NAME="${asset_name}" python3 - <<'PY'
import json
import os
import sys

release = json.loads(os.environ["RELEASE_JSON"])
asset_name = os.environ["ASSET_NAME"]
for asset in release.get("assets", []):
    if asset.get("name") == asset_name:
        print(asset["browser_download_url"])
        sys.exit(0)
raise SystemExit(f"missing asset: {asset_name}")
PY
}

velocity_url="$(extract_url FastLoginVelocity.jar)"
bukkit_url="$(extract_url FastLoginBukkit.jar)"

mkdir -p "${proxy_plugins_dir}" "${main_plugins_dir}" "${proxy_fastlogin_dir}" "${main_fastlogin_dir}"

curl -fsSL "${velocity_url}" -o "${proxy_plugins_dir}/FastLoginVelocity.jar"
curl -fsSL "${bukkit_url}" -o "${main_plugins_dir}/FastLoginBukkit.jar"

fastlogin_db_password="${FASTLOGIN_DB_PASSWORD:-}"
if [[ -z "${fastlogin_db_password}" && -f "${repo_root}/.env" ]]; then
  fastlogin_db_password="$(grep -E '^FASTLOGIN_DB_PASSWORD=' "${repo_root}/.env" | tail -n 1 | cut -d= -f2- || true)"
fi

if [[ -z "${fastlogin_db_password}" ]]; then
  echo "FASTLOGIN_DB_PASSWORD is required in the environment or .env" >&2
  exit 1
fi

cat > "${proxy_fastlogin_dir}/config.yml" <<EOF
autoRegister: true
secondAttemptCracked: true
premiumUuid: false
autoLogin: true
autoLoginFloodgate: false
allowFloodgateNameConflict: false
driver: 'mariadb'
host: 'fastlogin-db'
port: 3306
database: 'fastlogin'
username: 'fastlogin'
password: '${fastlogin_db_password}'
EOF

cat > "${main_fastlogin_dir}/config.yml" <<'EOF'
premiumUuid: false
autoLoginFloodgate: false
allowFloodgateNameConflict: false
EOF

echo "Installed FastLoginVelocity.jar to ${proxy_plugins_dir}"
echo "Installed FastLoginBukkit.jar to ${main_plugins_dir}"
echo "Wrote proxy config to ${proxy_fastlogin_dir}/config.yml"
echo "Wrote backend config to ${main_fastlogin_dir}/config.yml"
