#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
plugins_dir="${repo_root}/plugins-output"
config_dir="${plugins_dir}/veloauth"
version="${VELOAUTH_VERSION:-1.1.0}"
download_url="${VELOAUTH_URL:-https://cdn.modrinth.com/data/awl91tjn/versions/ybWVklIK/veloauth-1.1.0.jar}"
jar_path="${plugins_dir}/veloauth-${version}.jar"

mkdir -p "${plugins_dir}" "${config_dir}"
rm -f "${plugins_dir}"/veloauth-*.jar

echo "Downloading VeloAuth ${version}..."
curl -fsSL "${download_url}" -o "${jar_path}"

cat > "${config_dir}/config.yml" <<'EOF'
language: en
debug-enabled: false

database:
  storage-type: H2
  hostname: localhost
  port: 3306
  database: veloauth
  user: veloauth
  password: ""
  connection-pool-size: 20
  max-lifetime-millis: 1800000
  connection-url: ""
  connection-parameters: ""
  postgresql:
    ssl-enabled: false
    ssl-mode: "prefer"
    ssl-cert: ""
    ssl-key: ""
    ssl-root-cert: ""
    ssl-password: ""

cache:
  ttl-minutes: 60
  max-size: 10000
  cleanup-interval-minutes: 5
  session-timeout-minutes: 1440
  premium-ttl-hours: 24
  premium-refresh-threshold: 0.8

auth-server:
  server-name: limbo
  timeout-seconds: 300

connection:
  timeout-seconds: 20

security:
  bcrypt-cost: 10
  bruteforce-max-attempts: 5
  bruteforce-timeout-minutes: 10
  ip-limit-registrations: 3
  min-password-length: 6
  max-password-length: 72

premium:
  check-enabled: true
  online-mode-need-auth: false
  resolver:
    mojang-enabled: true
    ashcon-enabled: true
    wpme-enabled: false
    request-timeout-ms: 2000
    hit-ttl-minutes: 10
    miss-ttl-minutes: 3
    case-sensitive: true

alerts:
  enabled: false
  failure-rate-threshold: 0.5
  min-requests-for-alert: 10
  check-interval-minutes: 5
  alert-cooldown-minutes: 30
  discord:
    enabled: false
    webhook-url: ""
EOF

echo "Installed ${jar_path}"
echo "Configured ${config_dir}/config.yml"
