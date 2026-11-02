package seeder

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"master-server/internal/games"
	"master-server/internal/registry"
)

// MasterPeer represents an upstream UDP master server to sync from.
type MasterPeer struct {
	Game     string
	Protocol string // "quake2" or "quake3"
	Host     string
	Port     int
}

// ServerSeedEntry represents an entry in structured JSON mirrors.
type ServerSeedEntry struct {
	Game      string `json:"game"`
	IP        string `json:"ip"`
	Port      int    `json:"port"`
	QueryPort int    `json:"queryPort,omitempty"`
}

// RetroMirrorIndex represents the master catalog JSON schema from retro-server-mirror.
type RetroMirrorIndex struct {
	UpdatedAt        string                          `json:"updated_at"`
	TotalServers     int                             `json:"total_servers"`
	TotalGames       int                             `json:"total_games"`
	ActiveGamesCount int                             `json:"active_games_count"`
	Games            map[string]RetroMirrorGameEntry `json:"games"`
}

// RetroMirrorGameEntry represents each game entry in servers.json.
type RetroMirrorGameEntry struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Protocol    string `json:"protocol"`
	ServerCount int    `json:"server_count"`
	File        string `json:"file"`
}

// Seeder periodically fetches remote server lists and upstream masters.
type Seeder struct {
	store          *registry.Store
	gameReg        *games.Registry
	urls           []string
	peers          []MasterPeer
	interval       time.Duration
	client         *http.Client
	peerSyncActive bool
	onSyncComplete func()
}

// NewSeeder creates a new seeder.
func NewSeeder(store *registry.Store, gameReg *games.Registry, urls []string, interval time.Duration) *Seeder {
	// Standard upstream community masters
	peers := []MasterPeer{
		{Game: "quake2", Protocol: "quake2", Host: "master.quakeservers.net", Port: 27900},
		{Game: "quake2", Protocol: "quake2", Host: "master.maraakate.org", Port: 27900},
		{Game: "quake3", Protocol: "quake3", Host: "master.ioquake3.org", Port: 27950},
		{Game: "alienarena", Protocol: "quake2", Host: "master.alienarena.org", Port: 27900},
	}

	// Allow disabling direct upstream master queries via env var
	disableDirect := strings.EqualFold(os.Getenv("DISABLE_DIRECT_PEER_QUERY"), "true") ||
		strings.EqualFold(os.Getenv("MIRROR_ONLY"), "true")

	return &Seeder{
		store:          store,
		gameReg:        gameReg,
		urls:           urls,
		peers:          peers,
		interval:       interval,
		peerSyncActive: !disableDirect,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// SetOnSyncComplete sets a callback invoked after every full sync pass.
func (s *Seeder) SetOnSyncComplete(cb func()) {
	s.onSyncComplete = cb
}

// SyncOnce runs an immediate synchronization across all configured mirrors and peers.
func (s *Seeder) SyncOnce() {
	s.syncAll()
}

// Start begins the periodic fetching loop.
func (s *Seeder) Start(ctx context.Context) {
	s.syncAll()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncAll()
		}
	}
}

func (s *Seeder) syncAll() {
	// 1. Fetch HTTP/HTTPS and local file mirrors
	for _, rawURL := range s.urls {
		trimmed := strings.TrimSpace(rawURL)
		if trimmed == "" {
			continue
		}
		if err := s.fetchURL(trimmed); err != nil {
			log.Printf("[Seeder] Error fetching %s: %v", trimmed, err)
		}
	}

	// 2. Query upstream Quake 2 / Quake 3 masters (if enabled)
	if s.peerSyncActive {
		for _, peer := range s.peers {
			if err := s.queryUpstreamPeer(peer); err != nil {
				log.Printf("[Seeder] Peer sync %s (%s:%d): %v", peer.Game, peer.Host, peer.Port, err)
			}
		}
	}

	if s.onSyncComplete != nil {
		s.onSyncComplete()
	}
}

func (s *Seeder) queryUpstreamPeer(peer MasterPeer) error {
	addr := net.JoinHostPort(peer.Host, strconv.Itoa(peer.Port))
	conn, err := net.DialTimeout("udp", addr, 3*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	if peer.Protocol == "quake2" {
		payload := append([]byte{0xFF, 0xFF, 0xFF, 0xFF}, []byte("query\n")...)
		if _, err := conn.Write(payload); err != nil {
			return err
		}
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		buf := make([]byte, 8192)
		n, err := conn.Read(buf)
		if err != nil {
			return err
		}
		s.parseQuake2ServerList(buf[:n], peer.Game)
		return nil
	}

	if peer.Protocol == "quake3" {
		payload := append([]byte{0xFF, 0xFF, 0xFF, 0xFF}, []byte("getservers 68 empty full\n")...)
		if _, err := conn.Write(payload); err != nil {
			return err
		}
		_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		buf := make([]byte, 8192)
		n, err := conn.Read(buf)
		if err != nil {
			return err
		}
		s.parseQuake3ServerList(buf[:n], peer.Game)
		return nil
	}

	return nil
}

func (s *Seeder) parseQuake2ServerList(data []byte, game string) {
	header := []byte("\xff\xff\xff\xffservers\n")
	idx := bytes.Index(data, header)
	if idx == -1 {
		return
	}
	offset := idx + len(header)
	count := 0
	for i := offset; i+6 <= len(data); i += 6 {
		ip := net.IPv4(data[i], data[i+1], data[i+2], data[i+3]).String()
		port := int(binary.BigEndian.Uint16(data[i+4 : i+6]))
		if port == 0 {
			continue
		}
		if s.registerEntry(game, ip, port, 0) {
			count++
		}
	}
	if count > 0 {
		log.Printf("[Seeder] Discovered %d %s servers from upstream master", count, game)
	}
}

func (s *Seeder) parseQuake3ServerList(data []byte, game string) {
	header := []byte("\xff\xff\xff\xffgetserversResponse")
	idx := bytes.Index(data, header)
	if idx == -1 {
		return
	}
	offset := idx + len(header)
	count := 0
	for i := offset; i+7 <= len(data); {
		if data[i] == '\\' {
			i++
		}
		if i+6 > len(data) {
			break
		}
		if bytes.HasPrefix(data[i:], []byte("EOT")) {
			break
		}
		ip := net.IPv4(data[i], data[i+1], data[i+2], data[i+3]).String()
		port := int(binary.BigEndian.Uint16(data[i+4 : i+6]))
		i += 6
		if port == 0 {
			continue
		}
		if s.registerEntry(game, ip, port, 0) {
			count++
		}
	}
	if count > 0 {
		log.Printf("[Seeder] Discovered %d %s servers from upstream master", count, game)
	}
}

// fetchBytes retrieves payload either via HTTP(S) with no-cache headers or from the local filesystem.
func (s *Seeder) fetchBytes(target string) ([]byte, error) {
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)
		if err != nil {
			return nil, fmt.Errorf("create http request: %w", err)
		}
		// Bypass GitHub Raw / Fastly CDN cache
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Pragma", "no-cache")
		req.Header.Set("User-Agent", "UniversalMasterServer-Seeder/1.0")

		resp, err := s.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http get: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
		}
		return io.ReadAll(resp.Body)
	}

	// Local file path
	filePath := strings.TrimPrefix(target, "file://")
	return os.ReadFile(filePath)
}

// fetchURL fetches and parses mirror lists supporting retro-server-mirror, JSON maps, JSON arrays, local files, and text lists.
func (s *Seeder) fetchURL(target string) error {
	body, err := s.fetchBytes(target)
	if err != nil {
		return err
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil
	}

	// 1. Try JSON Object formats
	if trimmed[0] == '{' {
		// Sub-format A: retro-server-mirror central servers.json
		var mirrorIdx RetroMirrorIndex
		if err := json.Unmarshal(trimmed, &mirrorIdx); err == nil && len(mirrorIdx.Games) > 0 {
			return s.ingestRetroMirror(target, mirrorIdx)
		}

		// Sub-format B: {"kingpin": ["1.2.3.4:31510", ...], "quake2": [...]}
		var strMap map[string][]string
		if err := json.Unmarshal(trimmed, &strMap); err == nil && len(strMap) > 0 {
			count := 0
			for game, addrList := range strMap {
				for _, addr := range addrList {
					ip, port, err := net.SplitHostPort(addr)
					if err != nil {
						continue
					}
					p, _ := strconv.Atoi(port)
					if s.registerEntry(game, ip, p, 0) {
						count++
					}
				}
			}
			log.Printf("[Seeder] Imported %d servers from JSON map (%s)", count, target)
			return nil
		}

		// Sub-format C: {"kingpin": [{"ip":"...", "port": 31510}]}
		var objMap map[string][]ServerSeedEntry
		if err := json.Unmarshal(trimmed, &objMap); err == nil && len(objMap) > 0 {
			count := 0
			for game, entryList := range objMap {
				for _, entry := range entryList {
					g := entry.Game
					if g == "" {
						g = game
					}
					if s.registerEntry(g, entry.IP, entry.Port, entry.QueryPort) {
						count++
					}
				}
			}
			log.Printf("[Seeder] Imported %d servers from JSON object map (%s)", count, target)
			return nil
		}
	}

	// 2. Try JSON Array of ServerSeedEntry: [{"game":"...", "ip":"...", "port":...}]
	if trimmed[0] == '[' {
		var list []ServerSeedEntry
		if err := json.Unmarshal(trimmed, &list); err == nil && len(list) > 0 {
			count := 0
			for _, entry := range list {
				if s.registerEntry(entry.Game, entry.IP, entry.Port, entry.QueryPort) {
					count++
				}
			}
			log.Printf("[Seeder] Imported %d servers from JSON array (%s)", count, target)
			return nil
		}
	}

	// 3. Fallback: Line-by-line text parser
	return s.parseTextList(target, trimmed, "kingpin")
}

// ingestRetroMirror parses servers.json and fetches all active games concurrently.
func (s *Seeder) ingestRetroMirror(target string, idx RetroMirrorIndex) error {
	var baseURL string
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		lastSlash := strings.LastIndex(target, "/")
		if lastSlash != -1 {
			baseURL = target[:lastSlash+1]
		} else {
			baseURL = target + "/"
		}
	} else {
		filePath := strings.TrimPrefix(target, "file://")
		baseURL = filepath.Dir(filePath) + string(filepath.Separator)
	}

	type gameSeedJob struct {
		gameKey string
		fileURL string
	}

	var jobs []gameSeedJob
	for gameKey, entry := range idx.Games {
		if entry.ServerCount > 0 && entry.File != "" {
			fileURL := entry.File
			if !strings.HasPrefix(fileURL, "http://") && !strings.HasPrefix(fileURL, "https://") {
				if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
					fileURL = baseURL + strings.TrimPrefix(entry.File, "/")
				} else {
					fileURL = filepath.Join(strings.TrimPrefix(baseURL, "file://"), entry.File)
				}
			}
			jobs = append(jobs, gameSeedJob{
				gameKey: gameKey,
				fileURL: fileURL,
			})
		}
	}

	if len(jobs) == 0 {
		log.Printf("[Seeder] retro-server-mirror at %s had no active games", target)
		return nil
	}

	var totalImported atomic.Int64
	var wg sync.WaitGroup
	jobChan := make(chan gameSeedJob, len(jobs))
	for _, j := range jobs {
		jobChan <- j
	}
	close(jobChan)

	numWorkers := 16
	if len(jobs) < numWorkers {
		numWorkers = len(jobs)
	}

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobChan {
				n, err := s.fetchGameTextList(job.gameKey, job.fileURL)
				if err != nil {
					log.Printf("[Seeder] Error loading %s list (%s): %v", job.gameKey, job.fileURL, err)
				} else {
					totalImported.Add(int64(n))
				}
			}
		}()
	}
	wg.Wait()

	log.Printf("[Seeder] Ingested retro-server-mirror: %d active games, %d servers imported from %s", len(jobs), totalImported.Load(), target)
	return nil
}

// fetchGameTextList loads a single per-game IP:port list.
func (s *Seeder) fetchGameTextList(gameKey, fileURL string) (int, error) {
	body, err := s.fetchBytes(fileURL)
	if err != nil {
		return 0, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(body))
	count := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		addr := fields[0]
		ip, portStr, err := net.SplitHostPort(addr)
		if err != nil {
			continue
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 || port > 65535 {
			continue
		}
		if s.registerEntry(gameKey, ip, port, 0) {
			count++
		}
	}
	return count, scanner.Err()
}

func (s *Seeder) parseTextList(target string, body []byte, defaultGame string) error {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	count := 0
	currentGame := defaultGame

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		// Directive: # @game: <gamename> or [gamename]
		if strings.HasPrefix(line, "# @game:") || strings.HasPrefix(line, "# game:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				currentGame = strings.ToLower(strings.TrimSpace(parts[1]))
			}
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentGame = strings.ToLower(strings.Trim(line, "[] \t"))
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Format: "gamename ip:port" or "ip:port" or "ip:port gamename" or CSV "gamename,ip,port"
		var game string
		var addr string

		if strings.Contains(line, ",") {
			parts := strings.Split(line, ",")
			if len(parts) >= 3 {
				game = strings.TrimSpace(parts[0])
				ip := strings.TrimSpace(parts[1])
				p, _ := strconv.Atoi(strings.TrimSpace(parts[2]))
				if s.registerEntry(game, ip, p, 0) {
					count++
				}
				continue
			}
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			if strings.Contains(parts[0], ":") {
				addr = parts[0]
				game = parts[1]
			} else if strings.Contains(parts[1], ":") {
				game = parts[0]
				addr = parts[1]
			}
		} else if len(parts) == 1 {
			addr = parts[0]
			game = currentGame
		}

		if addr == "" {
			continue
		}

		ip, portStr, err := net.SplitHostPort(addr)
		if err != nil {
			continue
		}
		port, err := strconv.Atoi(portStr)
		if err != nil || port <= 0 || port > 65535 {
			continue
		}

		if s.registerEntry(game, ip, port, 0) {
			count++
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	log.Printf("[Seeder] Imported %d servers from text mirror (%s)", count, target)
	return nil
}

// registerEntry handles game-specific port offsets and registers into the store.
func (s *Seeder) registerEntry(game, ip string, port, queryPort int) bool {
	if ip == "" || port <= 0 || port > 65535 {
		return false
	}
	if game == "" {
		game = "kingpin"
	}
	game = strings.ToLower(game)

	qPort := queryPort
	if qPort <= 0 {
		qPort = port
		// Kingpin dual-port standard: GamePort 31510..31599 -> QueryPort 31500..31589
		if game == "kingpin" && port >= 31510 && port <= 31599 {
			qPort = port - 10
		} else if (game == "bf1942" || game == "bfield1942") && port == 14567 {
			qPort = 23000
		} else if game == "bfvietnam" && port == 15667 {
			qPort = 23000
		} else if s.gameReg != nil {
			offset := s.gameReg.QueryPortOffset(game)
			if offset == -10 && port >= 31510 && port <= 31599 {
				qPort = port - 10
			} else if offset == 8433 && port == 14567 {
				qPort = 23000
			}
		}
	}

	srv, _ := s.store.RegisterHeartbeat(ip, qPort, game)
	if srv != nil {
		srv.GamePort = port
		return true
	}
	return false
}
