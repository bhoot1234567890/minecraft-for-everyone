# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Minecraft server infrastructure supporting Java and Bedrock editions through Velocity proxy, Custom whitelist management with web dashboard.

**Documentation:** See `docs/` directory for comprehensive guides:
- `docs/ARCHITECTURE.md` - System architecture and data flows
- `docs/SETUP.md` - Step-by-step recreation guide
- `docs/API.md` - Complete API reference
- `docs/COMPONENTS.md` - Component-specific documentation

## Quick Commands

### Start/Stop Services
```bash
# Proxy mode (recommended)
docker-compose -f docker-compose.proxy.yml up -d
docker-compose -f docker-compose.proxy.yml down

# Simple mode
docker-compose up -d
docker-compose down
```

### View Logs
```bash
docker-compose -f docker-compose.proxy.yml logs -f velocity
docker-compose -f docker-compose.proxy.yml logs -f main
docker-compose -f docker-compose.proxy.yml logs -f whitelist-manager
```

### Execute Minecraft Commands
```bash
docker exec minecraft-main rcon-cli <command>
```

### Rebuild Custom Components
```bash
docker-compose -f docker-compose.proxy.yml build whitelist-router-builder whitelist-manager
docker-compose -f docker-compose.proxy.yml up -d
```

### Local Development

**Whitelist Manager (Go):**
```bash
cd whitelist-manager
go mod tidy
go run main.go
```

**WhitelistRouter Plugin (Java):**
```bash
cd velocity-whitelist-router
./gradlew shadowJar
# Output: build/libs/WhitelistRouter.jar
```

## Architecture Summary

```
Players (Java/Bedrock) → Velocity Proxy → [Whitelisted? → Main Server]
                                              ↓
                                    [Not Whitelisted? → Limbo Server]
                                              ↓
                                    [Bedrock? → Capture XUID → Pending List]
```

## Key Files
| Path | Purpose |
|------|---------|
| `docker-compose.proxy.yml` | Full proxy stack config |
| `proxy-data/velocity.toml` | Velocity proxy configuration |
| `shared-whitelist/whitelist.json` | Central whitelist |
| `whitelist-manager/main.go` | Go web dashboard |
| `velocity-whitelist-router/src/main/java/com/minecraft/router/` | Velocity plugin |

## Ports
| Port | Service |
|------|---------|
| 25565 | Velocity proxy (Java) |
| 19132/udp | Geyser (Bedrock) |
| 3000 | Whitelist Manager dashboard |
| 8080 | WhitelistRouter API (internal) |
