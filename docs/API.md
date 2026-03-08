# API Documentation

## Overview

There are two API layers:

1. **Whitelist Manager API** on `127.0.0.1:3000`
2. **WhitelistRouter internal API** on container-local port `8080`

The dashboard talks to the internal proxy API for routing-related state using `PROXY_API_TOKEN`.
The current deployment keeps `HYBRID_AUTH_MODE=false`, so the proxy API reflects the live whitelist-routing state.

---

## Whitelist Manager API (`:3000`)

Protected routes require:

- the session cookie returned by `POST /api/login`, or
- `Authorization: Bearer <token>` for non-browser/API clients

### Auth

#### `POST /api/login`

```json
{
  "username": "admin",
  "password": "your_password"
}
```

Response:

```json
{
  "message": "Login successful"
}
```

Notes:

- The session cookie is `HttpOnly`
- `SameSite=Strict`
- `Secure` is controlled by `SESSION_COOKIE_SECURE`

#### `GET /api/check-auth`

Response:

```json
{
  "authenticated": true,
  "expiresAt": "2026-03-08T00:00:00Z"
}
```

### Players

#### `GET /api/players?page=1&pageSize=20&search=query`

Returns dashboard players plus any blocked/deactivated proxy players merged into the list.

#### `POST /api/players`

Add a player to the dashboard-managed list.

```json
{
  "name": "PlayerName"
}
```

#### `DELETE /api/players/:name`

Remove a player from the dashboard list.

#### `PUT /api/players/:name/deactivate`

Marks the player inactive **and** removes effective proxy access.

#### `PUT /api/players/:name/activate`

Marks the player active **and** restores proxy whitelist access.

#### `PUT /api/players/:name/status`

```json
{
  "status": "online"
}
```

#### `PUT /api/players/:name/rank`

```json
{
  "rank": "Moderator"
}
```

### Bans

#### `GET /api/bans`

List active bans.

#### `POST /api/bans`

```json
{
  "name": "PlayerName",
  "reason": "Griefing",
  "duration": 24
}
```

#### `DELETE /api/bans/:name`

Remove an active ban.

### Bedrock workflow

#### `GET /api/pending`

List dashboard-local pending players.

#### `POST /api/pending`

```json
{
  "name": "BedrockPlayer",
  "platform": "bedrock"
}
```

#### `DELETE /api/pending/:name`

Remove a dashboard-local pending player.

#### `POST /api/bedrock/register`

Returns an error in hardened mode. Use the proxy approval flow or Open Mode instead.

#### `GET /api/bedrock/status`

Returns whether that registration flow is active.

### Logs and stats

#### `GET /api/stats`

Returns aggregated dashboard stats.

#### `GET /api/logs`

Return recent buffered lines from the mounted main server log file.

#### `GET /api/logs/stream`

Server-Sent Events stream of the mounted main server log file.

### Proxy integration endpoints exposed by the dashboard

#### `GET /api/proxy/pending`

Fetch pending Bedrock players from the proxy.

#### `POST /api/proxy/approve`

Approve a pending Bedrock player in the proxy.

```json
{
  "name": ".BedrockPlayer"
}
```

#### `GET /api/proxy/status`

Example response:

```json
{
  "online": true,
  "whitelisted": 5,
  "pending_bedrock": 0,
  "open_mode": false,
  "hybrid_auth_mode": false,
  "main_server": true,
  "limbo_server": true
}
```

#### `GET /api/proxy/open-mode`

Example response:

```json
{
  "enabled": false,
  "updated_at": 1772919792124,
  "updated_by": "dashboard"
}
```

#### `POST /api/proxy/open-mode`

```json
{
  "enabled": true
}
```

Note: Open Mode only affects normal routing when `hybrid_auth_mode` is `false`, which is the current deployment.

---

## WhitelistRouter internal API (`:8080`)

This API is intended for local/network-internal use by the dashboard and operators.
It is **not** published on the host in the recommended compose file.
When `PROXY_API_TOKEN` is set, every request must include:

```http
Authorization: Bearer <PROXY_API_TOKEN>
```

### `GET /api/status`

Example response:

```json
{
  "whitelisted_count": 5,
  "pending_count": 0,
  "blocked_count": 1,
  "open_mode": false,
  "hybrid_auth_mode": false,
  "main_server": true,
  "limbo_server": true
}
```

### `GET /api/pending`

Example response:

```json
{
  "pending": [
    {
      "name": ".BedrockPlayer",
      "xuid": "2535...",
      "floodgate_uuid": "00000000-0000-0000-0009-...",
      "captured_at": 1772910000000
    }
  ]
}
```

### `POST /api/approve`

Approve and persist a pending Bedrock player.

```json
{
  "name": ".BedrockPlayer"
}
```

### `POST /api/whitelist`

Directly add a player to the proxy whitelist.

```json
{
  "uuid": "00000000-0000-0000-0009-01f38476db79",
  "name": ".WinterMist88971"
}
```

### `GET /api/open-mode`

Example response:

```json
{
  "enabled": false,
  "updatedAt": 0,
  "updatedBy": "system"
}
```

### `POST /api/open-mode`

```json
{
  "enabled": true,
  "updatedBy": "dashboard"
}
```

### `GET /api/blocked`

Returns the proxy-side blocked/deactivated players.

Example response:

```json
{
  "blocked": [
    {
      "uuid": "00000000-0000-0000-0009-01f38476db79",
      "name": ".WinterMist88971",
      "blocked_at": 1772920028839
    }
  ]
}
```

### `POST /api/access`

Set whether a player is active at the proxy level.

This is how dashboard activate/deactivate maps to real proxy access.

```json
{
  "uuid": "00000000-0000-0000-0009-01f38476db79",
  "name": ".WinterMist88971",
  "active": false
}
```

Behavior:

- `active: false` -> remove from whitelist and add to blocked list
- `active: true` -> remove from blocked list and add to whitelist

---

## `/wlr` command surface

Available in Velocity with `whitelistrouter.admin`:

| Command | Purpose |
|---|---|
| `/wlr list` | Show whitelist entry count |
| `/wlr pending` | Show pending Bedrock players |
| `/wlr approve <name>` | Approve pending Bedrock player |
| `/wlr add <uuid> <name>` | Add a whitelist entry |
| `/wlr reload` | Reload proxy plugin state |
| `/wlr openmode <on\|off>` | Toggle Open Mode |
| `/wlr status` | Show routing/plugin status |
