package registry

import (
	"strings"
	"time"
)

// Player represents a live connected player on a game server.
type Player struct {
	Name  string `json:"name"`
	Frags int    `json:"frags"`
	Ping  int    `json:"ping"`
}

// Server represents an indexed game server.
type Server struct {
	ID            string            `json:"id"`
	IP            string            `json:"ip"`
	QueryPort     int               `json:"queryPort"`
	GamePort      int               `json:"gamePort"`
	Gamename      string            `json:"gamename"`
	Hostname      string            `json:"hostname"`
	Map           string            `json:"map"`
	Gametype      string            `json:"gametype"`
	Players       int               `json:"players"`
	MaxPlayers    int               `json:"maxPlayers"`
	Ping          int               `json:"ping"`
	Verified      bool              `json:"verified"`
	LastHeartbeat time.Time         `json:"lastHeartbeat"`
	LastChecked   time.Time         `json:"lastChecked"`
	Roster        []Player          `json:"roster"`
	CVars         map[string]string `json:"cvars"`
}

// Snapshot represents a public pre-computed view of active servers.
type Snapshot struct {
	GeneratedAt  time.Time `json:"generatedAt"`
	TotalServers int       `json:"totalServers"`
	TotalPlayers int       `json:"totalPlayers"`
	ActiveGames  int       `json:"activeGames"`
	Servers      []Server  `json:"servers"`
}

// DatabaseRecord represents the top-level structure of the JSON database on disk.
type DatabaseRecord struct {
	SavedAt     time.Time `json:"savedAt"`
	Version     int       `json:"version"`
	ServerCount int       `json:"serverCount"`
	Servers     []*Server `json:"servers"`
}

// ClassifyKingpinMode detects whether a server is Bagman, CTF, DM, or Team.
func ClassifyKingpinMode(name, mapname string, cvars map[string]string) string {
	n := strings.ToLower(name)
	m := strings.ToLower(mapname)

	if strings.Contains(n, "ctf") || strings.Contains(m, "ctf") || cvars["ctf"] == "1" {
		return "CTF"
	}
	if strings.Contains(n, "bagman") || strings.Contains(n, "bag man") || strings.HasPrefix(m, "kpb") || cvars["bagman"] == "1" {
		return "Bagman"
	}
	if cvars["teamplay"] == "1" {
		return "Team"
	}
	if cvars["deathmatch"] == "1" || strings.Contains(n, "dm") || strings.HasPrefix(m, "kpm") {
		return "DM"
	}
	return "Other"
}

// StripQuakeColors removes ^0 through ^9 color codes from Quake/Kingpin strings.
func StripQuakeColors(s string) string {
	var sb strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '^' && i+1 < len(runes) {
			next := runes[i+1]
			if (next >= '0' && next <= '9') || (next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z') {
				i++ // skip the color code
				continue
			}
		}
		sb.WriteRune(runes[i])
	}
	return strings.TrimSpace(sb.String())
}
