package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Store manages in-memory game servers with thread-safe reads and atomic snapshot publishing.
type Store struct {
	mu           sync.RWMutex
	servers      map[string]*Server
	blacklist    map[string]struct{}
	snapshot     atomic.Pointer[Snapshot]
	allowPrivate bool
	dirty        atomic.Bool
}

// NewStore creates a new server store.
func NewStore(allowPrivate bool) *Store {
	s := &Store{
		servers:      make(map[string]*Server),
		blacklist:    make(map[string]struct{}),
		allowPrivate: allowPrivate,
	}
	s.RebuildSnapshot()
	return s
}

// AddBlacklist adds IP addresses to the blocklist and immediately purges them from the store.
func (s *Store) AddBlacklist(ips ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, raw := range ips {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		s.blacklist[trimmed] = struct{}{}

		// Purge any registered server with this IP immediately
		for k, srv := range s.servers {
			if srv.IP == trimmed {
				delete(s.servers, k)
				s.dirty.Store(true)
			}
		}
	}
}

// IsBlacklisted reports whether an IP is in the blacklist.
func (s *Store) IsBlacklisted(ip string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, found := s.blacklist[ip]
	return found
}

// isPrivateIP checks if an IP is loopback or RFC1918 private.
func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	// Check standard private blocks
	privateCIDRs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	}
	for _, cidr := range privateCIDRs {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// RegisterHeartbeat records an incoming UDP heartbeat.
func (s *Store) RegisterHeartbeat(ip string, queryPort int, gamename string) (*Server, bool) {
	if s.IsBlacklisted(ip) {
		return nil, false
	}
	if !s.allowPrivate && isPrivateIP(ip) {
		return nil, false
	}
	if queryPort <= 0 || queryPort > 65535 {
		return nil, false
	}

	gn := strings.ToLower(gamename)
	actualQueryPort := queryPort
	actualGamePort := queryPort

	// Kingpin dual-port standard: GamePort 31510..31599 -> QueryPort 31500..31589
	if gn == "kingpin" && queryPort >= 31510 && queryPort <= 31599 {
		actualQueryPort = queryPort - 10
		actualGamePort = queryPort
	}

	key := fmt.Sprintf("%s:%d", ip, actualQueryPort)
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.dirty.Store(true)

	srv, exists := s.servers[key]
	if !exists {
		srv = &Server{
			ID:            key,
			IP:            ip,
			QueryPort:     actualQueryPort,
			GamePort:      actualGamePort,
			Gamename:      gn,
			Hostname:      key,
			Verified:      true,
			LastHeartbeat: now,
			CVars:         make(map[string]string),
		}
		s.servers[key] = srv
		return srv, true
	}

	srv.LastHeartbeat = now
	srv.Verified = true
	if gn != "" {
		srv.Gamename = gn
	}
	if actualGamePort > 0 {
		srv.GamePort = actualGamePort
	}
	return srv, false
}

// UpdateServerInfo updates a server's metadata (hostname, map, players, cvars).
func (s *Store) UpdateServerInfo(
	ip string,
	queryPort int,
	gamePort int,
	hostname string,
	mapname string,
	players int,
	maxPlayers int,
	ping int,
	roster []Player,
	cvars map[string]string,
) {
	key := fmt.Sprintf("%s:%d", ip, queryPort)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.dirty.Store(true)

	srv, exists := s.servers[key]
	if !exists {
		return
	}

	srv.Verified = true
	srv.LastChecked = time.Now()
	if gamePort > 0 {
		srv.GamePort = gamePort
	}
	cleanHost := StripQuakeColors(hostname)
	if cleanHost != "" {
		// Cap length to prevent layout abuse
		if len(cleanHost) > 64 {
			cleanHost = cleanHost[:64]
		}
		srv.Hostname = cleanHost
	}

	cleanMap := StripQuakeColors(mapname)
	if len(cleanMap) > 32 {
		cleanMap = cleanMap[:32]
	}
	srv.Map = cleanMap

	srv.Players = players
	srv.MaxPlayers = maxPlayers
	srv.Ping = ping
	srv.Roster = roster
	srv.CVars = cvars

	if srv.Gamename == "kingpin" {
		srv.Gametype = ClassifyKingpinMode(cleanHost, cleanMap, cvars)
	} else if gt, ok := cvars["gametype"]; ok && gt != "" {
		srv.Gametype = gt
	}
	s.dirty.Store(true)
}

// GetAllServers returns a slice of all registered servers in the store.
func (s *Store) GetAllServers() []Server {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]Server, 0, len(s.servers))
	for _, srv := range s.servers {
		if _, blocked := s.blacklist[srv.IP]; blocked {
			continue
		}
		list = append(list, *srv)
	}
	return list
}

// GetServersByGame returns all verified servers for a specific game (for TCP 28900 \list\).
// If gamename is "all", it returns all verified servers across every game.
func (s *Store) GetServersByGame(gamename string) []Server {
	s.mu.RLock()
	defer s.mu.RUnlock()

	gn := strings.ToLower(gamename)
	var list []Server
	for _, srv := range s.servers {
		if srv.Verified && (gn == "all" || srv.Gamename == gn || matchesGameAlias(srv.Gamename, gn)) {
			list = append(list, *srv)
		}
	}
	return list
}

func matchesGameAlias(srvGame, queryGame string) bool {
	s := strings.ToLower(srvGame)
	q := strings.ToLower(queryGame)
	if s == q {
		return true
	}

	aliases := map[string][]string{
		"cod":         {"callofduty"},
		"callofduty":  {"cod"},
		"coduo":       {"callofdutyuo"},
		"callofdutyuo": {"coduo"},
		"cod2":        {"callofduty2"},
		"callofduty2": {"cod2"},
		"cod4":        {"callofduty4", "callofduty4modernwarfare"},
		"ut":          {"ut99"},
		"ut99":        {"ut"},
		"ut2004":      {"ut2"},
		"ut2":         {"ut2004"},
		"q3a":         {"quake3"},
		"quake3":      {"q3a"},
		"q2":          {"quake2"},
		"quake2":      {"q2"},
		"kingpin":     {"kp"},
	}

	if match, ok := aliases[q]; ok {
		for _, m := range match {
			if s == m {
				return true
			}
		}
	}
	return false
}

// Prune removes servers that have not sent a heartbeat within the TTL.
func (s *Store) Prune(ttl time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-ttl)
	pruned := 0
	for k, srv := range s.servers {
		if srv.LastHeartbeat.Before(cutoff) && srv.LastChecked.Before(cutoff) {
			delete(s.servers, k)
			pruned++
		}
	}
	if pruned > 0 {
		s.dirty.Store(true)
	}
	return pruned
}

// RebuildSnapshot constructs an immutable public snapshot and publishes it atomically.
func (s *Store) RebuildSnapshot() {
	s.mu.RLock()
	activeGamesMap := make(map[string]struct{})
	totalPlayers := 0

	// Deduplicate verified servers by physical game instance
	dedupMap := make(map[string]*Server)
	for _, srv := range s.servers {
		if !srv.Verified {
			continue
		}

		effectiveGamePort := srv.GamePort
		if srv.Gamename == "kingpin" {
			if effectiveGamePort >= 31520 && srv.QueryPort >= 31510 && srv.QueryPort <= 31599 {
				effectiveGamePort = srv.QueryPort
			} else if effectiveGamePort <= 0 {
				effectiveGamePort = srv.QueryPort + 10
			}
		} else if effectiveGamePort <= 0 {
			effectiveGamePort = srv.QueryPort
		}

		// Endpoint key by game, IP and effective GamePort
		endpointKey := fmt.Sprintf("%s:%s:%d", srv.Gamename, srv.IP, effectiveGamePort)

		existing, found := dedupMap[endpointKey]
		if !found {
			dedupMap[endpointKey] = srv
		} else {
			// Duplicate found: Keep the server with more players, roster, or richer CVars
			if len(srv.Roster) > len(existing.Roster) || srv.Players > existing.Players || len(srv.CVars) > len(existing.CVars) {
				dedupMap[endpointKey] = srv
			}
		}
	}

	var verified []Server
	for _, srv := range dedupMap {
		verified = append(verified, *srv)
		activeGamesMap[srv.Gamename] = struct{}{}
		if srv.Players > 0 {
			totalPlayers += srv.Players
		}
	}
	s.mu.RUnlock()

	// Sort by player count descending, then hostname
	sort.Slice(verified, func(i, j int) bool {
		if verified[i].Players != verified[j].Players {
			return verified[i].Players > verified[j].Players
		}
		return verified[i].Hostname < verified[j].Hostname
	})

	snap := &Snapshot{
		GeneratedAt:  time.Now(),
		TotalServers: len(verified),
		TotalPlayers: totalPlayers,
		ActiveGames:  len(activeGamesMap),
		Servers:      verified,
	}

	s.snapshot.Store(snap)
}

// GetSnapshot returns the current atomic snapshot in O(1) time with zero lock contention.
func (s *Store) GetSnapshot() *Snapshot {
	return s.snapshot.Load()
}

// TotalCount returns the count of all registered servers (verified + unverified).
func (s *Store) TotalCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.servers)
}

// IsDirty returns whether the store has pending changes not yet written to disk.
func (s *Store) IsDirty() bool {
	return s.dirty.Load()
}

// SetDirty explicitly updates the dirty state.
func (s *Store) SetDirty(dirty bool) {
	s.dirty.Store(dirty)
}

// SaveToFile serializes the current server registry to an atomic JSON file.
func (s *Store) SaveToFile(path string) error {
	if path == "" {
		return nil
	}

	s.mu.RLock()
	servers := make([]*Server, 0, len(s.servers))
	for _, srv := range s.servers {
		cp := *srv
		if srv.Roster != nil {
			cp.Roster = make([]Player, len(srv.Roster))
			copy(cp.Roster, srv.Roster)
		}
		if srv.CVars != nil {
			cp.CVars = make(map[string]string, len(srv.CVars))
			for k, v := range srv.CVars {
				cp.CVars[k] = v
			}
		}
		servers = append(servers, &cp)
	}
	s.mu.RUnlock()

	rec := DatabaseRecord{
		SavedAt:     time.Now().UTC(),
		Version:     1,
		ServerCount: len(servers),
		Servers:     servers,
	}

	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal database json: %w", err)
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create db directory %s: %w", dir, err)
		}
	}

	// Atomic write: write to unique temporary file first, then atomic rename
	tmpFile := fmt.Sprintf("%s.tmp.%d", path, time.Now().UnixNano())
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temporary db file %s: %w", tmpFile, err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to atomically rename %s to %s: %w", tmpFile, path, err)
	}

	s.dirty.Store(false)
	return nil
}

// LoadFromFile reads a JSON database file and restores all servers into the registry.
func (s *Store) LoadFromFile(path string) (int, error) {
	if path == "" {
		return 0, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read db file %s: %w", path, err)
	}

	var servers []*Server

	// 1. Try structured DatabaseRecord first
	var rec DatabaseRecord
	if errRec := json.Unmarshal(data, &rec); errRec == nil && len(rec.Servers) > 0 {
		servers = rec.Servers
	} else {
		// 2. Fallback to raw []*Server array
		var rawServers []*Server
		if errRaw := json.Unmarshal(data, &rawServers); errRaw == nil && len(rawServers) > 0 {
			servers = rawServers
		} else {
			return 0, fmt.Errorf("invalid json database format in %s", path)
		}
	}

	s.mu.Lock()
	loadedCount := 0
	for _, srv := range servers {
		if srv == nil || srv.IP == "" || srv.QueryPort <= 0 {
			continue
		}
		if _, blocked := s.blacklist[srv.IP]; blocked {
			continue
		}

		// Auto-migrate legacy records where QueryPort == GamePort for games requiring offsets
		gn := strings.ToLower(srv.Gamename)
		if srv.QueryPort == srv.GamePort {
			switch gn {
			case "ut", "ut99", "unreal", "rune", "deusex", "tacticalops", "postal2", "postal2d",
				"swat4", "serioussam", "serioussamse", "serioussam2", "tribesv", "undying", "nerfarena", "wot":
				srv.QueryPort = srv.GamePort + 1
				srv.ID = fmt.Sprintf("%s:%d", srv.IP, srv.QueryPort)
			case "ut2004", "ut2", "ut2003":
				srv.QueryPort = srv.GamePort + 10
				srv.ID = fmt.Sprintf("%s:%d", srv.IP, srv.QueryPort)
			case "mohaa", "mohaab", "mohaas":
				if srv.GamePort >= 12200 && srv.GamePort <= 12220 {
					srv.QueryPort = srv.GamePort + 97
					srv.ID = fmt.Sprintf("%s:%d", srv.IP, srv.QueryPort)
				}
			case "bf1942", "bfield1942":
				if srv.GamePort == 14567 {
					srv.QueryPort = 23000
					srv.ID = fmt.Sprintf("%s:%d", srv.IP, srv.QueryPort)
				}
			case "bfvietnam":
				if srv.GamePort == 15667 {
					srv.QueryPort = 23000
					srv.ID = fmt.Sprintf("%s:%d", srv.IP, srv.QueryPort)
				}
			case "vietcong", "vietcong2":
				if srv.GamePort >= 5420 && srv.GamePort <= 5440 {
					srv.QueryPort = srv.GamePort + 10000
					srv.ID = fmt.Sprintf("%s:%d", srv.IP, srv.QueryPort)
				}
			case "kingpin":
				if srv.GamePort >= 31510 && srv.GamePort <= 31599 {
					srv.QueryPort = srv.GamePort - 10
					srv.ID = fmt.Sprintf("%s:%d", srv.IP, srv.QueryPort)
				}
			}
		}

		// In passive directory mode, all restored servers are verified and served immediately.
		srv.Verified = true
		if srv.LastHeartbeat.IsZero() {
			srv.LastHeartbeat = time.Now()
		}

		if srv.ID == "" {
			srv.ID = fmt.Sprintf("%s:%d", srv.IP, srv.QueryPort)
		}
		if srv.CVars == nil {
			srv.CVars = make(map[string]string)
		}
		if srv.Roster == nil {
			srv.Roster = []Player{}
		}
		s.servers[srv.ID] = srv
		loadedCount++
	}
	s.mu.Unlock()

	s.RebuildSnapshot()
	s.dirty.Store(false)

	return loadedCount, nil
}

// StartAutoSave periodically flushes modified store data to disk in the background.
func (s *Store) StartAutoSave(ctx context.Context, interval time.Duration, path string) {
	if path == "" || interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.dirty.Load() {
				if err := s.SaveToFile(path); err != nil {
					log.Printf("[Store] Auto-save error: %v", err)
				}
			}
		}
	}
}
