package com.minecraft.router;

import com.google.gson.Gson;
import com.google.gson.GsonBuilder;
import com.google.gson.JsonElement;
import com.google.gson.JsonParser;
import com.google.gson.reflect.TypeToken;
import com.google.inject.Inject;
import com.velocitypowered.api.event.Subscribe;
import com.velocitypowered.api.event.player.ServerPreConnectEvent;
import com.velocitypowered.api.event.proxy.ProxyInitializeEvent;
import com.velocitypowered.api.plugin.Plugin;
import com.velocitypowered.api.plugin.annotation.DataDirectory;
import com.velocitypowered.api.proxy.Player;
import com.velocitypowered.api.proxy.ProxyServer;
import com.velocitypowered.api.proxy.server.RegisteredServer;
import org.geysermc.floodgate.api.FloodgateApi;
import org.geysermc.floodgate.api.player.FloodgatePlayer;
import org.slf4j.Logger;

import java.io.IOException;
import java.lang.reflect.Type;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Optional;
import java.util.Set;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

@Plugin(
    id = "whitelist-router",
    name = "WhitelistRouter",
    version = "1.0.0",
    description = "Routes players based on whitelist, captures Bedrock XUIDs"
)
public class WhitelistRouterPlugin {

    private final ProxyServer server;
    private final Logger logger;
    private final Path dataDirectory;

    private final Gson gson = new GsonBuilder().setPrettyPrinting().create();
    private final Type whitelistType = new TypeToken<List<WhitelistEntry>>(){}.getType();
    private final Type pendingType = new TypeToken<List<PendingPlayer>>(){}.getType();
    private final Type blockedType = new TypeToken<List<BlockedPlayer>>(){}.getType();

    private final Set<String> whitelistedUuids = ConcurrentHashMap.newKeySet();
    private final Map<String, WhitelistEntry> whitelistEntries = new ConcurrentHashMap<>();
    private final Map<String, PendingPlayer> pendingBedrockPlayers = new ConcurrentHashMap<>();
    private final Map<String, BlockedPlayer> blockedPlayers = new ConcurrentHashMap<>();

    private Path whitelistFile;
    private Path pendingFile;
    private Path openModeFile;
    private Path blockedFile;
    private ApiServer apiServer;
    private volatile OpenModeState openModeState = OpenModeState.disabled();
    private volatile boolean hybridAuthMode;

    @Inject
    public WhitelistRouterPlugin(ProxyServer server, Logger logger, @DataDirectory Path dataDirectory) {
        this.server = server;
        this.logger = logger;
        this.dataDirectory = dataDirectory;
    }

    @Subscribe
    public void onProxyInitialize(ProxyInitializeEvent event) {
        logger.info("WhitelistRouter initializing...");

        try {
            Files.createDirectories(dataDirectory);
        } catch (IOException e) {
            logger.error("Failed to create data directory", e);
        }

        String sharedWhitelistPath = System.getenv().getOrDefault("WHITELIST_PATH", "/whitelist");
        Path whitelistDir = Path.of(sharedWhitelistPath);
        if (!Files.exists(whitelistDir)) {
            whitelistDir = dataDirectory;
        }

        whitelistFile = whitelistDir.resolve("whitelist.json");
        pendingFile = whitelistDir.resolve("pending_bedrock.json");
        openModeFile = dataDirectory.resolve("open_mode.json");
        blockedFile = dataDirectory.resolve("blocked_players.json");

        loadWhitelist();
        loadPendingPlayers();
        loadOpenModeState();
        loadBlockedPlayers();
        hybridAuthMode = Boolean.parseBoolean(System.getenv().getOrDefault("HYBRID_AUTH_MODE", "false"));

        server.getCommandManager().register("wlr", new WhitelistRouterCommand(this));

        int apiPort = Integer.parseInt(System.getenv().getOrDefault("API_PORT", "8080"));
        apiServer = new ApiServer(this, apiPort);
        try {
            apiServer.start();
        } catch (Exception e) {
            logger.error("Failed to start API server", e);
        }

        logger.info(
            "WhitelistRouter initialized! Loaded {} whitelist entries (open mode: {}, hybrid auth mode: {})",
            getWhitelistEntryCount(),
            openModeState.enabled,
            hybridAuthMode
        );
    }

    @Subscribe
    public void onServerPreConnect(ServerPreConnectEvent event) {
        Player player = event.getPlayer();
        String compactUuid = player.getUniqueId().toString().replace("-", "");
        String canonicalUuid = canonicalizeUuid(player.getUniqueId().toString());
        String correctedName = player.getUsername();

        FloodgateApi floodgateApi = null;
        try {
            floodgateApi = FloodgateApi.getInstance();
        } catch (NoClassDefFoundError | Exception ignored) {
        }

        boolean isBedrock = false;
        String xuid = null;

        try {
            if (floodgateApi != null && floodgateApi.isFloodgatePlayer(player.getUniqueId())) {
                isBedrock = true;
                FloodgatePlayer floodgatePlayer = floodgateApi.getPlayer(player.getUniqueId());
                if (floodgatePlayer != null) {
                    xuid = floodgatePlayer.getXuid();
                    correctedName = "." + floodgatePlayer.getUsername();
                    logger.info(
                        "Bedrock player detected: {} (XUID: {}, UUID: {})",
                        correctedName,
                        xuid,
                        player.getUniqueId()
                    );
                }
            }
        } catch (NoClassDefFoundError ignored) {
        }

        boolean isBlocked = blockedPlayers.containsKey(canonicalUuid);
        if (!isBlocked && !isBedrock) {
            BlockedPlayer blockedByName = findBlockedPlayerByName(correctedName);
            if (blockedByName != null) {
                String previousBlockedUuid = canonicalizeUuid(blockedByName.uuid);
                if (!canonicalUuid.equals(previousBlockedUuid)) {
                    migrateBlockedIdentity(blockedByName, previousBlockedUuid, canonicalUuid, correctedName);
                }
                isBlocked = true;
            }
        }
        if (isBlocked) {
            logger.info("Player {} is deactivated, routing to limbo", correctedName);
        }

        if (hybridAuthMode) {
            if (!isBlocked) {
                return;
            }

            Optional<RegisteredServer> limbo = server.getServer("limbo");
            if (limbo.isPresent()) {
                event.setResult(ServerPreConnectEvent.ServerResult.allowed(limbo.get()));
            } else {
                logger.warn("Limbo server not found! Blocked player {} will be disconnected", correctedName);
                event.setResult(ServerPreConnectEvent.ServerResult.denied());
            }
            return;
        }

        boolean isWhitelisted = whitelistedUuids.contains(canonicalUuid) || whitelistedUuids.contains(compactUuid);
        if (!isWhitelisted && !isBedrock) {
            WhitelistEntry whitelistedByName = findWhitelistEntryByName(correctedName);
            if (whitelistedByName != null) {
                String previousWhitelistUuid = canonicalizeUuid(whitelistedByName.uuid);
                if (!canonicalUuid.equals(previousWhitelistUuid)) {
                    migrateWhitelistIdentity(whitelistedByName, previousWhitelistUuid, canonicalUuid, correctedName);
                }
                isWhitelisted = true;
            }
        }
        if (!isBlocked && !isWhitelisted && openModeState.enabled) {
            addToWhitelist(canonicalUuid, correctedName);
            isWhitelisted = true;
            logger.info(
                "Open mode auto-whitelisted {} ({}){}",
                correctedName,
                canonicalUuid,
                xuid != null ? " with XUID " + xuid : ""
            );
        }

        if (!isWhitelisted) {
            Optional<RegisteredServer> limbo = server.getServer("limbo");
            if (limbo.isPresent()) {
                logger.info("Player {} not whitelisted, routing to limbo", correctedName);
                event.setResult(ServerPreConnectEvent.ServerResult.allowed(limbo.get()));

                if (isBedrock && xuid != null) {
                    captureBedrockPlayer(correctedName, xuid, canonicalUuid);
                }
            } else {
                logger.warn("Limbo server not found! Player {} will be disconnected", correctedName);
                event.setResult(ServerPreConnectEvent.ServerResult.denied());
            }
            return;
        }

        Optional<RegisteredServer> main = server.getServer("main");
        if (main.isPresent()) {
            logger.info("Player {} whitelisted, routing to main server", correctedName);
            event.setResult(ServerPreConnectEvent.ServerResult.allowed(main.get()));
        } else {
            logger.warn("Main server not found!");
        }
    }

    public synchronized void reloadState() {
        loadWhitelist();
        loadPendingPlayers();
        loadOpenModeState();
        loadBlockedPlayers();
    }

    private void captureBedrockPlayer(String name, String xuid, String floodgateUuid) {
        String key = normalizePendingKey(name);
        if (!pendingBedrockPlayers.containsKey(key)) {
            PendingPlayer pending = new PendingPlayer();
            pending.name = name;
            pending.xuid = xuid;
            pending.floodgateUuid = canonicalizeUuid(floodgateUuid);
            pending.capturedAt = System.currentTimeMillis();
            pendingBedrockPlayers.put(key, pending);
            savePendingPlayers();
            logger.info("Captured pending Bedrock player: {} (XUID: {})", name, xuid);
        }
    }

    public synchronized void loadWhitelist() {
        whitelistedUuids.clear();
        whitelistEntries.clear();
        try {
            if (!Files.exists(whitelistFile)) {
                logger.info("Whitelist file not found, creating empty");
                writeFileAtomically(whitelistFile, "[]");
                return;
            }

            String content = Files.readString(whitelistFile).trim();
            if (content.isEmpty()) {
                return;
            }

            List<WhitelistEntry> entries = gson.fromJson(content, whitelistType);
            if (entries == null) {
                return;
            }

            for (WhitelistEntry entry : entries) {
                if (entry == null || entry.uuid == null || entry.uuid.isBlank()) {
                    continue;
                }

                String canonicalUuid = canonicalizeUuid(entry.uuid);
                String entryName = entry.name == null || entry.name.isBlank() ? "unknown" : entry.name;

                WhitelistEntry normalizedEntry = new WhitelistEntry();
                normalizedEntry.uuid = canonicalUuid;
                normalizedEntry.name = entryName;
                whitelistEntries.put(canonicalUuid, normalizedEntry);
                whitelistedUuids.add(canonicalUuid);
                whitelistedUuids.add(stripDashes(canonicalUuid));
            }

            logger.info("Loaded {} whitelist entries", whitelistEntries.size());
        } catch (Exception e) {
            logger.error("Failed to load whitelist", e);
        }
    }

    public synchronized void saveWhitelist() {
        try {
            List<WhitelistEntry> entries = new ArrayList<>(whitelistEntries.values());
            entries.sort(Comparator.comparing(entry -> entry.name == null ? "" : entry.name.toLowerCase(Locale.ROOT)));
            writeFileAtomically(whitelistFile, gson.toJson(entries));
        } catch (Exception e) {
            logger.error("Failed to save whitelist", e);
        }
    }

    private synchronized void loadPendingPlayers() {
        pendingBedrockPlayers.clear();
        try {
            if (!Files.exists(pendingFile)) {
                writeFileAtomically(pendingFile, gson.toJson(Map.of("entries", List.of())));
                return;
            }

            String content = Files.readString(pendingFile).trim();
            if (content.isEmpty()) {
                return;
            }

            JsonElement root = JsonParser.parseString(content);
            JsonElement entriesElement = root;
            if (root.isJsonObject()) {
                entriesElement = root.getAsJsonObject().get("entries");
            }

            List<PendingPlayer> entries = entriesElement == null || entriesElement.isJsonNull()
                ? List.of()
                : gson.fromJson(entriesElement, pendingType);

            if (entries == null) {
                return;
            }

            for (PendingPlayer pendingPlayer : entries) {
                if (pendingPlayer == null || pendingPlayer.name == null || pendingPlayer.name.isBlank()) {
                    continue;
                }

                if (pendingPlayer.floodgateUuid != null && !pendingPlayer.floodgateUuid.isBlank()) {
                    pendingPlayer.floodgateUuid = canonicalizeUuid(pendingPlayer.floodgateUuid);
                }
                pendingBedrockPlayers.put(normalizePendingKey(pendingPlayer.name), pendingPlayer);
            }

            logger.info("Loaded {} pending Bedrock players", pendingBedrockPlayers.size());
        } catch (Exception e) {
            logger.error("Failed to load pending players", e);
        }
    }

    private synchronized void savePendingPlayers() {
        try {
            List<PendingPlayer> entries = new ArrayList<>(pendingBedrockPlayers.values());
            entries.sort(Comparator.comparing(entry -> entry.name == null ? "" : entry.name.toLowerCase(Locale.ROOT)));
            writeFileAtomically(pendingFile, gson.toJson(Map.of("entries", entries)));
        } catch (Exception e) {
            logger.error("Failed to save pending players", e);
        }
    }

    private synchronized void loadOpenModeState() {
        try {
            if (!Files.exists(openModeFile)) {
                saveOpenModeState();
                return;
            }

            String content = Files.readString(openModeFile).trim();
            if (content.isEmpty()) {
                openModeState = OpenModeState.disabled();
                saveOpenModeState();
                return;
            }

            OpenModeState loaded = gson.fromJson(content, OpenModeState.class);
            openModeState = loaded != null ? loaded : OpenModeState.disabled();
        } catch (Exception e) {
            logger.error("Failed to load open mode state", e);
            openModeState = OpenModeState.disabled();
        }
    }

    private synchronized void loadBlockedPlayers() {
        blockedPlayers.clear();
        try {
            if (!Files.exists(blockedFile)) {
                writeFileAtomically(blockedFile, "[]");
                return;
            }

            String content = Files.readString(blockedFile).trim();
            if (content.isEmpty()) {
                return;
            }

            List<BlockedPlayer> entries = gson.fromJson(content, blockedType);
            if (entries == null) {
                return;
            }

            for (BlockedPlayer blockedPlayer : entries) {
                if (blockedPlayer == null || blockedPlayer.uuid == null || blockedPlayer.uuid.isBlank()) {
                    continue;
                }
                blockedPlayer.uuid = canonicalizeUuid(blockedPlayer.uuid);
                blockedPlayers.put(blockedPlayer.uuid, blockedPlayer);
            }
        } catch (Exception e) {
            logger.error("Failed to load blocked players", e);
        }
    }

    private synchronized void saveBlockedPlayers() {
        try {
            List<BlockedPlayer> entries = new ArrayList<>(blockedPlayers.values());
            entries.sort(Comparator.comparing(entry -> entry.name == null ? "" : entry.name.toLowerCase(Locale.ROOT)));
            writeFileAtomically(blockedFile, gson.toJson(entries));
        } catch (Exception e) {
            logger.error("Failed to save blocked players", e);
        }
    }

    private synchronized void saveOpenModeState() {
        try {
            writeFileAtomically(openModeFile, gson.toJson(openModeState));
        } catch (Exception e) {
            logger.error("Failed to save open mode state", e);
        }
    }

    public synchronized void setOpenMode(boolean enabled, String updatedBy) {
        OpenModeState nextState = new OpenModeState();
        nextState.enabled = enabled;
        nextState.updatedAt = System.currentTimeMillis();
        nextState.updatedBy = updatedBy == null || updatedBy.isBlank() ? "api" : updatedBy;
        openModeState = nextState;
        saveOpenModeState();
        logger.info("Open mode {}", enabled ? "enabled" : "disabled");
    }

    public OpenModeState getOpenModeState() {
        return openModeState;
    }

    public boolean isOpenModeEnabled() {
        return openModeState.enabled;
    }

    public boolean isHybridAuthMode() {
        return hybridAuthMode;
    }

    public int getWhitelistEntryCount() {
        return whitelistEntries.size();
    }

    public Map<String, PendingPlayer> getPendingBedrockPlayers() {
        return pendingBedrockPlayers;
    }

    public synchronized void approvePlayer(String name) {
        PendingPlayer pending = removePendingPlayer(name);
        if (pending == null) {
            logger.warn("Pending player not found for approval: {}", name);
            return;
        }

        addToWhitelist(pending.floodgateUuid, pending.name);
        savePendingPlayers();
        logger.info("Approved player: {}", pending.name);
    }

    public synchronized void addToWhitelist(String uuid, String name) {
        String canonicalUuid = canonicalizeUuid(uuid);
        WhitelistEntry entry = whitelistEntries.computeIfAbsent(canonicalUuid, ignored -> new WhitelistEntry());
        entry.uuid = canonicalUuid;
        entry.name = name == null || name.isBlank() ? "unknown" : name;
        blockedPlayers.remove(canonicalUuid);
        whitelistedUuids.add(canonicalUuid);
        whitelistedUuids.add(stripDashes(canonicalUuid));
        saveWhitelist();
        saveBlockedPlayers();
        logger.info("Added {} ({}) to whitelist", entry.name, canonicalUuid);
    }

    public synchronized void setPlayerActive(String uuid, String name, boolean active) {
        String canonicalUuid = canonicalizeUuid(uuid);
        if (active) {
            blockedPlayers.remove(canonicalUuid);
            saveBlockedPlayers();
            addToWhitelist(canonicalUuid, name);
            logger.info("Reactivated player {} ({})", name, canonicalUuid);
            return;
        }

        removeFromWhitelist(canonicalUuid);

        BlockedPlayer blockedPlayer = new BlockedPlayer();
        blockedPlayer.uuid = canonicalUuid;
        blockedPlayer.name = name;
        blockedPlayer.blockedAt = System.currentTimeMillis();
        blockedPlayers.put(canonicalUuid, blockedPlayer);

        PendingPlayer removedPending = removePendingPlayer(name);
        saveBlockedPlayers();
        saveWhitelist();
        if (removedPending != null) {
            savePendingPlayers();
        }
        logger.info("Deactivated player {} ({})", name, canonicalUuid);
    }

    public ProxyServer getServer() {
        return server;
    }

    public Logger getLogger() {
        return logger;
    }

    public Set<String> getWhitelistedUuids() {
        return whitelistedUuids;
    }

    public Map<String, BlockedPlayer> getBlockedPlayers() {
        return blockedPlayers;
    }

    private WhitelistEntry findWhitelistEntryByName(String name) {
        if (name == null || name.isBlank()) {
            return null;
        }

        for (WhitelistEntry entry : whitelistEntries.values()) {
            if (entry.name != null && entry.name.equalsIgnoreCase(name)) {
                return entry;
            }
        }

        return null;
    }

    private BlockedPlayer findBlockedPlayerByName(String name) {
        if (name == null || name.isBlank()) {
            return null;
        }

        for (BlockedPlayer entry : blockedPlayers.values()) {
            if (entry.name != null && entry.name.equalsIgnoreCase(name)) {
                return entry;
            }
        }

        return null;
    }

    private synchronized void migrateWhitelistIdentity(WhitelistEntry entry, String previousUuid, String newUuid, String name) {
        whitelistEntries.remove(previousUuid);
        whitelistedUuids.remove(previousUuid);
        whitelistedUuids.remove(stripDashes(previousUuid));

        entry.uuid = newUuid;
        entry.name = name;
        whitelistEntries.put(newUuid, entry);
        whitelistedUuids.add(newUuid);
        whitelistedUuids.add(stripDashes(newUuid));
        saveWhitelist();
        logger.info("Migrated whitelist UUID for {} from {} to {}", name, previousUuid, newUuid);
    }

    private synchronized void migrateBlockedIdentity(BlockedPlayer blockedPlayer, String previousUuid, String newUuid, String name) {
        blockedPlayers.remove(previousUuid);
        blockedPlayer.uuid = newUuid;
        blockedPlayer.name = name;
        blockedPlayers.put(newUuid, blockedPlayer);
        saveBlockedPlayers();
        logger.info("Migrated blocked UUID for {} from {} to {}", name, previousUuid, newUuid);
    }

    private PendingPlayer removePendingPlayer(String name) {
        if (name == null || name.isBlank()) {
            return null;
        }

        String normalized = normalizePendingKey(name);
        PendingPlayer pending = pendingBedrockPlayers.remove(normalized);
        if (pending == null && !normalized.startsWith(".")) {
            pending = pendingBedrockPlayers.remove("." + normalized);
        }
        if (pending == null && normalized.startsWith(".")) {
            pending = pendingBedrockPlayers.remove(normalized.substring(1));
        }
        return pending;
    }

    private String normalizePendingKey(String name) {
        return name.toLowerCase(Locale.ROOT);
    }

    private void removeFromWhitelist(String uuid) {
        String canonicalUuid = canonicalizeUuid(uuid);
        whitelistEntries.remove(canonicalUuid);
        whitelistedUuids.remove(canonicalUuid);
        whitelistedUuids.remove(stripDashes(canonicalUuid));
    }

    private String canonicalizeUuid(String uuid) {
        if (uuid == null || uuid.isBlank()) {
            throw new IllegalArgumentException("UUID is required");
        }

        String compact = stripDashes(uuid);
        if (compact.length() != 32) {
            throw new IllegalArgumentException("Invalid UUID: " + uuid);
        }

        return UUID.fromString(
            compact.substring(0, 8) + "-" +
            compact.substring(8, 12) + "-" +
            compact.substring(12, 16) + "-" +
            compact.substring(16, 20) + "-" +
            compact.substring(20, 32)
        ).toString();
    }

    private String stripDashes(String uuid) {
        return uuid.replace("-", "").toLowerCase(Locale.ROOT);
    }

    private void writeFileAtomically(Path target, String content) throws IOException {
        Path parent = target.getParent();
        if (parent != null) {
            Files.createDirectories(parent);
        }

        Path tempFile = Files.createTempFile(parent, target.getFileName().toString(), ".tmp");
        try {
            Files.writeString(tempFile, content);
            Files.move(tempFile, target, StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE);
        } catch (IOException ex) {
            Files.deleteIfExists(tempFile);
            throw ex;
        }
    }

    public static class WhitelistEntry {
        public String uuid;
        public String name;
    }

    public static class PendingPlayer {
        public String name;
        public String xuid;
        public String floodgateUuid;
        public long capturedAt;
    }

    public static class BlockedPlayer {
        public String uuid;
        public String name;
        public long blockedAt;
    }

    public static class OpenModeState {
        public boolean enabled;
        public long updatedAt;
        public String updatedBy;

        public static OpenModeState disabled() {
            OpenModeState state = new OpenModeState();
            state.enabled = false;
            state.updatedAt = 0L;
            state.updatedBy = "system";
            return state;
        }
    }
}
