# Component Documentation

## WhitelistRouter Velocity plugin

### Location

`velocity-whitelist-router/`

### Responsibilities

- whitelist-based routing between `main` and `limbo`
- blocked-player enforcement for deactivated users
- pending limbo capture and approval support for Java and Bedrock
- token-protected internal HTTP API on port `8080`
- atomic persistence for whitelist, pending, blocked, and open-mode state files

### Main files

| File | Purpose |
|---|---|
| `WhitelistRouterPlugin.java` | Core routing, persistence, Open Mode, blocked-player logic |
| `ApiServer.java` | Internal HTTP API |
| `WhitelistRouterCommand.java` | `/wlr` admin commands |
| `build.gradle.kts` | Plugin build definition |

### State managed by the plugin

#### Shared files

- `shared-whitelist/whitelist.json`
- `shared-whitelist/pending_bedrock.json` (legacy filename, now used for all pending limbo requests)

#### Plugin-local files

- host: `plugins-output/whitelist-router/open_mode.json`
- host: `plugins-output/whitelist-router/blocked_players.json`
- container: `/server/plugins/whitelist-router/...`

### Key behavior

- **Whitelisted / approved player** -> route to `main`
- **Unwhitelisted player** -> route to `limbo`
- **Deactivated player** -> route to `limbo` even if previously approved
- **Open Mode** -> still supported by the plugin, but disabled by default in the current deployment

### Bedrock handling

For Bedrock players the plugin uses:

- Floodgate username: `.PlayerName`
- Floodgate UUID: canonical UUID used for whitelist matching
- Xbox XUID: stored in pending records for approval workflows

### Commands

| Command | Description |
|---|---|
| `/wlr list` | Show whitelist entry count |
| `/wlr pending` | Show pending limbo requests |
| `/wlr approve <name>` | Approve pending limbo player |
| `/wlr add <uuid> <name>` | Add player directly |
| `/wlr reload` | Reload whitelist, pending, blocked, and open-mode state |
| `/wlr openmode <on\|off>` | Toggle Open Mode |
| `/wlr status` | Show current plugin status |

## FastLogin

### Locations

Runtime plugin jars:

- Velocity: `plugins-output/FastLoginVelocity.jar`
- Paper: `main-data/plugins/FastLoginBukkit.jar`

Runtime data:

- Velocity: `plugins-output/fastlogin/`
- Paper: `main-data/plugins/FastLogin/`
- Database: `data/fastlogin-db/`

### Responsibilities

- detect whether a Java account is premium
- validate premium sessions against Mojang/sessionserver
- distinguish official premium joins from cracked joins on the same proxy
- coordinate proxy/backend trust with `proxyId.txt` and `allowed-proxies.txt`

### Important config

- `proxy-data/velocity.toml`
- `plugins-output/fastlogin/config.yml`
- `main-data/plugins/FastLogin/config.yml`
- `main-data/plugins/FastLogin/allowed-proxies.txt`

### Key behavior

- **premium Java** -> authenticates with Mojang and skips the cracked-password flow
- **cracked Java** -> is still allowed if whitelisted, subject to OptionalAuthGuard on Paper
- **Bedrock** -> keeps using Floodgate identity and does not need FastLogin password handling

## OptionalAuthGuard

### Location

Source:

- `optional-auth-guard/`

Runtime jar/data:

- `main-data/plugins/OptionalAuthGuard.jar`
- `main-data/plugins/OptionalAuthGuard/passwords.json`

### Responsibilities

- exempt premium Java players from manual login
- exempt Bedrock/Floodgate players from manual login
- warn cracked Java players when they have no password set
- require `/login <password>` for cracked Java players who enabled password protection
- provide `/register`, `/login`, and `/changepassword`

### Commands

| Command | Description |
|---|---|
| `/register <password> <confirm>` | Enable optional password protection for a cracked Java account |
| `/login <password>` | Complete login for a protected cracked Java account |
| `/changepassword <old> <new>` | Rotate an existing cracked-account password |

### Key behavior

- stores passwords by lowercase player name
- uses bcrypt hashes
- blocks movement, chat, block interaction, inventory access, and combat until `/login` succeeds for protected cracked users

## Whitelist Manager

### Location

`whitelist-manager/`

### Responsibilities

- browser-based admin UI
- login/session handling
- player list and local metadata
- activate/deactivate operations
- Bedrock approval flows
- proxy API client
- server log streaming from mounted log files

### Main files

| File | Purpose |
|---|---|
| `main.go` | Fiber app, API handlers, local player metadata, cookie auth, file-backed log tailing |
| `proxy_client.go` | Client for the token-protected Velocity plugin API |
| `static/index.html` | Indraprastha Commons dashboard |
| `Dockerfile` | Build and runtime image |

### Important behavior

The dashboard stores local player metadata, but effective access is enforced at the proxy.

That means:

- **Deactivate** updates both dashboard state and proxy access state
- **Activate** restores both dashboard state and proxy whitelist access
- **Remove** deletes the dashboard entry and removes proxy whitelist access without blocking
- the player list merges proxy blocked users back into the dashboard so inactive users stay visible
- dashboard auth depends on env-provided credentials instead of baked-in defaults
- the container no longer mounts `/var/run/docker.sock`
- log viewing reads mounted log files instead of talking to Docker directly

## PicoLimbo

### Location

`limbo-data/`

### Responsibilities

- hold players who are not currently allowed into `main`
- accept Velocity modern forwarding

### Important config

`limbo-data/server.toml`

Required forwarding section:

```toml
[forwarding]
method = "MODERN"
secret = "same-secret-as-velocity"
```

## Main Paper server

### Location

`main-data/`

### Responsibilities

- actual gameplay server
- relies on Velocity for admission
- runs FastLoginBukkit and OptionalAuthGuard
- exposes internal RCON so backups can coordinate world saves without publishing RCON on the host

### Important config

- `main-data/config/paper-global.yml`
- `main-data/server.properties`

Key requirements:

- Velocity forwarding must stay enabled and the secret must match `proxy-data/forwarding.secret`
- `proxies.velocity.online-mode` should remain `false`
- `prevent-proxy-connections=true` should remain set in `server.properties`
- `enforce-secure-profile=false` should remain set in `server.properties` for cracked Java compatibility
- `RCON_PASSWORD` in `.env` should match the main server's configured RCON password
- `main-data/plugins/` must stay writable by the Paper container UID/GID so `.paper-remapped/` can be regenerated on startup

## Backup services

### `backup-main`

- image: `itzg/mc-backup:2025.8.1`
- writes coordinated Paper world backups to `backups/main/`
- uses internal RCON only; no host RCON port is published

### `state-backup`

- build context: `ops/`
- writes tar snapshots to `backups/state/`
- covers:
  - `proxy-data/`
  - `limbo-data/`
  - `shared-whitelist/`
  - `plugins-output/`

## Monitoring

### `stack-monitor`

- build context: `ops/`
- checks dashboard reachability plus Velocity/Main/Limbo TCP listeners
- marks `main` and `state` backups stale if no fresh archive exists within the configured threshold
- writes:
  - `monitoring/status/latest.json`
  - `monitoring/alerts.log`

### Host-side smoke check

- `ops/restart-smoke-check.sh` provides a quick post-restart verification for operators
- it prints compose status, dashboard reachability, latest backups, and the latest monitoring snapshot

## Shared whitelist storage

### Location

`shared-whitelist/`

### Files

| File | Purpose |
|---|---|
| `whitelist.json` | Proxy-facing canonical whitelist |
| `pending_bedrock.json` | Pending Bedrock captures, stored as `{"entries":[]}` |
| `banned-players.json` | Dashboard ban data |

### Permission requirement

This directory must be writable by the Velocity container process.
If not, proxy-side updates fail and whitelist / Bedrock capture changes appear to work in logs but do not persist.

### Consistency notes

- both the proxy plugin and dashboard save JSON through atomic temp-file replacement
- the proxy remains the effective access-control source of truth
- the dashboard should keep using the proxy API for allow/deny decisions instead of editing access state locally
