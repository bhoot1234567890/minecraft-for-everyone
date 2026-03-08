# Server Rebranding Checklist

This document lists all files that need to be updated when changing the server name/branding.

**New Name:** Indraprastha Commons

---

## 1. Dashboard Branding (CreeperPanel → Indraprastha Commons)

| File | Line | Current | Change To |
|------|------|---------|-----------|
| `whitelist-manager/main.go` | 1001 | `AppName: "CreeperPanel"` | `AppName: "Indraprastha Commons"` |
| `whitelist-manager/static/index.html` | 46 | `<h2 class="...">CreeperPanel</h2>` | `<h2 class="...">Indraprastha Commons</h2>` |
| `whitelist-manager/static/index.html` | 83, 893, 905 | `Survival Hub` | `Indraprastha Commons` |
| `whitelist-manager/static/index.html` | 286 | `© 2025 CreeperPanel Admin Dashboard` | `© 2025 Indraprastha Commons` |
| `whitelist-manager/static/index.html` | 373 | `Login to CreeperPanel` | `Login to Indraprastha Commons` |
| `whitelist-manager/ui_team_prototype/index.html` | 43, 250 | CreeperPanel references | Indraprastha Commons (optional - prototype folder) |

---

## 2. Landing Page (play/index.html)

| File | Line | Current | Change To |
|------|------|---------|-----------|
| `whitelist-manager/static/play/index.html` | 6 | `<title>How to Play | Minecraft Server</title>` | `<title>How to Play | Indraprastha Commons</title>` |
| `whitelist-manager/static/play/index.html` | 42 | `02031998.xyz` (header title) | `Indraprastha Commons` |
| `whitelist-manager/static/play/index.html` | 115 | `Connect Now: 02031998.xyz` | Keep as-is (address) |
| `whitelist-manager/static/play/index.html` | 324, 354 | `Server Name: Our Server` | `Server Name: Indraprastha Commons` |
| `whitelist-manager/static/play/index.html` | 615 | `© 2025 CreeperPanel` | `© 2025 Indraprastha Commons` |

---

## 3. Server MOTD & Names (Config Files)

| File | Line | Current | Change To |
|------|------|---------|-----------|
| `docker-compose.proxy.yml` | 75 | `SERVER_NAME: "Main Survival"` | `SERVER_NAME: "Indraprastha Commons"` |
| `docker-compose.proxy.yml` | 76 | `MOTD: "Welcome to the Main Server!"` | `MOTD: "Welcome to Indraprastha Commons!"` |
| `main-data/server.properties` | 43 | `motd=Welcome to the Main Server!` | `motd=Welcome to Indraprastha Commons!` |
| `main-data/server.properties` | 61 | `server-name=Main Survival` | `server-name=Indraprastha Commons` |
| `proxy-data/velocity.toml` | 6 | `motd = "<green>Minecraft Server Proxy</green>"` | `motd = "<green>Indraprastha Commons</green>"` |
| `proxy-data/config/Geyser-Velocity/config.yml` | 34 | `motd1: "§aMinecraft Server"` | `motd1: "§aIndraprastha Commons"` |
| `proxy-data/config/Geyser-Velocity/config.yml` | 35 | `motd2: "§7Bedrock & Java Crossplay"` | Keep or update |
| `limbo-data/server.toml` | 42 | `message_of_the_day = "A Minecraft Server"` | `message_of_the_day = "Indraprastha Commons"` |
| `docker-compose.yml` | 12-13 | legacy `Cracked Minecraft Server` branding | `Indraprastha Commons` branding |

---

## 4. Documentation

| File | Lines | Current | Change To |
|------|-------|---------|-----------|
| `docs/HOW_TO_PLAY.md` | 94, 105 | `Server Name: Our Minecraft Server` | `Server Name: Indraprastha Commons` |
| `docs/HOW_TO_PLAY.md` | 1 | `# How to Play - Minecraft Server Guide` | `# How to Play - Indraprastha Commons` |
| `docs/ARCHITECTURE.md` | 1 | `# Minecraft Server Architecture` | `# Indraprastha Commons Architecture` |
| `docs/COMPONENTS.md` | 158 | `CreeperPanel dashboard` | `Indraprastha Commons dashboard` |

---

## 5. Optional / Low Priority

| File | Notes |
|------|-------|
| `whitelist-manager/ui_team_prototype/index.html` | Prototype folder - may not be in use |
| `velocity-whitelist-router/src/main/resources/velocity-plugin.json` | Plugin metadata - internal use only |
| `optional-auth-guard/src/main/resources/plugin.yml` | Plugin metadata - internal use only |

---

## Commands to Apply Changes

After making edits, rebuild and restart:

```bash
docker compose -f docker-compose.proxy.yml build whitelist-manager
docker compose -f docker-compose.proxy.yml up -d --force-recreate whitelist-manager velocity main limbo
```

---

## Summary

| Category | File Count |
|----------|------------|
| Dashboard (Go + HTML) | 4 files |
| Landing / Play pages | 2 files |
| Server Config (MOTD, server.properties) | 6 files |
| Docker Compose | 2 files |
| Documentation | 4 files |
| **Total** | **18 files** |

---

*Note: The server address `02031998.xyz` should remain unchanged - this is the connection address, not the display name.*
