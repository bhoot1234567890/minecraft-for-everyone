package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
)

type Player struct {
	UUID         string    `json:"uuid"`
	Name         string    `json:"name"`
	LastSeen     time.Time `json:"lastSeen"`
	Status       string    `json:"status"`    // "online" or "offline"
	Rank         string    `json:"rank"`      // "Administrator", "Moderator", "Member"
	Active       bool      `json:"active"`    // true = whitelisted, false = deactivated
	AvatarURL    string    `json:"avatarUrl"`
	BedrockUUID  string    `json:"bedrockUuid,omitempty"`  // Floodgate UUID for Bedrock players
	Platform     string    `json:"platform"`    // "java", "bedrock", or "both"
	LinkedName   string    `json:"linkedName,omitempty"`   // Linked Java username for Bedrock players
}

type Ban struct {
	UUID      string    `json:"uuid"`
	Name      string    `json:"name"`
	Reason    string    `json:"reason"`
	BannedAt  time.Time `json:"bannedAt"`
	BannedBy  string    `json:"bannedBy"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type ServerStats struct {
	TotalWhitelisted int       `json:"totalWhitelisted"`
	ActiveBans       int       `json:"activeBans"`
	OnlinePlayers    int       `json:"onlinePlayers"`
	MaxPlayers       int       `json:"maxPlayers"`
	QueueCapacity    float64   `json:"queueCapacity"`
}

type PendingPlayer struct {
	Name        string    `json:"name"`
	AddedAt     time.Time `json:"addedAt"`
	AddedBy     string    `json:"addedBy"`
	Platform    string    `json:"platform"` // "bedrock" or "java"
	Status      string    `json:"status"` // "pending", "registering", "registered", "failed"
	FloodgateUUID string  `json:"floodgateUuid,omitempty"`
}

type WhitelistManager struct {
	filepath        string
	bansFile        string
	pendingFile     string
	players         []Player
	bans            []Ban
	pendingPlayers  []PendingPlayer
	onlineCache     map[string]bool
	mu              sync.RWMutex
	registrationActive bool
	registrationTarget string
}

type AuthConfig struct {
	Username string
	Password string
}

type Session struct {
	Token     string
	ExpiresAt time.Time
}

type PaginatedResponse struct {
	Data       []Player `json:"data"`
	Total      int      `json:"total"`
	Page       int      `json:"page"`
	PageSize   int      `json:"pageSize"`
	TotalPages int      `json:"totalPages"`
}

var (
	wm                  *WhitelistManager
	sessions            = make(map[string]Session)
	authMu              sync.RWMutex
	authCfg             AuthConfig
	sessionCookieSecure bool
	corsAllowOrigins    string
	dashboardLogPath    string
)


func NewWhitelistManager(filepath, bansFile, pendingFile string) *WhitelistManager {
	log.Printf("Initializing WhitelistManager with filepath: %s", filepath)
	wm := &WhitelistManager{
		filepath:           filepath,
		bansFile:           bansFile,
		pendingFile:        pendingFile,
		players:            []Player{},
		bans:               []Ban{},
		pendingPlayers:     []PendingPlayer{},
		onlineCache:        make(map[string]bool),
		registrationActive: false,
	}
	wm.load()
	wm.loadBans()
	wm.loadPending()
	return wm
}

func (wm *WhitelistManager) load() error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	data, err := os.ReadFile(wm.filepath)
	if err != nil {
		if os.IsNotExist(err) {
			wm.players = []Player{}
			return nil
		}
		return err
	}

	var loadedPlayers []Player
	if err := json.Unmarshal(data, &loadedPlayers); err != nil {
		// Try loading old format
		var oldPlayers []struct {
			UUID string `json:"uuid"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(data, &oldPlayers); err != nil {
			return err
		}
		// Convert to new format
		for _, p := range oldPlayers {
			wm.players = append(wm.players, Player{
				UUID:      p.UUID,
				Name:      p.Name,
				LastSeen:  time.Now(),
				Status:    "offline",
				Rank:      "Member",
				AvatarURL: fmt.Sprintf("https://mc-heads.net/avatar/%s/40", p.Name),
			})
		}
		return nil
	}

	// Enhance existing players if needed
	for i := range loadedPlayers {
		if loadedPlayers[i].AvatarURL == "" {
			loadedPlayers[i].AvatarURL = fmt.Sprintf("https://mc-heads.net/avatar/%s/40", loadedPlayers[i].Name)
		}
		if loadedPlayers[i].Status == "" {
			loadedPlayers[i].Status = "offline"
		}
		if loadedPlayers[i].Rank == "" {
			loadedPlayers[i].Rank = "Member"
		}
		if loadedPlayers[i].LastSeen.IsZero() {
			loadedPlayers[i].LastSeen = time.Now()
		}
		// Set platform to "java" by default for existing players
		if loadedPlayers[i].Platform == "" {
			if isFloodgatePlayer(loadedPlayers[i].Name) {
				loadedPlayers[i].Platform = "bedrock"
			} else {
				loadedPlayers[i].Platform = "java"
			}
		}
		// Migrate players without Active field (default to true)
		if loadedPlayers[i].Active == false && len(loadedPlayers[i].Name) > 0 {
			// Check if this is a new player (empty UUID means not yet set)
			if loadedPlayers[i].UUID == "" {
				loadedPlayers[i].Active = true
			}
		}
		// Set Active to true by default for all players (migration)
		if loadedPlayers[i].Name != "" {
			loadedPlayers[i].Active = true
		}
	}

	wm.players = loadedPlayers
	log.Printf("Loaded %d players from %s", len(wm.players), wm.filepath)
	return nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}

	tempName := tempFile.Name()
	defer os.Remove(tempName)

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return err
	}

	if err := tempFile.Chmod(perm); err != nil {
		tempFile.Close()
		return err
	}

	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		return err
	}

	if err := tempFile.Close(); err != nil {
		return err
	}

	return os.Rename(tempName, path)
}

func (wm *WhitelistManager) save() error {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	return wm.saveUnsafe()
}

func (wm *WhitelistManager) saveUnsafe() error {
	data, err := json.MarshalIndent(wm.players, "", "  ")
	if err != nil {
		return err
	}

	return writeFileAtomic(wm.filepath, data, 0644)
}

func (wm *WhitelistManager) loadBans() error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	data, err := os.ReadFile(wm.bansFile)
	if err != nil {
		if os.IsNotExist(err) {
			wm.bans = []Ban{}
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &wm.bans)
}

func (wm *WhitelistManager) saveBans() error {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	return wm.saveBansUnsafe()
}

func (wm *WhitelistManager) saveBansUnsafe() error {
	data, err := json.MarshalIndent(wm.bans, "", "  ")
	if err != nil {
		return err
	}

	return writeFileAtomic(wm.bansFile, data, 0644)
}

// Pending player functions
// Floodgate helper functions
func isFloodgatePlayer(name string) bool {
	return strings.HasPrefix(name, ".")
}

func getBaseName(name string) string {
	if isFloodgatePlayer(name) {
		return strings.TrimPrefix(name, ".")
	}
	return name
}

// fuzzyMatch returns similarity percentage between two strings using Levenshtein distance
func fuzzyMatch(s1, s2 string) int {
	s1 = strings.ToLower(s1)
	s2 = strings.ToLower(s2)

	if s1 == s2 {
		return 100
	}

	len1 := len(s1)
	len2 := len(s2)

	if len1 == 0 || len2 == 0 {
		return 0
	}

	// Create matrix
	matrix := make([][]int, len1+1)
	for i := range matrix {
		matrix[i] = make([]int, len2+1)
	}

	// Initialize first column
	for i := 0; i <= len1; i++ {
		matrix[i][0] = i
	}

	// Initialize first row
	for j := 0; j <= len2; j++ {
		matrix[0][j] = j
	}

	// Fill in the rest
	for i := 1; i <= len1; i++ {
		for j := 1; j <= len2; j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}
			matrix[i][j] = min3(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	distance := matrix[len1][len2]
	maxLen := len1
	if len2 > maxLen {
		maxLen = len2
	}

	similarity := 100 - (distance*100)/maxLen
	return similarity
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

func (wm *WhitelistManager) loadPending() error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	data, err := os.ReadFile(wm.pendingFile)
	if err != nil {
		if os.IsNotExist(err) {
			wm.pendingPlayers = []PendingPlayer{}
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &wm.pendingPlayers)
}

func (wm *WhitelistManager) savePending() error {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	return wm.savePendingUnsafe()
}

// savePendingUnsafe saves without locking - caller must hold the lock
func (wm *WhitelistManager) savePendingUnsafe() error {
	data, err := json.MarshalIndent(wm.pendingPlayers, "", "  ")
	if err != nil {
		return err
	}

	return writeFileAtomic(wm.pendingFile, data, 0644)
}

func (wm *WhitelistManager) AddPendingPlayer(name, addedBy, platform string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	// Check if already in pending list
	for _, p := range wm.pendingPlayers {
		if strings.EqualFold(p.Name, name) {
			return fmt.Errorf("player %s is already in pending list", name)
		}
	}

	// Check if already whitelisted
	for _, p := range wm.players {
		if strings.EqualFold(p.Name, name) {
			return fmt.Errorf("player %s is already whitelisted", name)
		}
	}

	pending := PendingPlayer{
		Name:     name,
		AddedAt:  time.Now(),
		AddedBy:  addedBy,
		Platform: platform,
		Status:   "pending",
	}

	wm.pendingPlayers = append(wm.pendingPlayers, pending)
	return wm.savePendingUnsafe()
}

func (wm *WhitelistManager) RemovePendingPlayer(name string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	for i, p := range wm.pendingPlayers {
		if strings.EqualFold(p.Name, name) {
			wm.pendingPlayers = append(wm.pendingPlayers[:i], wm.pendingPlayers[i+1:]...)
			return wm.savePendingUnsafe()
		}
	}

	return fmt.Errorf("pending player %s not found", name)
}

func (wm *WhitelistManager) GetPendingPlayers() []PendingPlayer {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	result := make([]PendingPlayer, len(wm.pendingPlayers))
	copy(result, wm.pendingPlayers)
	return result
}

func (wm *WhitelistManager) GetPlayers(page, pageSize int, search string) PaginatedResponse {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	filtered := wm.players
	if search != "" {
		filtered = []Player{}
		for _, p := range wm.players {
			if contains(p.Name, search) || contains(p.UUID, search) {
				filtered = append(filtered, p)
			}
		}
	}

	total := len(filtered)
	totalPages := (total + pageSize - 1) / pageSize
	if totalPages == 0 {
		totalPages = 1
	}

	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= total {
		return PaginatedResponse{
			Data:       []Player{},
			Total:      total,
			Page:       page,
			PageSize:   pageSize,
			TotalPages: totalPages,
		}
	}

	if end > total {
		end = total
	}

	return PaginatedResponse{
		Data:       filtered[start:end],
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}
}

func (wm *WhitelistManager) AddPlayer(name string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	for i, p := range wm.players {
		if p.Name == name {
			// If player exists but is deactivated, reactivate them
			if !p.Active {
				wm.players[i].Active = true
				wm.players[i].LastSeen = time.Now()
				return wm.saveUnsafe()
			}
			return fmt.Errorf("player %s already exists", name)
		}
	}

	u := uuid.NewMD5(uuid.NameSpaceDNS, []byte("OfflinePlayer:"+name))
	player := Player{
		UUID:      u.String(),
		Name:      name,
		LastSeen:  time.Now(),
		Status:    "offline",
		Rank:      "Member",
		Active:    true,
		AvatarURL: fmt.Sprintf("https://mc-heads.net/avatar/%s/40", name),
	}

	wm.players = append(wm.players, player)
	return wm.saveUnsafe()
}

func (wm *WhitelistManager) DeactivatePlayer(name string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	for i, p := range wm.players {
		if p.Name == name {
			wm.players[i].Active = false
			return wm.saveUnsafe()
		}
	}

	return fmt.Errorf("player %s not found", name)
}

func (wm *WhitelistManager) ActivatePlayer(name string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	for i, p := range wm.players {
		if p.Name == name {
			wm.players[i].Active = true
			return wm.saveUnsafe()
		}
	}

	return fmt.Errorf("player %s not found", name)
}

func (wm *WhitelistManager) GetPlayer(name string) (Player, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	for _, p := range wm.players {
		if p.Name == name {
			return p, nil
		}
	}

	return Player{}, fmt.Errorf("player %s not found", name)
}

func (wm *WhitelistManager) RemovePlayer(name string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	for i, p := range wm.players {
		if p.Name == name {
			wm.players = append(wm.players[:i], wm.players[i+1:]...)
			return wm.saveUnsafe()
		}
	}

	return fmt.Errorf("player %s not found", name)
}

func (wm *WhitelistManager) UpdatePlayerStatus(name string, status string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	baseName := name
	isBedrock := isFloodgatePlayer(name)

	// For Floodgate players, get the base name (without . prefix)
	if isBedrock {
		baseName = getBaseName(name)
	}

	// First try to find exact name match
	for i, p := range wm.players {
		if p.Name == name {
			wm.players[i].Status = status
			if status == "online" {
				wm.players[i].LastSeen = time.Now()
			}
			return wm.saveUnsafe()
		}
	}

	// For Bedrock players, try to find and link with existing Java account
	if isBedrock {
		for i, p := range wm.players {
			if p.Name == baseName {
				// Found matching Java account - link them
				wm.players[i].Status = status
				wm.players[i].BedrockUUID = name // Store the Floodgate username
				if wm.players[i].Platform == "" {
					wm.players[i].Platform = "java"
				}
				if wm.players[i].Platform != "bedrock" {
					wm.players[i].Platform = "both"
				}
				if status == "online" {
					wm.players[i].LastSeen = time.Now()
				}
				log.Printf("✓ Linked Bedrock player %s to Java account %s", name, baseName)
				return wm.saveUnsafe()
			}
		}
	}

	// Player not found - this is expected for new players
	return fmt.Errorf("player %s not found", name)
}

func (wm *WhitelistManager) UpdatePlayerRank(name string, rank string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	for i, p := range wm.players {
		if p.Name == name {
			wm.players[i].Rank = rank
			return wm.saveUnsafe()
		}
	}

	return fmt.Errorf("player %s not found", name)
}

func (wm *WhitelistManager) BanPlayer(name, reason, bannedBy string, duration time.Duration) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	// Find player
	var player *Player
	for i, p := range wm.players {
		if p.Name == name {
			player = &wm.players[i]
			break
		}
	}

	if player == nil {
		return fmt.Errorf("player %s not found", name)
	}

	// Check if already banned
	for _, b := range wm.bans {
		if b.UUID == player.UUID {
			if b.ExpiresAt.After(time.Now()) {
				return fmt.Errorf("player %s is already banned", name)
			}
		}
	}

	ban := Ban{
		UUID:      player.UUID,
		Name:      name,
		Reason:    reason,
		BannedAt:  time.Now(),
		BannedBy:  bannedBy,
		ExpiresAt: time.Now().Add(duration),
	}

	wm.bans = append(wm.bans, ban)
	return wm.saveBansUnsafe()
}

func (wm *WhitelistManager) UnbanPlayer(name string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	for i, b := range wm.bans {
		if b.Name == name && b.ExpiresAt.After(time.Now()) {
			wm.bans = append(wm.bans[:i], wm.bans[i+1:]...)
			return wm.saveBansUnsafe()
		}
	}

	return fmt.Errorf("no active ban found for player %s", name)
}

func (wm *WhitelistManager) GetStats() ServerStats {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	activeBans := 0
	now := time.Now()
	for _, b := range wm.bans {
		if b.ExpiresAt.After(now) {
			activeBans++
		}
	}

	onlinePlayers := 0
	for _, p := range wm.players {
		if p.Status == "online" {
			onlinePlayers++
		}
	}

	maxPlayers := 100
	queueCapacity := float64(len(wm.players)) / float64(maxPlayers) * 100
	if queueCapacity > 100 {
		queueCapacity = 100
	}

	return ServerStats{
		TotalWhitelisted: len(wm.players),
		ActiveBans:       activeBans,
		OnlinePlayers:    onlinePlayers,
		MaxPlayers:       maxPlayers,
		QueueCapacity:    queueCapacity,
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		len(s) > 0 && len(substr) > 0 && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func cleanupSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for range ticker.C {
			authMu.Lock()
			now := time.Now()
			for token, session := range sessions {
				if now.After(session.ExpiresAt) {
					delete(sessions, token)
				}
			}
			authMu.Unlock()
		}
	}()
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return strings.EqualFold(value, "true") || value == "1" || strings.EqualFold(value, "yes")
}

func seedLogBufferFromFile(logPath string, limit int) {
	file, err := os.Open(logPath)
	if err != nil {
		log.Printf("Unable to seed logs from %s: %v", logPath, err)
		return
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > limit {
			lines = lines[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("Unable to read seed logs from %s: %v", logPath, err)
		return
	}

	logBufferMu.Lock()
	defer logBufferMu.Unlock()
	for _, line := range lines {
		logBuffer = append(logBuffer, parseLogLine(line))
		if len(logBuffer) > maxLogBuffer {
			logBuffer = logBuffer[1:]
		}
	}
}

func watchLogFile(logPath string, startAtEnd bool, onLine func(string)) {
	go func() {
		var offset int64
		initialized := false

		for {
			info, err := os.Stat(logPath)
			if err != nil {
				log.Printf("Waiting for log file %s: %v", logPath, err)
				time.Sleep(5 * time.Second)
				continue
			}

			if !initialized {
				if startAtEnd {
					offset = info.Size()
				}
				initialized = true
			} else if info.Size() < offset {
				offset = 0
			}

			file, err := os.Open(logPath)
			if err != nil {
				log.Printf("Unable to open log file %s: %v", logPath, err)
				time.Sleep(5 * time.Second)
				continue
			}

			if _, err := file.Seek(offset, io.SeekStart); err != nil {
				file.Close()
				log.Printf("Unable to seek log file %s: %v", logPath, err)
				time.Sleep(5 * time.Second)
				continue
			}

			scanner := bufio.NewScanner(file)
			scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for scanner.Scan() {
				onLine(scanner.Text())
			}

			currentOffset, seekErr := file.Seek(0, io.SeekCurrent)
			file.Close()
			if seekErr == nil {
				offset = currentOffset
			}

			if err := scanner.Err(); err != nil {
				log.Printf("Error watching log file %s: %v", logPath, err)
			}

			time.Sleep(1 * time.Second)
		}
	}()
}

func authMiddleware(c *fiber.Ctx) error {
	// Try cookie first
	token := c.Cookies("token")

	// Fallback to Authorization header
	if token == "" {
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			// Extract token from "Bearer <token>"
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}
	}

	if token == "" {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	session, exists := sessions[token]

	if !exists || time.Now().After(session.ExpiresAt) {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	return c.Next()
}

func monitorServerLogs() {
	log.Printf("Starting log monitor for file: %s", dashboardLogPath)

	// Regex patterns for join/leave messages
	joinPattern := regexp.MustCompile(`joined the game`)
	leavePattern := regexp.MustCompile(`left the game`)
	uuidPattern := regexp.MustCompile(`UUID of player (\w+) is ([a-f0-9-]+)`)
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*m`)

	// Track UUIDs for usernames
	uuidMap := make(map[string]string)
	uuidMapMu := sync.Mutex{}

	// Channel to serialize status updates and prevent race conditions
	statusUpdateChan := make(chan struct{name string; status string}, 100)

	// Goroutine to process status updates sequentially
	go func() {
		for update := range statusUpdateChan {
			if err := wm.UpdatePlayerStatus(update.name, update.status); err != nil {
				log.Printf("Error updating %s to %s: %v", update.name, update.status, err)
			} else {
				log.Printf("✓ %s is now %s", update.name, update.status)
			}
		}
	}()

	watchLogFile(dashboardLogPath, true, func(line string) {
		cleanLine := ansiRegex.ReplaceAllString(line, "")

		if uuidMatches := uuidPattern.FindStringSubmatch(cleanLine); uuidMatches != nil {
			username := uuidMatches[1]
			playerUUID := uuidMatches[2]
			uuidMapMu.Lock()
			uuidMap[username] = playerUUID
			uuidMapMu.Unlock()
		}

		if joinPattern.MatchString(cleanLine) {
			parts := strings.Fields(cleanLine)
			for i, part := range parts {
				if strings.Contains(part, "joined") && i > 0 {
					username := strings.TrimPrefix(strings.TrimSuffix(parts[i-1], ":"), "[")
					select {
					case statusUpdateChan <- struct{name string; status string}{name: username, status: "online"}:
					default:
						log.Printf("Warning: status update channel full, dropping update for %s", username)
					}
					break
				}
			}
		}

		if leavePattern.MatchString(cleanLine) {
			parts := strings.Fields(cleanLine)
			for i, part := range parts {
				if strings.Contains(part, "left") && i > 0 {
					username := strings.TrimPrefix(strings.TrimSuffix(parts[i-1], ":"), "[")
					select {
					case statusUpdateChan <- struct{name string; status string}{name: username, status: "offline"}:
					default:
						log.Printf("Warning: status update channel full, dropping update for %s", username)
					}
					break
				}
			}
		}
	})
}

func main() {
	username := strings.TrimSpace(os.Getenv("ADMIN_USERNAME"))
	password := strings.TrimSpace(os.Getenv("ADMIN_PASSWORD"))
	if username == "" || password == "" {
		log.Fatal("ADMIN_USERNAME and ADMIN_PASSWORD must be set")
	}
	if username == "admin" && password == "minecraft123" {
		log.Fatal("Refusing to start with default dashboard credentials")
	}

	authCfg = AuthConfig{
		Username: username,
		Password: password,
	}
	sessionCookieSecure = envBool("SESSION_COOKIE_SECURE", false)
	corsAllowOrigins = envOrDefault("CORS_ALLOW_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000")
	dashboardLogPath = envOrDefault("LOG_FILE_PATH", "/logs/main/latest.log")

	whitelistPath := os.Getenv("WHITELIST_PATH")
	if whitelistPath == "" {
		whitelistPath = "/data/whitelist.json"
	}

	bansPath := os.Getenv("BANS_PATH")
	if bansPath == "" {
		bansPath = "/data/banned-players.json"
	}

	pendingPath := os.Getenv("PENDING_PATH")
	if pendingPath == "" {
		pendingPath = whitelistPath[:len(whitelistPath)-len(".json")] + "_pending.json"
	}

	wm = NewWhitelistManager(whitelistPath, bansPath, pendingPath)
	cleanupSessions()
	monitorServerLogs()
	startLogReader()

	app := fiber.New(fiber.Config{
		AppName:               "CreeperPanel",
		DisableStartupMessage: false,
		EnablePrintRoutes:     false,
	})

	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     corsAllowOrigins,
		AllowCredentials: true,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
	}))
	app.Use(recover.New())

	// Add cache-busting headers to all responses
	app.Use(func(c *fiber.Ctx) error {
		c.Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		c.Set("Pragma", "no-cache")
		c.Set("Expires", "0")
		return c.Next()
	})

	// Serve static files
	app.Static("/", "./static")

	// Auth routes
	api := app.Group("/api")
	api.Post("/login", limiter.New(limiter.Config{
		Max:        10,
		Expiration: time.Minute,
	}), login)
	api.Post("/logout", logout)
	api.Get("/check-auth", checkAuth)

	// Public routes (no auth required)
	api.Get("/public/open-mode", getProxyOpenMode)

	// Protected routes
	protected := api.Group("")
	protected.Use(authMiddleware)
	protected.Get("/players", getPlayers)
	protected.Post("/players", addPlayer)
	protected.Delete("/players/:name", removePlayer)
	protected.Put("/players/:name/status", updatePlayerStatus)
	protected.Put("/players/:name/rank", updatePlayerRank)
	protected.Get("/bans", getBans)
	protected.Post("/bans", banPlayer)
	protected.Delete("/bans/:name", unbanPlayer)
	protected.Get("/stats", getStats)
	protected.Post("/players/:name/kick", kickPlayer)
	protected.Put("/players/:name/deactivate", deactivatePlayer)
	protected.Put("/players/:name/activate", activatePlayer)
	protected.Post("/whitelist/sync", syncWhitelist)
	protected.Post("/bedrock/register", registerBedrockPlayer)
	protected.Get("/bedrock/status", getRegistrationStatus)
	protected.Get("/pending", getPendingPlayersAPI)
	protected.Post("/pending", addPendingPlayerAPI)
	protected.Delete("/pending/:name", removePendingPlayerAPI)
	protected.Get("/logs/stream", streamLogs)
	protected.Get("/logs", getLogs)

	// Proxy integration routes
	protected.Get("/proxy/pending", getProxyPending)
	protected.Post("/proxy/approve", approveProxyPlayer)
	protected.Get("/proxy/status", getProxyStatus)
	protected.Get("/proxy/open-mode", getProxyOpenMode)
	protected.Post("/proxy/open-mode", setProxyOpenMode)

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(app.Listen(":" + port))
}

func login(c *fiber.Ctx) error {
	type Request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.Username != authCfg.Username || req.Password != authCfg.Password {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid credentials"})
	}

	token := uuid.New().String()
	session := Session{
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	authMu.Lock()
	sessions[token] = session
	authMu.Unlock()

	c.Cookie(&fiber.Cookie{
		Name:     "token",
		Value:    token,
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: true,
		Secure:   sessionCookieSecure,
		SameSite: "strict",
	})

	return c.JSON(fiber.Map{
		"message": "Login successful",
	})
}

func logout(c *fiber.Ctx) error {
	token := c.Cookies("token")

	authMu.Lock()
	delete(sessions, token)
	authMu.Unlock()

	c.ClearCookie("token")
	return c.JSON(fiber.Map{"message": "Logout successful"})
}

func checkAuth(c *fiber.Ctx) error {
	// Try cookie first
	token := c.Cookies("token")

	// Fallback to Authorization header
	if token == "" {
		authHeader := c.Get("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) == 2 && parts[0] == "Bearer" {
				token = parts[1]
			}
		}
	}

	if token == "" {
		return c.Status(401).JSON(fiber.Map{"authenticated": false})
	}

	authMu.RLock()
	session, exists := sessions[token]
	authMu.RUnlock()

	if !exists || time.Now().After(session.ExpiresAt) {
		return c.Status(401).JSON(fiber.Map{"authenticated": false})
	}

	return c.JSON(fiber.Map{
		"authenticated": true,
		"expiresAt":     session.ExpiresAt,
	})
}

func getPlayers(c *fiber.Ctx) error {
	if err := wm.load(); err != nil {
		log.Printf("Failed to refresh players from disk: %v", err)
	}
	syncBlockedPlayersFromProxy()

	page := c.QueryInt("page", 1)
	pageSize := c.QueryInt("pageSize", 10)
	search := c.Query("search")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	result := wm.GetPlayers(page, pageSize, search)
	return c.JSON(result)
}

func kickPlayer(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{
		"error": "Kick from dashboard is disabled in hardened mode",
	})
}

func syncWhitelist(c *fiber.Ctx) error {
	if err := wm.load(); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to refresh whitelist state"})
	}
	syncBlockedPlayersFromProxy()
	return c.JSON(fiber.Map{
		"message": "Dashboard state refreshed from disk and proxy",
		"count":   len(wm.players),
	})
}

// Register Bedrock player - monitors for player to join and auto-whitelists them
func registerBedrockPlayer(c *fiber.Ctx) error {
	return c.Status(501).JSON(fiber.Map{
		"error": "Legacy Bedrock registration is disabled in hardened mode; use proxy pending approvals instead",
	})
}

// Get registration status
func getRegistrationStatus(c *fiber.Ctx) error {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return c.JSON(fiber.Map{
		"active":   wm.registrationActive,
		"target":   wm.registrationTarget,
	})
}

// Pending player API handlers
func getPendingPlayersAPI(c *fiber.Ctx) error {
	return c.JSON(wm.GetPendingPlayers())
}

func addPendingPlayerAPI(c *fiber.Ctx) error {
	type Request struct {
		Name     string `json:"name"`
		Platform string `json:"platform"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Name is required"})
	}

	addedBy := "admin"

	if err := wm.AddPendingPlayer(req.Name, addedBy, req.Platform); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "Pending player added",
		"name":    req.Name,
	})
}

func removePendingPlayerAPI(c *fiber.Ctx) error {
	name := c.Params("name")
	if err := wm.RemovePendingPlayer(name); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "Pending player removed", "name": name})
}

func addPlayer(c *fiber.Ctx) error {
	type Request struct {
		Name string `json:"name"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Name is required"})
	}

	if err := wm.AddPlayer(req.Name); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "Player added successfully",
		"player":  req.Name,
	})
}

func removePlayer(c *fiber.Ctx) error {
	name := c.Params("name")

	if err := wm.RemovePlayer(name); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "Player removed successfully",
		"player":  name,
	})
}

func updatePlayerStatus(c *fiber.Ctx) error {
	name := c.Params("name")

	type Request struct {
		Status string `json:"status"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.Status != "online" && req.Status != "offline" {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid status"})
	}

	if err := wm.UpdatePlayerStatus(name, req.Status); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "Player status updated successfully",
		"player":  name,
		"status":  req.Status,
	})
}

func updatePlayerRank(c *fiber.Ctx) error {
	name := c.Params("name")

	type Request struct {
		Rank string `json:"rank"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	validRanks := map[string]bool{
		"Administrator": true,
		"Moderator":     true,
		"Member":        true,
	}

	if !validRanks[req.Rank] {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid rank"})
	}

	if err := wm.UpdatePlayerRank(name, req.Rank); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "Player rank updated successfully",
		"player":  name,
		"rank":    req.Rank,
	})
}

func getBans(c *fiber.Ctx) error {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	now := time.Now()
	activeBans := []Ban{}
	for _, b := range wm.bans {
		if b.ExpiresAt.After(now) {
			activeBans = append(activeBans, b)
		}
	}

	return c.JSON(activeBans)
}

func banPlayer(c *fiber.Ctx) error {
	type Request struct {
		Name     string `json:"name"`
		Reason   string `json:"reason"`
		Duration int    `json:"duration"` // in hours
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Name is required"})
	}

	if req.Reason == "" {
		req.Reason = "Banned by administrator"
	}

	duration := time.Duration(req.Duration) * time.Hour
	if duration == 0 {
		duration = 24 * time.Hour // Default 24 hours
	}

	if err := wm.BanPlayer(req.Name, req.Reason, "admin", duration); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "Player banned successfully",
		"player":  req.Name,
	})
}

func unbanPlayer(c *fiber.Ctx) error {
	name := c.Params("name")

	if err := wm.UnbanPlayer(name); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"message": "Player unbanned successfully",
		"player":  name,
	})
}

func deactivatePlayer(c *fiber.Ctx) error {
	name := c.Params("name")
	syncBlockedPlayersFromProxy()
	player, err := wm.GetPlayer(name)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	if err := wm.DeactivatePlayer(name); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	if proxyClient == nil {
		proxyClient = NewProxyClient()
	}
	if err := proxyClient.SetPlayerActive(player.UUID, player.Name, false); err != nil {
		_ = wm.ActivatePlayer(name)
		return c.Status(500).JSON(fiber.Map{
			"error":   "Failed to deactivate player access",
			"message": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Player deactivated successfully",
		"player":  name,
	})
}

func activatePlayer(c *fiber.Ctx) error {
	name := c.Params("name")
	syncBlockedPlayersFromProxy()
	player, err := wm.GetPlayer(name)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	if err := wm.ActivatePlayer(name); err != nil {
		return c.Status(404).JSON(fiber.Map{"error": err.Error()})
	}

	if proxyClient == nil {
		proxyClient = NewProxyClient()
	}
	if err := proxyClient.SetPlayerActive(player.UUID, player.Name, true); err != nil {
		_ = wm.DeactivatePlayer(name)
		return c.Status(500).JSON(fiber.Map{
			"error":   "Failed to reactivate player access",
			"message": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"message": "Player activated successfully",
		"player":  name,
	})
}

func getStats(c *fiber.Ctx) error {
	stats := wm.GetStats()
	return c.JSON(stats)
}

// LogEntry represents a single log line
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

// Log buffer stores last 1000 log lines
var (
	logBuffer     []LogEntry
	logBufferMu   sync.RWMutex
	maxLogBuffer  = 1000
)

// streamLogs sends server logs via Server-Sent Events
func streamLogs(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")

	// Send last 100 logs from buffer first
	logBufferMu.RLock()
	for _, log := range logBuffer {
		data := fmt.Sprintf("data: %s\n\n", mustMarshalJSON(log))
		if _, err := c.Write([]byte(data)); err != nil {
			logBufferMu.RUnlock()
			return err
		}
	}
	logBufferMu.RUnlock()

	// Keep connection alive
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		// Send keepalive every 30 seconds
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Send comment to keep connection alive
				fmt.Fprintf(w, ": keepalive\n\n")
				w.Flush()
			case <-c.Context().Done():
				return
			}
		}
	})

	return nil
}

// getLogs returns recent logs as JSON
func getLogs(c *fiber.Ctx) error {
	lines := c.QueryInt("lines", 100)
	if lines > 1000 {
		lines = 1000
	}
	if lines < 1 {
		lines = 100
	}

	logBufferMu.RLock()
	defer logBufferMu.RUnlock()

	start := len(logBuffer) - lines
	if start < 0 {
		start = 0
	}

	return c.JSON(logBuffer[start:])
}

// mustMarshalJSON converts LogEntry to JSON string
func mustMarshalJSON(log LogEntry) string {
	data, _ := json.Marshal(log)
	return string(data)
}

// Proxy client instance
var proxyClient *ProxyClient

// getProxyPending returns pending Bedrock players from the proxy
func getProxyPending(c *fiber.Ctx) error {
	if proxyClient == nil {
		proxyClient = NewProxyClient()
	}

	pending, err := proxyClient.GetPendingPlayers()
	if err != nil {
		return c.Status(503).JSON(fiber.Map{
			"error": "Proxy unavailable",
			"message": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"pending": pending,
		"count":   len(pending),
	})
}

func syncBlockedPlayersFromProxy() {
	if proxyClient == nil {
		proxyClient = NewProxyClient()
	}

	blocked, err := proxyClient.GetBlockedPlayers()
	if err != nil {
		log.Printf("Failed to sync blocked players from proxy: %v", err)
		return
	}

	wm.mu.Lock()
	defer wm.mu.Unlock()

	for _, blockedPlayer := range blocked {
		found := false
		for i, player := range wm.players {
			if player.UUID == blockedPlayer.UUID || player.Name == blockedPlayer.Name {
				wm.players[i].UUID = blockedPlayer.UUID
				wm.players[i].Name = blockedPlayer.Name
				wm.players[i].Active = false
				wm.players[i].Status = "offline"
				if wm.players[i].AvatarURL == "" {
					wm.players[i].AvatarURL = fmt.Sprintf("https://mc-heads.net/avatar/%s/40", blockedPlayer.Name)
				}
				if wm.players[i].Platform == "" {
					if isFloodgatePlayer(blockedPlayer.Name) {
						wm.players[i].Platform = "bedrock"
					} else {
						wm.players[i].Platform = "java"
					}
				}
				found = true
				break
			}
		}

		if !found {
			platform := "java"
			if isFloodgatePlayer(blockedPlayer.Name) {
				platform = "bedrock"
			}

			wm.players = append(wm.players, Player{
				UUID:      blockedPlayer.UUID,
				Name:      blockedPlayer.Name,
				LastSeen:  time.Now(),
				Status:    "offline",
				Rank:      "Member",
				Active:    false,
				AvatarURL: fmt.Sprintf("https://mc-heads.net/avatar/%s/40", blockedPlayer.Name),
				Platform:  platform,
			})
		}
	}
}

// approveProxyPlayer approves a pending player in the proxy
func approveProxyPlayer(c *fiber.Ctx) error {
	type Request struct {
		Name string `json:"name"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Name required"})
	}

	if proxyClient == nil {
		proxyClient = NewProxyClient()
	}

	if err := proxyClient.ApprovePlayer(req.Name); err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "Failed to approve player",
			"message": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Player " + req.Name + " approved",
	})
}

// getProxyStatus returns the proxy status
func getProxyStatus(c *fiber.Ctx) error {
	if proxyClient == nil {
		proxyClient = NewProxyClient()
	}

	status, err := proxyClient.GetStatus()
	if err != nil {
		return c.Status(503).JSON(fiber.Map{
			"error":    "Proxy unavailable",
			"message":  err.Error(),
			"online":   false,
		})
	}

	return c.JSON(fiber.Map{
		"online":          true,
		"whitelisted":     status.WhitelistedCount,
		"pending_bedrock": status.PendingCount,
		"open_mode":       status.OpenMode,
		"hybrid_auth_mode": status.HybridAuthMode,
		"main_server":     status.MainServer,
		"limbo_server":    status.LimboServer,
	})
}

func getProxyOpenMode(c *fiber.Ctx) error {
	if proxyClient == nil {
		proxyClient = NewProxyClient()
	}

	status, err := proxyClient.GetOpenMode()
	if err != nil {
		return c.Status(503).JSON(fiber.Map{
			"error":   "Proxy unavailable",
			"message": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"enabled":    status.Enabled,
		"updated_at": status.UpdatedAt,
		"updated_by": status.UpdatedBy,
	})
}

func setProxyOpenMode(c *fiber.Ctx) error {
	type Request struct {
		Enabled bool `json:"enabled"`
	}

	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	if proxyClient == nil {
		proxyClient = NewProxyClient()
	}

	status, err := proxyClient.SetOpenMode(req.Enabled, "dashboard")
	if err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error":   "Failed to update open mode",
			"message": err.Error(),
		})
	}

	if err := wm.load(); err != nil {
		log.Printf("Failed to refresh whitelist after open mode update: %v", err)
	}

	return c.JSON(fiber.Map{
		"success":    true,
		"enabled":    status.Enabled,
		"updated_at": status.UpdatedAt,
		"updated_by": status.UpdatedBy,
	})
}

// startLogReader reads the mounted server log file and broadcasts to subscribers
func startLogReader() {
	seedLogBufferFromFile(dashboardLogPath, 100)
	watchLogFile(dashboardLogPath, true, func(line string) {
		logEntry := parseLogLine(line)
		logBufferMu.Lock()
		logBuffer = append(logBuffer, logEntry)
		if len(logBuffer) > maxLogBuffer {
			logBuffer = logBuffer[1:]
		}
		logBufferMu.Unlock()
	})
}

// parseLogLine parses a Minecraft log line into a LogEntry
func parseLogLine(line string) LogEntry {
	// Example: [14:23:54 INFO]: Starting Minecraft server on *:25565

	// Try to match standard Minecraft log format
	parts := strings.SplitN(line, " ", 3)
	if len(parts) >= 3 {
		timestamp := strings.Trim(parts[0], "[]")
		rest := strings.Join(parts[1:], " ")

		// Extract log level
		level := "INFO"
		if strings.Contains(rest, "INFO") {
			level = "INFO"
		} else if strings.Contains(rest, "WARN") {
			level = "WARN"
		} else if strings.Contains(rest, "ERROR") {
			level = "ERROR"
		} else if strings.Contains(rest, "DEBUG") {
			level = "DEBUG"
		}

		return LogEntry{
			Timestamp: timestamp,
			Level:     level,
			Message:   rest,
		}
	}

	// Fallback: treat entire line as message
	return LogEntry{
		Timestamp: time.Now().Format("15:04:05"),
		Level:     "INFO",
		Message:   line,
	}
}
