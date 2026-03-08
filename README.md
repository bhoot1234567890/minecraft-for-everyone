# Indraprastha Commons

**A production-ready Minecraft server infrastructure for cross-platform, hybrid-auth communities.**

Play together regardless of platform - Java Edition, Bedrock Edition, premium accounts, or cracked launchers. This project provides a complete whitelist management system with a web dashboard, intelligent routing, and built-in operational tooling.

---

## Why This Project?

Running a private Minecraft server for a community sounds simple until you encounter the real-world constraints:

### The Cross-Platform Problem

Java Edition and Bedrock Edition are fundamentally different. They use different protocols, different authentication systems, and normally can't play together. Your friends on phones, consoles, and PCs should be able to join the same world without technical barriers.

### The Hybrid Auth Problem

Not everyone owns a premium Minecraft account. Communities often have a mix of:
- **Premium Java players** with official Mojang-authenticated accounts
- **Cracked/offline Java players** using alternative launchers
- **Bedrock players** authenticated through Microsoft/Xbox

A proper server needs to handle all three securely - validating premium accounts, protecting cracked accounts from impersonation, and integrating Bedrock identity correctly.

### The Access Control Problem

Standard Minecraft whitelists are manual and console-based. For a community server, you need:
- A way for new players to request access
- An admin dashboard to approve, manage, and deactivate players
- Proper access enforcement at the proxy level
- The ability to temporarily deactivate players without losing their data

### The Operations Problem

Running a server means backups, monitoring, and maintenance. These shouldn't be afterthoughts - they should be built into the infrastructure from day one.

---

## What This Solves

| Problem | Solution |
|---------|----------|
| Cross-platform play | Geyser proxy enables Bedrock clients to join Java servers |
| Hybrid authentication | FastLogin validates premium accounts; OptionalAuthGuard protects cracked accounts |
| Whitelist management | Web dashboard for approving, deactivating, and managing players |
| Access enforcement | Custom Velocity plugin routes unapproved players to a holding server |
| Pending player workflow | Players automatically enter limbo; admins approve from dashboard |
| Security | Premium validation, optional password protection for cracked accounts, token-protected APIs |
| Operations | Automated backups, health monitoring, log streaming |

---

## How It Works

### Architecture Overview

```
                          ┌─────────────────────────────────────────┐
                          │           Velocity Proxy                │
   Java Players ─────────▶│  (Port 25565)                           │
                          │    ├─ Geyser (Bedrock protocol)         │
   Bedrock Players ──────▶│  (UDP 19132)                            │
                          │    ├─ Floodgate (Bedrock identity)      │
                          │    ├─ FastLogin (Premium detection)     │
                          │    └─ WhitelistRouter (Access control)  │
                          └───────────────┬─────────────────────────┘
                                          │
                              ┌───────────┴───────────┐
                              │                       │
                    Whitelisted?                 Not Whitelisted?
                              │                       │
                              ▼                       ▼
                     ┌────────────────┐      ┌────────────────┐
                     │  Main Server   │      │  Limbo Server  │
                     │  (Paper 1.21)  │      │  (PicoLimbo)   │
                     │                │      │                │
                     │ Gameplay with: │      │ Holding area   │
                     │ - Premium Java │      │ for pending    │
                     │ - Bedrock      │      │ approval       │
                     │ - Cracked Java │      └────────────────┘
                     └────────────────┘
```

### Player Flow

1. **Player connects** to Velocity proxy (Java on 25565, Bedrock on 19132/udp)
2. **WhitelistRouter checks access**:
   - Whitelisted/approved -> route to main server
   - Not whitelisted -> route to limbo (holding server)
   - Deactivated -> stay in limbo
3. **FastLogin validates** Java premium accounts against Mojang
4. **On main server**, OptionalAuthGuard:
   - Premium Java / Bedrock -> play immediately
   - Cracked Java (no password) -> play with warning
   - Cracked Java (password set) -> must `/login` first

### Key Components

| Component | Purpose |
|-----------|---------|
| **Velocity** | Proxy entry point, hosts all proxy-side plugins |
| **Geyser + Floodgate** | Bedrock protocol translation and identity |
| **FastLogin** | Premium Java account validation |
| **WhitelistRouter** | Custom plugin for access control and routing |
| **OptionalAuthGuard** | Custom plugin for cracked Java password protection |
| **PicoLimbo** | Lightweight holding server for unapproved players |
| **Whitelist Manager** | Go web dashboard for admin management |
| **Backup Sidecars** | Automated world and state backups |
| **Stack Monitor** | Health and backup freshness monitoring |

---

## Who This Is For

### Community Server Operators

If you're running a private server for friends, a Discord community, or a small group, this infrastructure gives you:
- A professional setup without the complexity of large networks
- Easy player management through a web dashboard
- Cross-platform support out of the box

### Mixed-Auth Communities

If your community includes both premium and cracked players:
- Premium players get seamless authenticated access
- Cracked players can optionally protect their accounts with passwords
- No one gets locked out because of their account type

### Bedrock + Java Groups

If you want mobile, console, and PC players together:
- Full Bedrock support with proper Microsoft authentication
- No weird workarounds or third-party clients needed
- Bedrock players use their normal Xbox/Microsoft identity

### Operators Who Value Operations

If you want a server that's easy to maintain:
- Docker Compose for simple deployment
- Automated backups built-in
- Health monitoring and alerting
- All configuration in version-controllable files

---

## Quick Start

### Prerequisites

- Linux host
- Docker + Docker Compose plugin
- Available ports: 25565/tcp (Java), 19132/udp (Bedrock)

### Deploy

```bash
# Clone the repository
git clone <repository-url>
cd minecraft-server

# Create environment file
cp .env.example .env
# Edit .env with your secrets

# Install FastLogin
./ops/install-fastlogin.sh

# Build and start
docker compose -f docker-compose.proxy.yml build
docker compose -f docker-compose.proxy.yml up -d

# Sync FastLogin proxy ID
./ops/sync-fastlogin-proxy-id.sh
docker compose -f docker-compose.proxy.yml up -d --force-recreate main
```

### Access the Dashboard

```bash
# Dashboard is bound to localhost only
http://localhost:3000/admin
```

### Connect Players

| Platform | Address | Port |
|----------|---------|------|
| Java Edition | `your-domain.com` | 25565 (default) |
| Bedrock Edition | `your-domain.com` | 19132 |

---

## Features

### Access Control

- **Whitelist-based routing**: Only approved players reach the main world
- **Pending limbo flow**: Unapproved players enter a holding area
- **Dashboard approval**: Admins approve pending players with one click
- **Instant transfer**: Approved players in limbo move to main immediately
- **Deactivation**: Temporarily revoke access without losing player data
- **Blocked player persistence**: Deactivated players stay blocked even if Open Mode is enabled

### Authentication

- **Premium Java**: Validated against Mojang/sessionserver
- **Cracked Java**: Optional password protection with `/register`, `/login`, `/changepassword`
- **Bedrock**: Uses Microsoft/Xbox identity through Floodgate
- **Session warnings**: Unprotected cracked accounts get warned on every join

### Administration

- **Web dashboard**: Player list, pending requests, logs, stats
- **Session-based auth**: Secure admin login with HttpOnly cookies
- **Log streaming**: Real-time server logs in the browser
- **Proxy API**: Token-protected internal API for automation

### Operations

- **Automated backups**: RCON-coordinated world backups
- **State backups**: Periodic snapshots of proxy, whitelist, and plugin state
- **Health monitoring**: Dashboard reachability, TCP listeners, backup freshness
- **Alert logging**: State changes and stale backup warnings

---

## Project Structure

```
minecraft-server/
├── docker-compose.proxy.yml    # Full stack configuration
├── main-data/                  # Paper server data
│   └── plugins/
│       ├── FastLogin/          # FastLogin Bukkit config
│       └── OptionalAuthGuard/  # Password hashes
├── limbo-data/                 # PicoLimbo holding server
├── proxy-data/                 # Velocity configuration
│   ├── velocity.toml
│   ├── forwarding.secret
│   └── config/Geyser-Velocity/
├── plugins-output/             # Proxy plugins and data
│   ├── fastlogin/
│   └── whitelist-router/
├── shared-whitelist/           # Shared state files
│   ├── whitelist.json
│   └── pending_bedrock.json
├── velocity-whitelist-router/  # Custom proxy plugin source
├── whitelist-manager/          # Dashboard source (Go)
├── optional-auth-guard/        # Custom Paper plugin source
├── ops/                        # Operational scripts
├── backups/                    # Backup output
│   ├── main/
│   └── state/
├── monitoring/                 # Health snapshots
└── docs/                       # Documentation
    ├── ARCHITECTURE.md
    ├── SETUP.md
    ├── API.md
    ├── COMPONENTS.md
    └── HOW_TO_PLAY.md
```

---

## Documentation

| Document | Purpose |
|----------|---------|
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | System architecture and data flows |
| [SETUP.md](docs/SETUP.md) | Step-by-step deployment guide |
| [API.md](docs/API.md) | Complete API reference |
| [COMPONENTS.md](docs/COMPONENTS.md) | Component-specific documentation |
| [HOW_TO_PLAY.md](docs/HOW_TO_PLAY.md) | Player-facing guide |

---

## Common Commands

```bash
# Start/stop services
docker compose -f docker-compose.proxy.yml up -d
docker compose -f docker-compose.proxy.yml down

# View logs
docker compose -f docker-compose.proxy.yml logs -f velocity
docker compose -f docker-compose.proxy.yml logs -f main
docker compose -f docker-compose.proxy.yml logs -f whitelist-manager

# Execute Minecraft commands
docker exec minecraft-main rcon-cli <command>

# Rebuild custom components
docker compose -f docker-compose.proxy.yml build whitelist-router-builder whitelist-manager
docker compose -f docker-compose.proxy.yml up -d

# Health check
./ops/restart-smoke-check.sh
```

---

## Security Notes

- Dashboard binds to localhost only (`127.0.0.1:3000`)
- Internal proxy API is not published to host
- Admin credentials come from environment variables
- Proxy API requires token authentication
- Session cookies are HttpOnly with SameSite=Strict
- For public deployment, consider adding TCPShield or similar DDoS protection

---

## License

This project is provided as-is for educational and personal use. Individual components (Minecraft, Paper, Velocity, Geyser, FastLogin, etc.) are subject to their respective licenses.
