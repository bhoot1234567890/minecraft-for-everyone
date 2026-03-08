#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
proxy_id_file="${repo_root}/plugins-output/fastlogin/proxyId.txt"
allowed_proxies_file="${repo_root}/main-data/plugins/FastLogin/allowed-proxies.txt"

if [[ ! -f "${proxy_id_file}" ]]; then
  echo "FastLogin proxyId.txt not found at ${proxy_id_file}" >&2
  exit 1
fi

mkdir -p "$(dirname "${allowed_proxies_file}")"
cp "${proxy_id_file}" "${allowed_proxies_file}"
echo "Synced FastLogin proxy id to ${allowed_proxies_file}"
