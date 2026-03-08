package com.minecraft.router;

import com.velocitypowered.api.command.RawCommand;
import net.kyori.adventure.text.Component;
import net.kyori.adventure.text.format.NamedTextColor;

public class WhitelistRouterCommand implements RawCommand {

    private final WhitelistRouterPlugin plugin;

    public WhitelistRouterCommand(WhitelistRouterPlugin plugin) {
        this.plugin = plugin;
    }

    @Override
    public void execute(Invocation invocation) {
        String[] args = invocation.arguments().split("\\s+");

        if (args.length == 0 || args[0].isEmpty()) {
            sendHelp(invocation);
            return;
        }

        String subCommand = args[0].toLowerCase();

        switch (subCommand) {
            case "list":
                handleList(invocation);
                break;
            case "pending":
                handlePending(invocation);
                break;
            case "approve":
                handleApprove(invocation, args);
                break;
            case "add":
                handleAdd(invocation, args);
                break;
            case "reload":
                handleReload(invocation);
                break;
            case "status":
                handleStatus(invocation);
                break;
            case "openmode":
                handleOpenMode(invocation, args);
                break;
            default:
                sendHelp(invocation);
        }
    }

    private void sendHelp(Invocation invocation) {
        invocation.source().sendMessage(Component.text()
            .append(Component.text("=== WhitelistRouter Commands ===\n", NamedTextColor.GOLD))
            .append(Component.text("/wlr list - Show whitelisted players\n", NamedTextColor.YELLOW))
            .append(Component.text("/wlr pending - Show pending Bedrock players\n", NamedTextColor.YELLOW))
            .append(Component.text("/wlr approve <name> - Approve pending player\n", NamedTextColor.YELLOW))
            .append(Component.text("/wlr add <uuid> <name> - Add player to whitelist\n", NamedTextColor.YELLOW))
            .append(Component.text("/wlr reload - Reload whitelist\n", NamedTextColor.YELLOW))
            .append(Component.text("/wlr openmode <on|off> - Toggle open onboarding mode\n", NamedTextColor.YELLOW))
            .append(Component.text("/wlr status - Show plugin status\n", NamedTextColor.YELLOW))
            .build());
    }

    private void handleList(Invocation invocation) {
        invocation.source().sendMessage(Component.text("Whitelist entries: " + plugin.getWhitelistEntryCount(), NamedTextColor.GREEN));
    }

    private void handlePending(Invocation invocation) {
        var pending = plugin.getPendingBedrockPlayers();
        if (pending.isEmpty()) {
            invocation.source().sendMessage(Component.text("No pending Bedrock players", NamedTextColor.YELLOW));
            return;
        }

        invocation.source().sendMessage(Component.text("=== Pending Bedrock Players ===", NamedTextColor.GOLD));
        for (var entry : pending.values()) {
            String time = new java.text.SimpleDateFormat("yyyy-MM-dd HH:mm").format(new java.util.Date(entry.capturedAt));
            invocation.source().sendMessage(Component.text()
                .append(Component.text("• ", NamedTextColor.GRAY))
                .append(Component.text(entry.name, NamedTextColor.GREEN))
                .append(Component.text(" (XUID: " + entry.xuid + ") ", NamedTextColor.YELLOW))
                .append(Component.text("[" + time + "]", NamedTextColor.GRAY))
                .build());
        }
    }

    private void handleApprove(Invocation invocation, String[] args) {
        if (args.length < 2) {
            invocation.source().sendMessage(Component.text("Usage: /wlr approve <name>", NamedTextColor.RED));
            return;
        }

        String name = args[1];
        var pending = plugin.getPendingBedrockPlayers();
        String key = name.toLowerCase().replace(".", "");

        // Try to find with or without dot prefix
        WhitelistRouterPlugin.PendingPlayer player = pending.get(key);
        if (player == null) {
            player = pending.get("." + key);
        }
        if (player == null) {
            player = pending.get(name.toLowerCase());
        }

        if (player == null) {
            invocation.source().sendMessage(Component.text("Player not found in pending list: " + name, NamedTextColor.RED));
            return;
        }

        plugin.approvePlayer(player.name);
        invocation.source().sendMessage(Component.text("Approved player: " + player.name + " (XUID: " + player.xuid + ")", NamedTextColor.GREEN));
    }

    private void handleAdd(Invocation invocation, String[] args) {
        if (args.length < 3) {
            invocation.source().sendMessage(Component.text("Usage: /wlr add <uuid> <name>", NamedTextColor.RED));
            return;
        }

        String uuid = args[1];
        String name = args[2];

        plugin.addToWhitelist(uuid, name);
        invocation.source().sendMessage(Component.text("Added " + name + " to whitelist", NamedTextColor.GREEN));
    }

    private void handleReload(Invocation invocation) {
        plugin.reloadState();
        invocation.source().sendMessage(Component.text("State reloaded! " + plugin.getWhitelistEntryCount() + " whitelist entries", NamedTextColor.GREEN));
    }

    private void handleOpenMode(Invocation invocation, String[] args) {
        if (args.length < 2) {
            invocation.source().sendMessage(
                Component.text("Open mode is currently " + (plugin.isOpenModeEnabled() ? "ON" : "OFF"), NamedTextColor.YELLOW)
            );
            return;
        }

        String mode = args[1].toLowerCase();
        if (!mode.equals("on") && !mode.equals("off")) {
            invocation.source().sendMessage(Component.text("Usage: /wlr openmode <on|off>", NamedTextColor.RED));
            return;
        }

        boolean enabled = mode.equals("on");
        plugin.setOpenMode(enabled, "command");
        invocation.source().sendMessage(
            Component.text("Open mode " + (enabled ? "enabled" : "disabled"), enabled ? NamedTextColor.GREEN : NamedTextColor.YELLOW)
        );
    }

    private void handleStatus(Invocation invocation) {
        var main = plugin.getServer().getServer("main");
        var limbo = plugin.getServer().getServer("limbo");

        invocation.source().sendMessage(Component.text()
            .append(Component.text("=== WhitelistRouter Status ===\n", NamedTextColor.GOLD))
            .append(Component.text("Whitelisted: " + plugin.getWhitelistEntryCount() + "\n", NamedTextColor.GREEN))
            .append(Component.text("Pending Bedrock: " + plugin.getPendingBedrockPlayers().size() + "\n", NamedTextColor.YELLOW))
            .append(Component.text("Open Mode: " + (plugin.isOpenModeEnabled() ? "ON" : "OFF") + "\n", plugin.isOpenModeEnabled() ? NamedTextColor.GREEN : NamedTextColor.YELLOW))
            .append(Component.text("Main Server: " + (main.isPresent() ? "✓" : "✗") + "\n", main.isPresent() ? NamedTextColor.GREEN : NamedTextColor.RED))
            .append(Component.text("Limbo Server: " + (limbo.isPresent() ? "✓" : "✗"), limbo.isPresent() ? NamedTextColor.GREEN : NamedTextColor.RED))
            .build());
    }

    @Override
    public boolean hasPermission(Invocation invocation) {
        return invocation.source().hasPermission("whitelistrouter.admin");
    }
}
