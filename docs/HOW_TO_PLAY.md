# How to Play - Minecraft Server Guide

Welcome! This guide will help you get started playing on our Minecraft server. We support both **Java Edition** (PC) and **Bedrock Edition** (mobile, console, Windows 10/11).

---

## Quick Start

| Platform | Server Address | Port |
|----------|---------------|------|
| Java Edition (PC/Mac/Linux) | `02031998.xyz` | `25565` (default) |
| Bedrock Edition (Mobile/Console/Windows) | `02031998.xyz` | `19132` |

---

## Step 1: Get Minecraft

### Option A: Java Edition (PC/Mac/Linux)

Java Edition is the original version of Minecraft for desktop computers.

**Purchase (Official):**
- Official website: https://www.minecraft.net/en-us/store/minecraft-java-bedrock
- Microsoft Store (Windows)

**Free Alternative - TLauncher:**

TLauncher is a free alternative launcher that lets you play without an official premium login.

**System Requirements:**
- Java 21 or later (usually bundled with modern launchers)
- Windows 10/11, macOS 10.14+, or Linux

### Option B: Bedrock Edition (Mobile/Console/Windows 10+)

Bedrock Edition runs on phones, tablets, consoles, and Windows 10/11.

| Platform | Where to Get |
|----------|-------------|
| iOS (iPhone/iPad) | App Store |
| Android | Google Play |
| Windows 10/11 | Microsoft Store |
| Xbox | Xbox Store |
| PlayStation | PlayStation Store |
| Nintendo Switch | Nintendo eShop |

---

## Step 2: Launch the Game

### Java Edition

1. Open your launcher
2. Start Minecraft `1.21.x`
3. Wait for the main menu to load

### Bedrock Edition

1. Open Minecraft from your device
2. Sign in with your Microsoft account
3. Wait for the main menu to load

---

## Step 3: Add the Server

### Java Edition

1. From the main menu, click **Multiplayer**
2. Click **Add Server**
3. Enter:
   - **Server Name:** `Our Minecraft Server`
   - **Server Address:** `02031998.xyz`
4. Click **Done**
5. Click the server in your list, then click **Join Server**

### Bedrock Edition

1. From the main menu, click **Play**
2. Click the **Servers** tab
3. Scroll down and click **Add Server**
4. Enter:
   - **Server Name:** `Our Minecraft Server`
   - **Server Address:** `02031998.xyz`
   - **Port:** `19132`
5. Click **Save**
6. Click the server to connect

---

## Step 4: Know the login flow

This server is **whitelist-based**.
If your account is not approved, you will not reach the main world.

### Premium Java players

- If you own Minecraft Java Edition, connect normally with the official launcher
- If your username is whitelisted, you should go straight in with no password prompt
- The server validates your session as a real premium account

### Cracked / offline Java players

- If your username is whitelisted, you can still join with a cracked launcher
- If you have **not** set a password, you will be allowed in but warned every login that your name can be impersonated
- You can enable protection with:
  - `/register <password> <confirm>`
- If you **did** set a password, you must use:
  - `/login <password>`
  every time you join before you can move, chat, or interact
- You can change it later with:
  - `/changepassword <oldPassword> <newPassword>`

### Bedrock players

- Bedrock uses your Microsoft / Xbox identity through Floodgate
- If your Bedrock account is approved, you should join normally without the Java password flow

### Unapproved players

- Unapproved Java or Bedrock players are sent to the holding server instead of the main world
- Ask an admin to whitelist or approve you first

---

## Understanding Your Connection

```
You Connect → Velocity Proxy → Are you approved?
                                 ↓
                      NO  → Holding server (Limbo)
                                 ↓
                      YES → Bedrock or premium Java?
                                 ↓
                   YES → Main World immediately
                                 ↓
                   NO  → Cracked Java on Main
                                 ↓
              No password set? → Warning only
              Password set?    → /login required
```

### What is Limbo?

Limbo is a holding server for players who are not currently allowed into the main world.

You may end up there if:
- your account is not whitelisted yet
- your Bedrock account is still pending approval
- an admin deactivated your access

---

## Common Issues & Solutions

### "Failed to connect to server"

| Possible Cause | Solution |
|---------------|----------|
| Wrong server address | Double-check the address with an admin |
| Server is offline | Contact an admin to check server status |
| Firewall blocking | Make sure ports 25565 (Java) or 19132/UDP (Bedrock) are open |
| VPN interference | Try disabling your VPN |

### "Disconnected" immediately after joining

Common causes:
1. Your account is not whitelisted yet
2. You are using a premium Java name from a cracked launcher, so the server rejected the premium check
3. The proxy or main server is restarting

### "Invalid session" or "Not authenticated"

| Platform | Solution |
|----------|---------|
| Premium Java Edition | Log out and back into the Minecraft Launcher |
| Cracked / offline Java | Make sure your whitelisted name is correct; if you enabled password protection, use `/login <password>` after joining |
| Bedrock Edition | Sign out and back into your Microsoft account |

### Bedrock: "Unable to connect to world"

1. Make sure you're using port **19132**
2. Check that you have a stable internet connection
3. Verify your Microsoft account is signed in
4. Ask an admin whether your Bedrock account is approved yet

### Java: "Outdated client" or "Outdated server"

1. Open your launcher
2. Make sure you're using the expected version
3. If needed, ask an admin which version is currently running

---

## Tips for New Players

### Recommended Settings

**Video Settings (Java):**
- Render Distance: 8-12 chunks
- Smooth Lighting: Maximum
- Clouds: Fancy

**Settings (Bedrock):**
- Render Distance: 8-12 chunks
- Beautiful Skies: On
- Fancy Leaves: On

### Useful Commands

| Command | Description |
|---------|-------------|
| `/register <password> <confirm>` | Enable optional password protection for cracked Java logins |
| `/login <password>` | Log into a protected cracked Java account |
| `/changepassword <old> <new>` | Change your cracked-account password |

*Gameplay commands depend on the server plugins installed on the main world.*

---

## Server Rules

1. **Be respectful** to all players
2. **No griefing**
3. **No cheating**
4. **No spam** in chat
5. **Have fun!**

---

## Need Help?

- **In-game:** Ask in chat if you can already reach the main world
- **Approval issues:** Contact an admin to get whitelisted or approved
- **Dashboard:** Admins should use the locally bound panel at `http://localhost:3000`

---

## Quick Reference Card

```
JAVA EDITION
Address: 02031998.xyz
Port:   25565

BEDROCK EDITION
Address: 02031998.xyz
Port:   19132

BEFORE YOU JOIN
1. Make sure your account is approved
2. Use the correct address
3. Premium Java: official launcher recommended
4. Cracked Java: /register is optional, /login is only needed if you enabled protection
```
