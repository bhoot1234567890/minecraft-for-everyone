package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ProxyClient handles communication with the Velocity proxy API
type ProxyClient struct {
	baseURL    string
	apiToken   string
	httpClient *http.Client
}

// PendingBedrockPlayer represents a pending Bedrock player from the proxy
type PendingBedrockPlayer struct {
	Name          string `json:"name"`
	XUID          string `json:"xuid"`
	FloodgateUUID string `json:"floodgate_uuid"`
	CapturedAt    int64  `json:"captured_at"`
}

type BlockedPlayer struct {
	UUID      string `json:"uuid"`
	Name      string `json:"name"`
	BlockedAt int64  `json:"blocked_at"`
}

// ProxyStatus represents the proxy status
type ProxyStatus struct {
	WhitelistedCount int  `json:"whitelisted_count"`
	PendingCount     int  `json:"pending_count"`
	BlockedCount     int  `json:"blocked_count"`
	OpenMode         bool `json:"open_mode"`
	HybridAuthMode   bool `json:"hybrid_auth_mode"`
	MainServer       bool `json:"main_server"`
	LimboServer      bool `json:"limbo_server"`
}

type OpenModeStatus struct {
	Enabled   bool   `json:"enabled"`
	UpdatedAt int64  `json:"updatedAt"`
	UpdatedBy string `json:"updatedBy"`
}

// NewProxyClient creates a new proxy client
func NewProxyClient() *ProxyClient {
	baseURL := os.Getenv("PROXY_API_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	return &ProxyClient{
		baseURL: baseURL,
		apiToken: os.Getenv("PROXY_API_TOKEN"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (pc *ProxyClient) newRequest(method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, pc.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if pc.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+pc.apiToken)
	}
	return req, nil
}

// GetPendingPlayers fetches pending Bedrock players from the proxy
func (pc *ProxyClient) GetPendingPlayers() ([]PendingBedrockPlayer, error) {
	req, err := pc.newRequest(http.MethodGet, "/api/pending", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build pending request: %w", err)
	}
	resp, err := pc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch pending players: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("proxy API returned status %d", resp.StatusCode)
	}

	var result struct {
		Pending []PendingBedrockPlayer `json:"pending"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Pending, nil
}

func (pc *ProxyClient) GetBlockedPlayers() ([]BlockedPlayer, error) {
	req, err := pc.newRequest(http.MethodGet, "/api/blocked", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build blocked request: %w", err)
	}
	resp, err := pc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch blocked players: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("proxy API returned status %d", resp.StatusCode)
	}

	var result struct {
		Blocked []BlockedPlayer `json:"blocked"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Blocked, nil
}

// ApprovePlayer approves a pending Bedrock player in the proxy
func (pc *ProxyClient) ApprovePlayer(name string) error {
	body := map[string]string{"name": name}
	jsonBody, _ := json.Marshal(body)

	req, err := pc.newRequest(http.MethodPost, "/api/approve", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to build approve request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := pc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to approve player: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("proxy API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// AddToWhitelist adds a player to the proxy whitelist
func (pc *ProxyClient) AddToWhitelist(uuid, name string) error {
	body := map[string]string{"uuid": uuid, "name": name}
	jsonBody, _ := json.Marshal(body)

	req, err := pc.newRequest(http.MethodPost, "/api/whitelist", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to build whitelist request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := pc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to add to whitelist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("proxy API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// GetStatus fetches the proxy status
func (pc *ProxyClient) GetStatus() (*ProxyStatus, error) {
	req, err := pc.newRequest(http.MethodGet, "/api/status", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build status request: %w", err)
	}
	resp, err := pc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("proxy API returned status %d", resp.StatusCode)
	}

	var status ProxyStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &status, nil
}

func (pc *ProxyClient) GetOpenMode() (*OpenModeStatus, error) {
	req, err := pc.newRequest(http.MethodGet, "/api/open-mode", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build open mode request: %w", err)
	}
	resp, err := pc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch open mode: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("proxy API returned status %d", resp.StatusCode)
	}

	var status OpenModeStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &status, nil
}

func (pc *ProxyClient) SetOpenMode(enabled bool, updatedBy string) (*OpenModeStatus, error) {
	body := map[string]any{
		"enabled":   enabled,
		"updatedBy": updatedBy,
	}
	jsonBody, _ := json.Marshal(body)

	req, err := pc.newRequest(http.MethodPost, "/api/open-mode", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to build open mode update request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := pc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update open mode: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("proxy API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result OpenModeStatus
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

func (pc *ProxyClient) SetPlayerActive(uuid, name string, active bool) error {
	body := map[string]any{
		"uuid":   uuid,
		"name":   name,
		"active": active,
	}
	jsonBody, _ := json.Marshal(body)

	req, err := pc.newRequest(http.MethodPost, "/api/access", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to build player access request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := pc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update player access: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("proxy API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
