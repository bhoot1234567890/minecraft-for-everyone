# Minecraft Server Architecture

## Overview

This repository runs a proxy-first Minecraft network:

- **Java players** connect to **Velocity** on port `25565`
- **Bedrock players** connect to **Geyser-on-Velocity** on UDP port `19132`
- **Dashboard admins** use the whitelist manager on `127.0.0.1:3000`
- **Whitelisted premium Java players** are validated by **FastLogin** against Mojang and go straight to the **main Paper** server
- **Whitelisted cracked Java players** also reach **main**, where **OptionalAuthGuard** either warns them every join or requires `/login` if they enabled a password
- **Whitelisted Bedrock players** keep using **Floodgate/XUID** identity and do not need a Java password flow
- **Unwhitelisted or blocked players** are routed to **PicoLimbo** as a holding server

The recommended stack is defined in `docker-compose.proxy.yml`.

## High-Level Flow

```text
Java client  ─┐
              ├─> Velocity proxy (25565 / 19132 UDP)
Bedrock client┘        │
                       ├─> Geyser handles Bedrock protocol translation
                       ├─> Floodgate exposes Bedrock UUID + XUID
                       ├─> WhitelistRouter checks whitelist / blocked state
                       │      ├─ allowed     -> main
                       │      └─ not allowed -> limbo
                       └─> FastLogin checks Java premium ownership on allowed joins
                                  │
                                  └─> Main Paper
                                         ├─ premium Java -> play immediately
                                         ├─ Bedrock -> play immediately
                                         ├─ cracked Java with no password -> play, but warned each join
                                         └─ cracked Java with password -> must `/login <password>` before interaction
```

## Components

### 1. Velocity proxy

- **Compose service:** `velocity`
- **Image:** `itzg/mc-proxy:java21`
- **Config:** `proxy-data/velocity.toml`
- **External ports:**
  - `25565 -> 25577/tcp` for Java
  - `19132/udp` for Bedrock
- **Internal-only API:** WhitelistRouter listens on `8080` inside Docker and is authenticated with `PROXY_API_TOKEN`

Velocity is the network entrypoint and hosts:

- `Geyser.jar`
- `Floodgate.jar`
- `FastLoginVelocity.jar`
- `WhitelistRouter.jar`

### 2. Main Paper server

- **Compose service:** `main`
- **Image:** `itzg/minecraft-server:java21`
- **Version:** Paper `1.21.11`
- **Data dir:** `main-data/`
- **Behavior:** accepts only players that the proxy allows
- **Exposure:** no host port is published; only Velocity reaches it

Paper is configured for Velocity modern forwarding in:

- `main-data/config/paper-global.yml`
- `main-data/server.properties`

Runtime plugins on main:

- `FastLoginBukkit.jar`
- `OptionalAuthGuard.jar`

### 3. PicoLimbo

- **Compose service:** `limbo`
- **Type:** PicoLimbo via custom server jar
- **Data dir:** `limbo-data/`
- **Behavior:** holding server for unwhitelisted or blocked players
- **Exposure:** no host port is published; only Velocity reaches it

PicoLimbo is configured for **Velocity Modern Forwarding**, which is required for the current setup.

### 4. Whitelist Manager dashboard

- **Compose service:** `whitelist-manager`
- **Port:** `3000`
- **Host binding:** `127.0.0.1:3000`
- **Source:** `whitelist-manager/`
- **UI:** `whitelist-manager/static/index.html`

The dashboard provides:

- login-protected admin access
- whitelist and pending Bedrock visibility
- activate/deactivate controls
- log viewing
- proxy-side blocked-player access control via authenticated internal API

### 5. Authentication helpers

#### FastLogin

- runs on both Velocity and Paper
- validates premium Java accounts with Mojang/sessionserver
- stores proxy-side auth data in MariaDB (`fastlogin-db`)
- uses `proxyId.txt` on Velocity plus `allowed-proxies.txt` on Paper for proxy/backend trust

#### OptionalAuthGuard

- runs on Paper only
- exempts premium Java and Bedrock players
- warns unprotected cracked Java players every join
- requires `/login <password>` for cracked Java players who opted into password protection
- stores password hashes in `main-data/plugins/OptionalAuthGuard/passwords.json`

### 6. Operational sidecars

- **Compose service:** `backup-main`
  - **Image:** `itzg/mc-backup:2025.8.1`
  - **Behavior:** RCON-coordinated backups of `main-data/` into `backups/main/`
- **Compose service:** `state-backup`
  - **Source:** `ops/state-backup-loop.sh`
  - **Behavior:** periodic tar snapshots of proxy, limbo, whitelist, and plugin state into `backups/state/`
- **Compose service:** `stack-monitor`
  - **Source:** `ops/stack-monitor.sh`
  - **Behavior:** local health and backup-freshness checks, writing snapshots under `monitoring/`

## Authentication and routing logic

1. Player connects to Velocity.
2. If the player is Bedrock, Floodgate exposes the Floodgate UUID and XUID.
3. WhitelistRouter evaluates access:
   - **whitelisted / approved** -> route to `main`
   - **unwhitelisted or deactivated** -> route to `limbo`
4. For Java players routed to `main`, FastLogin checks premium ownership:
   - **official premium session** -> trusted as premium
   - **non-premium / cracked session** -> treated as cracked
5. On Paper, OptionalAuthGuard applies the optional cracked-password rules:
   - **premium Java** -> no extra prompt
   - **Bedrock** -> no extra prompt
   - **cracked Java with no password** -> allowed to play, but shown a warning every login
   - **cracked Java with password** -> must use `/login <password>` before moving, chatting, or interacting

## Current auth model

The current deployment is whitelist-based and keeps `HYBRID_AUTH_MODE=false`:

- whitelist decisions happen in `WhitelistRouter`
- Open Mode still exists in the plugin, but it is disabled by default
- PicoLimbo is not the auth server anymore; it is the holding server for players who are not currently allowed into main
- premium detection is handled by FastLogin
- cracked Java password protection is optional and backend-enforced by OptionalAuthGuard

## Operations sidecars

The hardened stack includes lightweight operational helpers:

- `backup-main` uses the Paper server's internal RCON port to flush world state before archiving `main-data/`
- `state-backup` keeps a second archive stream for proxy, limbo, whitelist, and plugin state
- `stack-monitor` checks dashboard HTTP reachability, backend TCP reachability, and backup freshness

Generated artifacts live on the host in:

- `backups/main/`
- `backups/state/`
- `monitoring/status/latest.json`
- `monitoring/alerts.log`

## Deactivation / blocked players

Deactivation is more than a UI flag.

When a player is deactivated from the dashboard:

1. the dashboard marks them inactive
2. the proxy removes them from the effective whitelist
3. the proxy stores them in a blocked list
4. Open Mode will not re-add them until they are activated again

This keeps deactivated players blocked even if Open Mode is re-enabled later.

## Persistent State

### Shared whitelist volume

These files are shared between the proxy and the dashboard through `shared-whitelist/`:

- `shared-whitelist/whitelist.json`
- `shared-whitelist/pending_bedrock.json`
- `shared-whitelist/banned-players.json`

The proxy remains the effective source of truth for access control.

### Proxy plugin state

These files are stored in the plugin data directory:

- container path: `/server/plugins/whitelist-router/`
- host path: `plugins-output/whitelist-router/`

Files:

- `open_mode.json`
- `blocked_players.json`

### FastLogin state

FastLogin stores runtime state in:

- Velocity host path: `plugins-output/fastlogin/`
- Paper host path: `main-data/plugins/FastLogin/`
- MariaDB data path: `data/fastlogin-db/`

Key files:

- `plugins-output/fastlogin/config.yml`
- `plugins-output/fastlogin/proxyId.txt`
- `main-data/plugins/FastLogin/config.yml`
- `main-data/plugins/FastLogin/allowed-proxies.txt`

### OptionalAuthGuard state

- host path: `main-data/plugins/OptionalAuthGuard/passwords.json`
- behavior: stores bcrypt password hashes for cracked Java players who enabled protection

## Important operational note: permissions

The Velocity process must be able to write `shared-whitelist/`.
The Paper container must also be able to write `main-data/plugins/`, because Paper remaps plugin jars into `main-data/plugins/.paper-remapped/` at startup.

If the Paper plugin directory ownership is wrong, you will see errors like:

- `AccessDeniedException: /data/plugins/.paper-remapped/FastLoginBukkit.jar`
- `AccessDeniedException: /data/plugins/.paper-remapped/OptionalAuthGuard.jar`

The current working ownership for Paper is UID/GID `1000:1000`.

## File layout

```text
minecraft-server/
├── docker-compose.proxy.yml
├── main-data/
│   └── plugins/
│       ├── FastLogin/
│       ├── FastLoginBukkit.jar
│       ├── OptionalAuthGuard/
│       └── OptionalAuthGuard.jar
├── limbo-data/
├── proxy-data/
│   ├── velocity.toml
│   ├── forwarding.secret
│   └── config/
├── plugins-output/
│   ├── FastLoginVelocity.jar
│   ├── WhitelistRouter.jar
│   ├── fastlogin/
│   │   ├── config.yml
│   │   └── proxyId.txt
│   └── whitelist-router/
│       ├── open_mode.json
│       └── blocked_players.json
├── shared-whitelist/
│   ├── whitelist.json
│   ├── pending_bedrock.json
│   └── banned-players.json
├── optional-auth-guard/
├── velocity-whitelist-router/
├── whitelist-manager/
└── docs/
```

## Current recommended compose path

Use:

```bash
docker compose -f docker-compose.proxy.yml up -d
```

The older `docker-compose.yml` is a standalone/simple setup and does not represent the full proxy + limbo architecture documented above.

Current exposure policy:

- public: `25565/tcp`, `19132/udp`
- localhost only: `127.0.0.1:3000`
- not host-published: main backend, limbo backend, proxy API
