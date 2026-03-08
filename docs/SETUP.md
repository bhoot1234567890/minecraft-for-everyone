# Setup Guide

This guide reflects the **current recommended deployment**: `docker-compose.proxy.yml`.

## Prerequisites

- Linux host
- Docker + Docker Compose plugin (`docker compose`)
- Ports available:
  - `25565/tcp`
  - `19132/udp`
  - `127.0.0.1:3000/tcp` for the dashboard

## Important directories

```text
main-data/                  # Paper data
limbo-data/                 # PicoLimbo data
proxy-data/                 # Velocity config and runtime data
plugins-output/             # Proxy plugin jars + plugin data
shared-whitelist/           # Shared whitelist JSON files
optional-auth-guard/        # Paper plugin source for optional cracked passwords
whitelist-manager/          # Dashboard source
velocity-whitelist-router/  # Proxy plugin source
```

## Current stack

- Velocity proxy
- Geyser + Floodgate on Velocity
- FastLogin on Velocity and Paper
- Main Paper `1.21.11`
- OptionalAuthGuard on Paper
- PicoLimbo `v1.11.0+mc1.21.11`
- MariaDB (`fastlogin-db`) for FastLogin proxy state
- Whitelist Manager dashboard on `127.0.0.1:3000`
- `backup-main` sidecar for world backups
- `state-backup` sidecar for proxy/whitelist state backups
- `stack-monitor` for local health and backup freshness snapshots

## 0. Create local secrets

Copy the example env file and set strong secrets:

```bash
cp .env.example .env
```

Required values in `.env`:

- `ADMIN_USERNAME`
- `ADMIN_PASSWORD`
- `PROXY_API_TOKEN`
- `RCON_PASSWORD`
- `FASTLOGIN_DB_PASSWORD`

Optional values:

- `SESSION_COOKIE_SECURE=false` for local HTTP-only admin access
- `CORS_ALLOW_ORIGINS=http://localhost:3000,http://127.0.0.1:3000`
- `BACKUP_INITIAL_DELAY=30s`
- `BACKUP_INTERVAL=6h`
- `PRUNE_BACKUPS_DAYS=7`
- `STATE_INITIAL_DELAY=30s`
- `STATE_BACKUP_INTERVAL=6h`
- `STATE_PRUNE_DAYS=7`
- `MONITOR_INTERVAL_SECONDS=60`
- `BACKUP_MAX_AGE_HOURS=30`
- `TZ=UTC`

## 1. Install plugins and start the stack

Install FastLogin first:

```bash
./ops/install-fastlogin.sh
```

Build and start the stack:

```bash
docker compose -f docker-compose.proxy.yml build
docker compose -f docker-compose.proxy.yml up -d
```

After Velocity starts once, sync the generated FastLogin proxy ID to Paper and recreate the main server:

```bash
./ops/sync-fastlogin-proxy-id.sh
docker compose -f docker-compose.proxy.yml up -d --force-recreate main
```

If you update the custom proxy plugin:

```bash
docker compose -f docker-compose.proxy.yml build whitelist-router-builder
docker compose -f docker-compose.proxy.yml run --rm whitelist-router-builder
docker compose -f docker-compose.proxy.yml up -d --force-recreate velocity
```

If you update OptionalAuthGuard:

```bash
docker compose -f docker-compose.proxy.yml build optional-auth-guard-builder
docker compose -f docker-compose.proxy.yml run --rm optional-auth-guard-builder
docker compose -f docker-compose.proxy.yml up -d --force-recreate main
```

If you update the dashboard:

```bash
docker compose -f docker-compose.proxy.yml build whitelist-manager
docker compose -f docker-compose.proxy.yml up -d --force-recreate whitelist-manager
```

## 2. Required configs

### Velocity

`proxy-data/velocity.toml`

Important settings:

```toml
bind = "0.0.0.0:25577"
online-mode = false
force-key-authentication = false
player-info-forwarding-mode = "MODERN"
forwarding-secret-file = "forwarding.secret"

[servers]
main = "main:25565"
limbo = "limbo:25565"
try = ["main"]
```

Notes:

- `HYBRID_AUTH_MODE` should stay `false` in the current deployment
- FastLogin handles premium detection

### Main Paper

Velocity forwarding must be enabled and the secret must match `proxy-data/forwarding.secret`.
The backend remains `online-mode=false` in `server.properties`, but:

- `main-data/config/paper-global.yml` must keep `proxies.velocity.online-mode: false`
- `main-data/server.properties` should keep `prevent-proxy-connections=true`
- `main-data/server.properties` should keep `enforce-secure-profile=false`

### PicoLimbo

`limbo-data/server.toml` must use Velocity modern forwarding:

```toml
[forwarding]
method = "MODERN"
secret = "same-secret-as-velocity"
```

### Geyser

`proxy-data/config/Geyser-Velocity/config.yml`

Important settings:

```yaml
bedrock:
  address: 0.0.0.0
  port: 19132

remote:
  address: auto
  port: 25577
  auth-type: floodgate
```

### FastLogin

Generated files:

- `plugins-output/fastlogin/config.yml`
- `plugins-output/fastlogin/proxyId.txt`
- `main-data/plugins/FastLogin/config.yml`
- `main-data/plugins/FastLogin/allowed-proxies.txt`

Keep the sync step in place whenever `proxyId.txt` changes.

## 3. Shared whitelist file format

### `shared-whitelist/whitelist.json`

```json
[
  {
    "uuid": "00000000-0000-0000-0009-01f38476db79",
    "name": ".WinterMist88971"
  }
]
```

### `shared-whitelist/pending_bedrock.json`

```json
{
  "entries": []
}
```

## 4. File permissions

This is critical.

Velocity must be able to write:

- `shared-whitelist/whitelist.json`
- `shared-whitelist/pending_bedrock.json`

Paper must be able to write:

- `main-data/plugins/`
- `main-data/plugins/.paper-remapped/`

If Paper cannot write its plugin directory, startup fails while remapping plugins.
Typical symptoms:

- player reaches Main, but dashboard does not update
- `AccessDeniedException` for `/whitelist/whitelist.json`
- `AccessDeniedException` for `/whitelist/pending_bedrock.json`
- `AccessDeniedException: /data/plugins/.paper-remapped/FastLoginBukkit.jar`
- `AccessDeniedException: /data/plugins/.paper-remapped/OptionalAuthGuard.jar`

The current working ownership for Paper is UID/GID `1000:1000`:

```bash
sudo chown -R 1000:1000 main-data/plugins
sudo find main-data/plugins -type d -exec chmod 775 {} +
sudo find main-data/plugins -type f -exec chmod 664 {} +
```

For shared whitelist files, align ownership with the Velocity container user or use explicit writable permissions.

## 5. Dashboard access

Open:

```text
http://localhost:3000/admin
```

Credentials come from `.env`:

- `ADMIN_USERNAME`
- `ADMIN_PASSWORD`

The dashboard is intentionally bound to localhost only in the recommended compose file.

Public browser routes on the same service:

- `/` -> public landing page
- `/play` -> public how-to-play page
- `/admin` -> admin dashboard

## 6. Backups and monitoring

### Backup outputs

- `backups/main/` receives RCON-coordinated world backups from `backup-main`
- `backups/state/` receives tar snapshots of proxy, limbo, whitelist, and plugin state from `state-backup`

### Monitoring outputs

- `monitoring/status/latest.json` contains the latest local health snapshot
- `monitoring/alerts.log` records state changes and stale-backup alerts

Useful commands:

```bash
docker compose -f docker-compose.proxy.yml logs -f backup-main state-backup stack-monitor
./ops/restart-smoke-check.sh
```

## 7. Whitelist mode and Open Mode

The current deployment is whitelist-based.

- approved players go to `main`
- unapproved players go to `limbo`
- deactivated players stay blocked even if Open Mode is later re-enabled

Open Mode still exists, but it is disabled by default and is no longer the normal way players are admitted.

## 8. Login flow

### Premium Java players

- connect normally with the official Minecraft launcher
- must already be whitelisted
- FastLogin validates the session with Mojang/sessionserver
- if valid, they go straight to `main`

### Cracked / offline Java players

- connect normally to the same Java address
- must already be whitelisted
- if no password is set, they are allowed in and warned every join that the name can be impersonated
- they can enable protection with `/register <password> <confirm>`
- if a password is set, they must use `/login <password>` before moving, chatting, or interacting
- they can rotate it with `/changepassword <oldPassword> <newPassword>`

### Bedrock players

- connect through Geyser/Floodgate on UDP `19132`
- continue using Microsoft/Xbox authentication
- must be approved/whitelisted through the Bedrock flow
- do not need the Java password flow

### Unapproved players

- are routed to PicoLimbo instead of `main`
- remain there until approved or otherwise granted access

## 9. Deactivation behavior

Deactivate from the dashboard means:

- dashboard row becomes inactive
- proxy removes effective whitelist access
- proxy stores the player in `blocked_players.json`
- Open Mode will not re-add them until reactivated

Proxy blocked-player state is stored in:

- host: `plugins-output/whitelist-router/blocked_players.json`
- container: `/server/plugins/whitelist-router/blocked_players.json`

## 10. Verification

### Check running containers

```bash
docker compose -f docker-compose.proxy.yml ps
```

### Check container health

```bash
docker compose -f docker-compose.proxy.yml ps
docker inspect --format='{{if .State.Health}}{{.State.Health.Status}}{{else}}no-healthcheck{{end}}' whitelist-manager
```

### Confirm restricted ports

```bash
ss -lntup | grep -E '(:3000|:8080|:25566|:25567)'
```

Expected:

- `127.0.0.1:3000` is present
- `:8080`, `:25566`, and `:25567` are absent from host listeners
- `whitelist-manager` reports `healthy`

### Check dashboard proxy view

```bash
curl -X POST http://localhost:3000/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"your_username","password":"your_password"}'
```

### Confirm FastLogin and OptionalAuthGuard startup

```bash
docker logs --tail=120 velocity-proxy
docker logs --tail=120 minecraft-main
```

Expected:

- Velocity shows `Loaded plugin fastlogin`
- Main shows `Initialized 2 plugins`
- Main shows `OptionalAuthGuard` and `FastLogin` enabling successfully
- Main reaches `Done (...)! For help, type "help"`

### Confirm backups and monitoring

```bash
ls -1 backups/main backups/state
cat monitoring/status/latest.json
```

Expected:

- at least one recent archive exists in both backup directories
- the monitoring snapshot shows `ok` for dashboard, velocity, main, limbo, main backup, and state backup

### Run the smoke check

```bash
./ops/restart-smoke-check.sh
```

## 11. DDoS and edge protection

For a public deployment, add an upstream protection layer such as TCPShield before sharing the server widely.

Recommended rollout:

1. keep only the protected edge IP public
2. move DNS for the Minecraft hostname to the protection provider
3. keep backend and dashboard ports private exactly as this compose file now does
4. if your provider supports Proxy Protocol for Velocity, enable it there and in Velocity together
5. apply host firewall rules so only the provider's published ranges can reach `25565/tcp` and `19132/udp`
6. rotate the origin IP if it was ever shared publicly before protection was enabled

## 12. Troubleshooting

### Bedrock players cannot enter

Check:

- `proxy-data/config/Geyser-Velocity/config.yml`
- PicoLimbo forwarding mode
- Velocity logs after startup
- `force-key-authentication = false` in `proxy-data/velocity.toml`

### Whitelist or Bedrock approvals do not persist

Check shared whitelist permissions first.

### Main server fails during plugin remap

Check that the Paper container can write `main-data/plugins/.paper-remapped/`.

The known-good fix is:

```bash
sudo chown -R 1000:1000 main-data/plugins
sudo find main-data/plugins -type d -exec chmod 775 {} +
sudo find main-data/plugins -type f -exec chmod 664 {} +
docker compose -f docker-compose.proxy.yml up -d --force-recreate main
```

### Premium Java player is still asked to log in as cracked

Check:

- `plugins-output/fastlogin/config.yml` exists
- `main-data/plugins/FastLogin/allowed-proxies.txt` matches the current `plugins-output/fastlogin/proxyId.txt`
- `docker logs velocity-proxy` shows FastLogin loaded cleanly
- Mojang/sessionserver requests are not failing in the proxy log

### Cracked Java player cannot play after joining

Check:

- the player is actually whitelisted
- if they set a password, they are using `/login <password>` on the main server
- `docker logs minecraft-main` shows `OptionalAuthGuard` loaded cleanly
- `enforce-secure-profile=false` is still set on Paper

### Backups are not being created

Check:

- `.env` contains `RCON_PASSWORD` and it matches `main-data/server.properties`
- `docker compose -f docker-compose.proxy.yml logs backup-main`
- `docker compose -f docker-compose.proxy.yml logs state-backup`
- `backups/main/` and `backups/state/` are writable on the host

### Monitoring shows stale backups

Check:

- a new archive exists in `backups/main/` and `backups/state/`
- `BACKUP_INTERVAL`, `STATE_BACKUP_INTERVAL`, and `BACKUP_MAX_AGE_HOURS` are compatible
- `docker compose -f docker-compose.proxy.yml logs stack-monitor`

### Deactivated player still gets in

Check:

- dashboard row is `active: false`
- proxy status includes `blocked_count > 0`
- `plugins-output/whitelist-router/blocked_players.json` contains the player

### Proxy plugin changes are not taking effect

Rebuild and copy the plugin:

```bash
docker compose -f docker-compose.proxy.yml build whitelist-router-builder
docker compose -f docker-compose.proxy.yml run --rm whitelist-router-builder
docker compose -f docker-compose.proxy.yml up -d --force-recreate velocity
```
