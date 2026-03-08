package com.minecraft.auth;

import com.google.gson.Gson;
import com.google.gson.GsonBuilder;
import com.google.gson.reflect.TypeToken;
import net.kyori.adventure.text.Component;
import net.kyori.adventure.text.format.NamedTextColor;
import net.kyori.adventure.title.Title;
import org.bukkit.Bukkit;
import org.bukkit.Location;
import org.bukkit.command.Command;
import org.bukkit.command.CommandExecutor;
import org.bukkit.command.CommandSender;
import org.bukkit.entity.Entity;
import org.bukkit.entity.Player;
import org.bukkit.event.EventHandler;
import org.bukkit.event.EventPriority;
import org.bukkit.event.Listener;
import org.bukkit.event.block.BlockBreakEvent;
import org.bukkit.event.block.BlockPlaceEvent;
import org.bukkit.event.entity.EntityDamageByEntityEvent;
import org.bukkit.event.inventory.InventoryOpenEvent;
import org.bukkit.event.player.AsyncPlayerChatEvent;
import org.bukkit.event.player.PlayerCommandPreprocessEvent;
import org.bukkit.event.player.PlayerDropItemEvent;
import org.bukkit.event.player.PlayerInteractEvent;
import org.bukkit.event.player.PlayerJoinEvent;
import org.bukkit.event.player.PlayerMoveEvent;
import org.bukkit.event.player.PlayerQuitEvent;
import org.bukkit.plugin.Plugin;
import org.bukkit.plugin.java.JavaPlugin;
import org.mindrot.jbcrypt.BCrypt;

import java.io.IOException;
import java.io.Writer;
import java.lang.reflect.Method;
import java.lang.reflect.Type;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.Objects;
import java.util.Set;
import java.util.UUID;
import java.util.concurrent.ConcurrentHashMap;

public final class OptionalAuthGuardPlugin extends JavaPlugin implements Listener, CommandExecutor {

    private static final int STATUS_RETRY_TICKS = 20;
    private static final int STATUS_MAX_RETRIES = 6;
    private static final int MIN_PASSWORD_LENGTH = 6;
    private static final int MAX_PASSWORD_LENGTH = 72;
    private static final Set<String> BEDROCK_PREFIXES = Set.of(".");
    private static final Set<String> ALLOWED_PENDING_COMMANDS = Set.of("/login");

    private final Gson gson = new GsonBuilder().setPrettyPrinting().create();
    private final Type passwordRecordListType = new TypeToken<List<PasswordRecord>>() {}.getType();
    private final Map<String, PasswordRecord> passwordRecords = new ConcurrentHashMap<>();
    private final Map<UUID, PendingLogin> pendingLogins = new ConcurrentHashMap<>();

    private Path passwordFile;
    private volatile boolean warnedFastLoginMissing;

    @Override
    public void onEnable() {
        passwordFile = getDataFolder().toPath().resolve("passwords.json");
        loadPasswords();
        getServer().getPluginManager().registerEvents(this, this);
        Objects.requireNonNull(getCommand("register"), "register command").setExecutor(this);
        Objects.requireNonNull(getCommand("login"), "login command").setExecutor(this);
        Objects.requireNonNull(getCommand("changepassword"), "changepassword command").setExecutor(this);
    }

    @Override
    public boolean onCommand(CommandSender sender, Command command, String label, String[] args) {
        if (!(sender instanceof Player player)) {
            sender.sendMessage("Only players can use this command.");
            return true;
        }

        String normalizedName = normalizeName(player.getName());
        PasswordRecord record = passwordRecords.get(normalizedName);
        JavaIdentity identity = classifyPlayer(player);

        switch (command.getName().toLowerCase(Locale.ROOT)) {
            case "register":
                return handleRegister(player, identity, record, args);
            case "login":
                return handleLogin(player, record, args);
            case "changepassword":
                return handleChangePassword(player, identity, record, args);
            default:
                return false;
        }
    }

    @EventHandler(priority = EventPriority.MONITOR)
    public void onPlayerJoin(PlayerJoinEvent event) {
        Player player = event.getPlayer();
        pendingLogins.remove(player.getUniqueId());

        if (isBedrockPlayer(player)) {
            return;
        }

        scheduleStatusCheck(player.getUniqueId(), 0);
    }

    @EventHandler
    public void onPlayerQuit(PlayerQuitEvent event) {
        pendingLogins.remove(event.getPlayer().getUniqueId());
    }

    @EventHandler(ignoreCancelled = true)
    public void onPlayerMove(PlayerMoveEvent event) {
        if (!requiresLogin(event.getPlayer())) {
            return;
        }

        Location from = event.getFrom();
        Location to = event.getTo();
        if (to == null) {
            return;
        }

        if (from.getBlockX() != to.getBlockX()
            || from.getBlockY() != to.getBlockY()
            || from.getBlockZ() != to.getBlockZ()
            || !from.getWorld().equals(to.getWorld())) {
            event.setTo(from);
        }
    }

    @EventHandler(ignoreCancelled = true)
    public void onPlayerCommandPreprocess(PlayerCommandPreprocessEvent event) {
        if (!requiresLogin(event.getPlayer())) {
            return;
        }

        String command = event.getMessage().split("\\s+", 2)[0].toLowerCase(Locale.ROOT);
        if (ALLOWED_PENDING_COMMANDS.contains(command)) {
            return;
        }

        event.setCancelled(true);
        event.getPlayer().sendMessage(Component.text("Use /login <password> before playing.", NamedTextColor.RED));
    }

    @EventHandler(ignoreCancelled = true)
    public void onPlayerChat(AsyncPlayerChatEvent event) {
        if (!requiresLogin(event.getPlayer())) {
            return;
        }

        event.setCancelled(true);
        event.getPlayer().sendMessage(Component.text("Chat is blocked until you log in with /login <password>.", NamedTextColor.RED));
    }

    @EventHandler(ignoreCancelled = true)
    public void onPlayerInteract(PlayerInteractEvent event) {
        if (requiresLogin(event.getPlayer())) {
            event.setCancelled(true);
        }
    }

    @EventHandler(ignoreCancelled = true)
    public void onBlockBreak(BlockBreakEvent event) {
        if (requiresLogin(event.getPlayer())) {
            event.setCancelled(true);
        }
    }

    @EventHandler(ignoreCancelled = true)
    public void onBlockPlace(BlockPlaceEvent event) {
        if (requiresLogin(event.getPlayer())) {
            event.setCancelled(true);
        }
    }

    @EventHandler(ignoreCancelled = true)
    public void onInventoryOpen(InventoryOpenEvent event) {
        if (event.getPlayer() instanceof Player player && requiresLogin(player)) {
            event.setCancelled(true);
        }
    }

    @EventHandler(ignoreCancelled = true)
    public void onPlayerDropItem(PlayerDropItemEvent event) {
        if (requiresLogin(event.getPlayer())) {
            event.setCancelled(true);
        }
    }

    @EventHandler(ignoreCancelled = true)
    public void onEntityDamage(EntityDamageByEntityEvent event) {
        Entity damager = event.getDamager();
        Entity entity = event.getEntity();
        if (damager instanceof Player player && requiresLogin(player)) {
            event.setCancelled(true);
            return;
        }
        if (entity instanceof Player player && requiresLogin(player)) {
            event.setCancelled(true);
        }
    }

    private boolean handleRegister(Player player, JavaIdentity identity, PasswordRecord record, String[] args) {
        if (identity == JavaIdentity.PREMIUM || identity == JavaIdentity.BEDROCK) {
            player.sendMessage(Component.text("Only cracked Java players need optional password protection here.", NamedTextColor.YELLOW));
            return true;
        }

        if (record != null && record.enabled) {
            player.sendMessage(Component.text("You already have a password. Use /changepassword <old> <new> instead.", NamedTextColor.RED));
            return true;
        }

        if (args.length < 2) {
            player.sendMessage(Component.text("Usage: /register <password> <confirm>", NamedTextColor.RED));
            return true;
        }

        String password = args[0];
        String confirm = args[1];
        if (!password.equals(confirm)) {
            player.sendMessage(Component.text("Passwords do not match.", NamedTextColor.RED));
            return true;
        }

        String validationError = validatePassword(password);
        if (validationError != null) {
            player.sendMessage(Component.text(validationError, NamedTextColor.RED));
            return true;
        }

        PasswordRecord newRecord = new PasswordRecord();
        newRecord.name = player.getName();
        newRecord.passwordHash = BCrypt.hashpw(password, BCrypt.gensalt(10));
        newRecord.enabled = true;
        newRecord.updatedAt = Instant.now().toString();
        passwordRecords.put(normalizeName(player.getName()), newRecord);
        savePasswords();

        pendingLogins.remove(player.getUniqueId());
        player.sendMessage(Component.text("Password protection enabled. Future cracked logins will require /login <password>.", NamedTextColor.GREEN));
        player.showTitle(Title.title(
            Component.text("Password enabled", NamedTextColor.GREEN),
            Component.text("Future cracked logins will require /login", NamedTextColor.YELLOW),
            Title.Times.times(Duration.ofMillis(250), Duration.ofSeconds(3), Duration.ofMillis(500))
        ));
        return true;
    }

    private boolean handleLogin(Player player, PasswordRecord record, String[] args) {
        if (!requiresLogin(player)) {
            player.sendMessage(Component.text("You do not need to log in right now.", NamedTextColor.YELLOW));
            return true;
        }

        if (record == null || !record.enabled) {
            pendingLogins.remove(player.getUniqueId());
            player.sendMessage(Component.text("This account does not have a password yet. Use /register <password> <confirm> if you want protection.", NamedTextColor.YELLOW));
            return true;
        }

        if (args.length < 1) {
            player.sendMessage(Component.text("Usage: /login <password>", NamedTextColor.RED));
            return true;
        }

        if (!BCrypt.checkpw(args[0], record.passwordHash)) {
            player.sendMessage(Component.text("Incorrect password.", NamedTextColor.RED));
            return true;
        }

        pendingLogins.remove(player.getUniqueId());
        player.sendMessage(Component.text("Login successful. Have fun!", NamedTextColor.GREEN));
        player.showTitle(Title.title(
            Component.text("Logged in", NamedTextColor.GREEN),
            Component.text("You can play now.", NamedTextColor.YELLOW),
            Title.Times.times(Duration.ofMillis(150), Duration.ofSeconds(2), Duration.ofMillis(400))
        ));
        return true;
    }

    private boolean handleChangePassword(Player player, JavaIdentity identity, PasswordRecord record, String[] args) {
        if (identity == JavaIdentity.PREMIUM || identity == JavaIdentity.BEDROCK) {
            player.sendMessage(Component.text("Only cracked Java players use this password flow.", NamedTextColor.YELLOW));
            return true;
        }

        if (record == null || !record.enabled) {
            player.sendMessage(Component.text("No password is set yet. Use /register <password> <confirm> first.", NamedTextColor.RED));
            return true;
        }

        if (args.length < 2) {
            player.sendMessage(Component.text("Usage: /changepassword <oldPassword> <newPassword>", NamedTextColor.RED));
            return true;
        }

        if (!BCrypt.checkpw(args[0], record.passwordHash)) {
            player.sendMessage(Component.text("Incorrect current password.", NamedTextColor.RED));
            return true;
        }

        String validationError = validatePassword(args[1]);
        if (validationError != null) {
            player.sendMessage(Component.text(validationError, NamedTextColor.RED));
            return true;
        }

        record.passwordHash = BCrypt.hashpw(args[1], BCrypt.gensalt(10));
        record.updatedAt = Instant.now().toString();
        passwordRecords.put(normalizeName(player.getName()), record);
        savePasswords();
        pendingLogins.remove(player.getUniqueId());
        player.sendMessage(Component.text("Password changed successfully.", NamedTextColor.GREEN));
        return true;
    }

    private void scheduleStatusCheck(UUID playerId, int attempt) {
        Bukkit.getScheduler().runTaskLater(this, () -> {
            Player player = Bukkit.getPlayer(playerId);
            if (player == null || !player.isOnline()) {
                return;
            }

            if (isBedrockPlayer(player)) {
                pendingLogins.remove(player.getUniqueId());
                return;
            }

            JavaIdentity identity = classifyPlayer(player);
            if (identity == JavaIdentity.UNKNOWN && attempt < STATUS_MAX_RETRIES) {
                scheduleStatusCheck(playerId, attempt + 1);
                return;
            }

            if (identity == JavaIdentity.PREMIUM) {
                pendingLogins.remove(player.getUniqueId());
                return;
            }

            PasswordRecord record = passwordRecords.get(normalizeName(player.getName()));
            if (record != null && record.enabled) {
                PendingLogin pending = new PendingLogin();
                pending.name = player.getName();
                pendingLogins.put(player.getUniqueId(), pending);
                sendProtectedPrompt(player);
                return;
            }

            pendingLogins.remove(player.getUniqueId());
            sendUnprotectedWarning(player);
        }, STATUS_RETRY_TICKS);
    }

    private void sendProtectedPrompt(Player player) {
        player.sendMessage(Component.text("This cracked Java account is protected. Use /login <password> to continue.", NamedTextColor.RED));
        player.showTitle(Title.title(
            Component.text("Login required", NamedTextColor.RED),
            Component.text("/login <password>", NamedTextColor.YELLOW),
            Title.Times.times(Duration.ofMillis(200), Duration.ofSeconds(4), Duration.ofMillis(500))
        ));
    }

    private void sendUnprotectedWarning(Player player) {
        player.sendMessage(Component.text("Warning: this cracked Java name is not protected. Anyone can impersonate you until you set a password.", NamedTextColor.GOLD));
        player.sendMessage(Component.text("Use /register <password> <confirm> if you want future cracked logins to require a password.", NamedTextColor.YELLOW));
        player.showTitle(Title.title(
            Component.text("Account unprotected", NamedTextColor.GOLD),
            Component.text("Use /register <password> <confirm>", NamedTextColor.YELLOW),
            Title.Times.times(Duration.ofMillis(200), Duration.ofSeconds(4), Duration.ofMillis(500))
        ));
    }

    private boolean requiresLogin(Player player) {
        return pendingLogins.containsKey(player.getUniqueId());
    }

    private JavaIdentity classifyPlayer(Player player) {
        if (isBedrockPlayer(player)) {
            return JavaIdentity.BEDROCK;
        }

        Plugin fastLogin = getServer().getPluginManager().getPlugin("FastLogin");
        if (fastLogin == null) {
            if (!warnedFastLoginMissing) {
                warnedFastLoginMissing = true;
                getLogger().warning("FastLogin is not installed on the Paper backend. Premium detection will fall back to cracked mode.");
            }
            return JavaIdentity.UNKNOWN;
        }

        try {
            Method getStatus = fastLogin.getClass().getMethod("getStatus", UUID.class);
            Object result = getStatus.invoke(fastLogin, player.getUniqueId());
            if (result == null) {
                return JavaIdentity.UNKNOWN;
            }

            String status = result.toString();
            if ("PREMIUM".equalsIgnoreCase(status)) {
                return JavaIdentity.PREMIUM;
            }
            if ("CRACKED".equalsIgnoreCase(status)) {
                return JavaIdentity.CRACKED;
            }
            return JavaIdentity.UNKNOWN;
        } catch (ReflectiveOperationException exception) {
            getLogger().warning("Failed to query FastLogin premium status: " + exception.getMessage());
            return JavaIdentity.UNKNOWN;
        }
    }

    private boolean isBedrockPlayer(Player player) {
        String name = player.getName();
        for (String prefix : BEDROCK_PREFIXES) {
            if (name.startsWith(prefix)) {
                return true;
            }
        }
        return false;
    }

    private String validatePassword(String password) {
        if (password.length() < MIN_PASSWORD_LENGTH) {
            return "Password must be at least " + MIN_PASSWORD_LENGTH + " characters.";
        }
        if (password.length() > MAX_PASSWORD_LENGTH) {
            return "Password must be at most " + MAX_PASSWORD_LENGTH + " characters.";
        }
        return null;
    }

    private void loadPasswords() {
        passwordRecords.clear();
        try {
            Files.createDirectories(passwordFile.getParent());
            if (!Files.exists(passwordFile)) {
                writePasswords(List.of());
                return;
            }

            String content = Files.readString(passwordFile).trim();
            if (content.isEmpty()) {
                return;
            }

            List<PasswordRecord> loaded = gson.fromJson(content, passwordRecordListType);
            if (loaded == null) {
                return;
            }

            for (PasswordRecord record : loaded) {
                if (record == null || record.name == null || record.name.isBlank()) {
                    continue;
                }
                passwordRecords.put(normalizeName(record.name), record);
            }
        } catch (IOException exception) {
            getLogger().severe("Failed to load password records: " + exception.getMessage());
        }
    }

    private void savePasswords() {
        List<PasswordRecord> records = new ArrayList<>(passwordRecords.values());
        records.sort(Comparator.comparing(record -> normalizeName(record.name)));
        try {
            writePasswords(records);
        } catch (IOException exception) {
            getLogger().severe("Failed to save password records: " + exception.getMessage());
        }
    }

    private void writePasswords(List<PasswordRecord> records) throws IOException {
        Files.createDirectories(passwordFile.getParent());
        Path tempFile = Files.createTempFile(passwordFile.getParent(), "passwords", ".tmp");
        try {
            try (Writer writer = Files.newBufferedWriter(tempFile)) {
                gson.toJson(records, writer);
            }
            Files.move(tempFile, passwordFile, StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE);
        } finally {
            Files.deleteIfExists(tempFile);
        }
    }

    private String normalizeName(String name) {
        return name.toLowerCase(Locale.ROOT);
    }

    private enum JavaIdentity {
        PREMIUM,
        CRACKED,
        BEDROCK,
        UNKNOWN
    }

    private static final class PendingLogin {
        String name;
    }

    private static final class PasswordRecord {
        String name;
        String passwordHash;
        boolean enabled;
        String updatedAt;
    }
}
