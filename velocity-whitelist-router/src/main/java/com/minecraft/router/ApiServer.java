package com.minecraft.router;

import com.google.gson.Gson;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpHandler;
import com.sun.net.httpserver.HttpServer;
import org.slf4j.Logger;

import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.*;

public class ApiServer {

    private final WhitelistRouterPlugin plugin;
    private final Logger logger;
    private final Gson gson = new Gson();
    private HttpServer server;
    private final int port;
    private final String apiToken;

    public ApiServer(WhitelistRouterPlugin plugin, int port) {
        this.plugin = plugin;
        this.logger = plugin.getLogger();
        this.port = port;
        this.apiToken = System.getenv().getOrDefault("PROXY_API_TOKEN", "").trim();
    }

    public void start() throws IOException {
        server = HttpServer.create(new InetSocketAddress(port), 0);

        // API endpoints
        server.createContext("/api/pending", new PendingHandler());
        server.createContext("/api/blocked", new BlockedHandler());
        server.createContext("/api/whitelist", new WhitelistHandler());
        server.createContext("/api/approve", new ApproveHandler());
        server.createContext("/api/access", new AccessHandler());
        server.createContext("/api/open-mode", new OpenModeHandler());
        server.createContext("/api/status", new StatusHandler());

        server.setExecutor(null);
        server.start();
        logger.info("API server started on port {}", port);
    }

    public void stop() {
        if (server != null) {
            server.stop(0);
            logger.info("API server stopped");
        }
    }

    abstract class BaseHandler implements HttpHandler {
        @Override
        public void handle(HttpExchange exchange) throws IOException {
            String method = exchange.getRequestMethod();
            String response;

            try {
                // Add CORS headers
                exchange.getResponseHeaders().add("Access-Control-Allow-Origin", "*");
                exchange.getResponseHeaders().add("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS");
                exchange.getResponseHeaders().add("Access-Control-Allow-Headers", "Content-Type");

                if ("OPTIONS".equals(method)) {
                    exchange.sendResponseHeaders(200, -1);
                    return;
                }

                if (requiresAuth(exchange)) {
                    response = gson.toJson(Map.of("error", "Unauthorized"));
                    exchange.getResponseHeaders().set("Content-Type", "application/json");
                    exchange.sendResponseHeaders(401, response.getBytes(StandardCharsets.UTF_8).length);
                    try (OutputStream os = exchange.getResponseBody()) {
                        os.write(response.getBytes(StandardCharsets.UTF_8));
                    }
                    return;
                }

                response = handleRequest(exchange, method);
                exchange.getResponseHeaders().set("Content-Type", "application/json");
                exchange.sendResponseHeaders(200, response.getBytes(StandardCharsets.UTF_8).length);
            } catch (Exception e) {
                logger.error("API error", e);
                response = gson.toJson(Map.of("error", e.getMessage()));
                exchange.getResponseHeaders().set("Content-Type", "application/json");
                exchange.sendResponseHeaders(500, response.getBytes(StandardCharsets.UTF_8).length);
            }

            try (OutputStream os = exchange.getResponseBody()) {
                os.write(response.getBytes(StandardCharsets.UTF_8));
            }
        }

        protected String readBody(HttpExchange exchange) throws IOException {
            InputStream is = exchange.getRequestBody();
            return new String(is.readAllBytes(), StandardCharsets.UTF_8);
        }

        private boolean requiresAuth(HttpExchange exchange) {
            if (apiToken.isBlank()) {
                return false;
            }

            String authHeader = exchange.getRequestHeaders().getFirst("Authorization");
            if (authHeader == null || !authHeader.startsWith("Bearer ")) {
                return true;
            }

            String suppliedToken = authHeader.substring("Bearer ".length()).trim();
            return !apiToken.equals(suppliedToken);
        }

        abstract protected String handleRequest(HttpExchange exchange, String method) throws Exception;
    }

    class PendingHandler extends BaseHandler {
        @Override
        protected String handleRequest(HttpExchange exchange, String method) throws Exception {
            if ("GET".equals(method)) {
                List<Map<String, Object>> pending = new ArrayList<>();
                for (WhitelistRouterPlugin.PendingPlayer p : plugin.getPendingBedrockPlayers().values()) {
                    Map<String, Object> entry = new LinkedHashMap<>();
                    entry.put("name", p.name);
                    entry.put("xuid", p.xuid);
                    entry.put("floodgate_uuid", p.floodgateUuid);
                    entry.put("captured_at", p.capturedAt);
                    pending.add(entry);
                }
                return gson.toJson(Map.of("pending", pending));
            }
            return gson.toJson(Map.of("error", "Method not allowed"));
        }
    }

    class BlockedHandler extends BaseHandler {
        @Override
        protected String handleRequest(HttpExchange exchange, String method) throws Exception {
            if ("GET".equals(method)) {
                List<Map<String, Object>> blocked = new ArrayList<>();
                for (WhitelistRouterPlugin.BlockedPlayer p : plugin.getBlockedPlayers().values()) {
                    Map<String, Object> entry = new LinkedHashMap<>();
                    entry.put("uuid", p.uuid);
                    entry.put("name", p.name);
                    entry.put("blocked_at", p.blockedAt);
                    blocked.add(entry);
                }
                return gson.toJson(Map.of("blocked", blocked));
            }
            return gson.toJson(Map.of("error", "Method not allowed"));
        }
    }

    class WhitelistHandler extends BaseHandler {
        @Override
        protected String handleRequest(HttpExchange exchange, String method) throws Exception {
            if ("GET".equals(method)) {
                return gson.toJson(Map.of("count", plugin.getWhitelistedUuids().size()));
            } else if ("POST".equals(method)) {
                String body = readBody(exchange);
                Map<String, String> data = gson.fromJson(body, Map.class);
                String uuid = data.get("uuid");
                String name = data.getOrDefault("name", "unknown");
                if (uuid != null) {
                    plugin.addToWhitelist(uuid, name);
                    return gson.toJson(Map.of("success", true, "message", "Added " + name + " to whitelist"));
                }
                return gson.toJson(Map.of("error", "UUID required"));
            }
            return gson.toJson(Map.of("error", "Method not allowed"));
        }
    }

    class ApproveHandler extends BaseHandler {
        @Override
        protected String handleRequest(HttpExchange exchange, String method) throws Exception {
            if ("POST".equals(method)) {
                String body = readBody(exchange);
                Map<String, String> data = gson.fromJson(body, Map.class);
                String name = data.get("name");
                if (name != null) {
                    plugin.approvePlayer(name);
                    return gson.toJson(Map.of("success", true, "message", "Approved " + name));
                }
                return gson.toJson(Map.of("error", "Name required"));
            }
            return gson.toJson(Map.of("error", "Method not allowed"));
        }
    }

    class StatusHandler extends BaseHandler {
        @Override
        protected String handleRequest(HttpExchange exchange, String method) throws Exception {
            Map<String, Object> status = new LinkedHashMap<>();
            status.put("whitelisted_count", plugin.getWhitelistEntryCount());
            status.put("pending_count", plugin.getPendingBedrockPlayers().size());
            status.put("blocked_count", plugin.getBlockedPlayers().size());
            status.put("open_mode", plugin.isOpenModeEnabled());
            status.put("hybrid_auth_mode", plugin.isHybridAuthMode());
            status.put("main_server", plugin.getServer().getServer("main").isPresent());
            status.put("limbo_server", plugin.getServer().getServer("limbo").isPresent());
            return gson.toJson(status);
        }
    }

    class AccessHandler extends BaseHandler {
        @Override
        protected String handleRequest(HttpExchange exchange, String method) throws Exception {
            if (!"POST".equals(method)) {
                return gson.toJson(Map.of("error", "Method not allowed"));
            }

            String body = readBody(exchange);
            Map<String, Object> data = gson.fromJson(body, Map.class);
            String uuid = String.valueOf(data.get("uuid"));
            String name = String.valueOf(data.getOrDefault("name", "unknown"));
            Object activeValue = data.get("active");
            if (uuid == null || uuid.isBlank() || "null".equals(uuid) || activeValue == null) {
                return gson.toJson(Map.of("error", "uuid and active are required"));
            }

            boolean active = Boolean.parseBoolean(String.valueOf(activeValue));
            plugin.setPlayerActive(uuid, name, active);
            return gson.toJson(Map.of(
                "success", true,
                "uuid", uuid,
                "name", name,
                "active", active
            ));
        }
    }

    class OpenModeHandler extends BaseHandler {
        @Override
        protected String handleRequest(HttpExchange exchange, String method) throws Exception {
            if ("GET".equals(method)) {
                return gson.toJson(plugin.getOpenModeState());
            } else if ("POST".equals(method)) {
                String body = readBody(exchange);
                Map<String, Object> data = gson.fromJson(body, Map.class);
                Object enabledValue = data.get("enabled");
                if (enabledValue == null) {
                    return gson.toJson(Map.of("error", "enabled is required"));
                }

                boolean enabled = Boolean.parseBoolean(String.valueOf(enabledValue));
                String updatedBy = String.valueOf(data.getOrDefault("updatedBy", "api"));
                plugin.setOpenMode(enabled, updatedBy);
                return gson.toJson(Map.of(
                    "success", true,
                    "enabled", plugin.isOpenModeEnabled(),
                    "updatedAt", plugin.getOpenModeState().updatedAt,
                    "updatedBy", plugin.getOpenModeState().updatedBy
                ));
            }
            return gson.toJson(Map.of("error", "Method not allowed"));
        }
    }
}
