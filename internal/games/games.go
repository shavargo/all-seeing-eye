package games

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

//go:embed SupportedGames.json
var supportedGamesJSON []byte

//go:embed lookout_metadata.json
var lookoutMetadataJSON []byte

// Game represents a game supported by the GameSpy master protocol.
type Game struct {
	Gamename string `json:"gamename"`
	Cipher   string `json:"cipher"`
	Label    string `json:"label"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol,omitempty"`
	Category string `json:"category,omitempty"`
}

// Mode represents a specific game mode configuration.
type Mode struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Label string `json:"label"`
}

// MasterRef represents an upstream master server.
type MasterRef struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

// LookoutGameMeta holds enriched metadata from lookout-games.
type LookoutGameMeta struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Protocol        string            `json:"protocol"`
	QueryPortOffset int               `json:"queryPortOffset"`
	Modes           []Mode            `json:"modes"`
	Keys            map[string]any    `json:"keys"`
	Masters         []MasterRef       `json:"masters"`
	ProtocolOptions map[string]string `json:"protocolOptions"`
}

// Registry manages the catalog of games, ciphers, and validation rules.
type Registry struct {
	mu           sync.RWMutex
	games        map[string]Game
	lookoutMeta  map[string]LookoutGameMeta
	skipValidate map[string]bool
}

// NewRegistry initializes and loads the embedded catalogs.
func NewRegistry() (*Registry, error) {
	var list []Game
	if err := json.Unmarshal(supportedGamesJSON, &list); err != nil {
		return nil, fmt.Errorf("failed to parse SupportedGames.json: %w", err)
	}

	var lookoutMap map[string]LookoutGameMeta
	if err := json.Unmarshal(lookoutMetadataJSON, &lookoutMap); err != nil {
		return nil, fmt.Errorf("failed to parse lookout_metadata.json: %w", err)
	}

	r := &Registry{
		games:        make(map[string]Game, len(list)),
		lookoutMeta:  make(map[string]LookoutGameMeta, len(lookoutMap)),
		skipValidate: make(map[string]bool),
	}

	for _, g := range list {
		lower := strings.ToLower(g.Gamename)
		r.games[lower] = g
	}

	for k, meta := range lookoutMap {
		r.lookoutMeta[strings.ToLower(k)] = meta
		// Also index under the GameSpy master game name if specified in protocol options
		if mg, ok := meta.ProtocolOptions["masterGame"]; ok && mg != "" {
			r.lookoutMeta[strings.ToLower(mg)] = meta
		}
	}

	// Games that deviate from or do not support \secure\ / \validate\ challenge
	// Kingpin, Deus Ex, Rune, and Quake 2/3 games are explicitly bypassed
	overrides := []string{
		"kingpin",
		"kp",
		"deusex",
		"rune",
		"wot",
		"undying",
		"mobileforces",
		"quake2",
		"q2",
		"quake3",
		"q3a",
		"quake3arena",
		"alienarena",
		"sin",
		"rtcw",
		"wolf",
		"wolfmp",
		"et",
		"wolfet",
		"enemyterritory",
		"cod",
		"callofduty",
		"cod2",
		"callofduty2",
		"cod4",
		"callofduty4",
		"callofduty4modernwarfare",
		"coduo",
		"callofdutyuo",
		"mohaa",
		"mohaab",
		"mohaas",
		"sof2",
		"jk2",
		"jka",
		"eliteforce",
		"eliteforce2",
		"ef2",
		"openarena",
		"nexuiz",
		"warsow",
		"xonotic",
		"urbanterror",
		"smokinguns",
		"tremulous",
		"unvanquished",
		"bf1942",
		"bfield1942",
		"bfvietnam",
	}
	for _, o := range overrides {
		r.skipValidate[o] = true
	}

	return r, nil
}

// Lookup finds a game by gamename (case-insensitive).
func (r *Registry) Lookup(name string) (Game, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.games[strings.ToLower(name)]
	return g, ok
}

// LookupMeta returns enriched metadata from lookout-games if available.
func (r *Registry) LookupMeta(name string) (LookoutGameMeta, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	meta, ok := r.lookoutMeta[strings.ToLower(name)]
	return meta, ok
}

// ResolveDisplayName returns the best human-readable name for a game.
func (r *Registry) ResolveDisplayName(name string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	lower := strings.ToLower(name)
	if meta, ok := r.lookoutMeta[lower]; ok && meta.Name != "" {
		return meta.Name
	}
	if g, ok := r.games[lower]; ok && g.Label != "" {
		return g.Label
	}
	return strings.ToUpper(name)
}

// ResolveMode resolves mode labels using lookout-games rules.
func (r *Registry) ResolveMode(gamename string, cvars map[string]string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	meta, ok := r.lookoutMeta[strings.ToLower(gamename)]
	if !ok || len(meta.Modes) == 0 {
		return ""
	}

	for _, mode := range meta.Modes {
		if val, exists := cvars[strings.ToLower(mode.Key)]; exists && strings.EqualFold(val, mode.Value) {
			return mode.Label
		}
	}
	return ""
}

// ProtocolFor returns the detected communication protocol for a game ("quake2", "quake3", "gamespy1", etc.).
func (r *Registry) ProtocolFor(gamename string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	lower := strings.ToLower(gamename)

	switch lower {
	// Quake 3 / idTech 3 family & aliases
	case "q3a", "quake3arena", "quake3", "wolfet", "enemyterritory", "et", "wolf", "rtcw",
		"jka", "jk2", "sof2", "eliteforce", "urbanterror", "warsow", "smokinguns",
		"tremulous", "unvanquished", "nexuiz", "xonotic", "openarena", "cod", "callofduty",
		"coduo", "cod2", "cod4":
		return "quake3"

	// Quake 2 / idTech 2 family & aliases
	case "q2", "quake2", "alienarena", "heretic2":
		return "quake2"

	// QuakeWorld
	case "qw", "quakeworld":
		return "quakeworld"

	// Medal of Honor: Allied Assault & expansions (speak GameSpy 1 query protocol on GamePort + 97)
	case "mohaa", "mohaab", "mohaas":
		return "gamespy1"

	// Unreal Engine 1 & 2 games (speak GameSpy 1 query protocol on their query port)
	case "ut", "ut99", "tacticalops", "unreal", "deusex", "rune", "postal2", "postal2d",
		"undying", "nerfarena", "wot", "devastation", "klingon", "ut2004", "ut2", "ut2003",
		"serioussam", "serioussamse", "serioussam2", "swat4", "tribesv":
		return "gamespy1"

	// Battlefield & Vietcong (speak GameSpy 1 query protocol on dedicated query ports)
	case "bf1942", "bfield1942", "bfvietnam", "vietcong", "vietcong2", "kingpin", "sin":
		return "gamespy1"
	}

	if meta, ok := r.lookoutMeta[lower]; ok && meta.Protocol != "" {
		return strings.ToLower(meta.Protocol)
	}
	if g, ok := r.games[lower]; ok && g.Protocol != "" {
		return strings.ToLower(g.Protocol)
	}
	return ""
}

// QueryPortOffset returns any known query port offset for the game (e.g. BF1942: 8433, Kingpin: -10, UT99: +1, UT2004: +10, MOHAA: +97, Vietcong: +10000).
func (r *Registry) QueryPortOffset(gamename string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	lower := strings.ToLower(gamename)

	switch lower {
	// Unreal Engine 1 & 2 games (+1 offset: GamePort 7777 -> QueryPort 7778, SWAT 4: 10480 -> 10481)
	case "ut", "ut99", "unreal", "deusex", "rune", "tacticalops", "postal2", "postal2d",
		"swat4", "serioussam", "serioussamse", "serioussam2", "tribesv", "undying",
		"nerfarena", "wot", "devastation", "klingon":
		return 1

	// Unreal Engine 2 tournament games (+10 offset: GamePort 7777 -> GameSpy QueryPort 7787)
	case "ut2004", "ut2", "ut2003":
		return 10

	// Kingpin dual-port standard (-10 offset: 31510..31599 -> 31500..31589)
	case "kingpin":
		return -10

	// Sin (-1 offset: 22450 -> 22449)
	case "sin":
		return -1

	// Medal of Honor: Allied Assault / Spearhead / Breakthrough (+97 offset: 12203 -> 12300)
	case "mohaa", "mohaab", "mohaas":
		return 97

	// Battlefield 1942 (14567 -> 23000)
	case "bf1942", "bfield1942":
		return 8433

	// Battlefield: Vietnam (15667 -> 23000)
	case "bfvietnam":
		return 7333

	// Vietcong (5425 -> 15425)
	case "vietcong", "vietcong2":
		return 10000
	}

	if meta, ok := r.lookoutMeta[lower]; ok && meta.QueryPortOffset != 0 {
		return meta.QueryPortOffset
	}

	return 0
}

// IsPassiveGame returns true if active 24/7 UDP health probing should be disabled for this game.
// These are Unreal Engine games (UT99, UT2004, Tactical Ops, etc.) where continuous polling packets
// can trigger connection logic (<Unconnected Client> / Open MyLevel) or log spam on server consoles,
// or where community preference is passive indexing without unsolicited outbound UDP probes.
func IsPassiveGame(gamename string) bool {
	switch strings.ToLower(gamename) {
	case "ut", "ut99", "ut2", "ut2003", "ut2004", "tacticalops", "unreal", "rune", "deusex", "postal2", "postal2d":
		return true
	default:
		return false
	}
}

// SkipsValidation returns true if this game does not support \secure\ validation.
func (r *Registry) SkipsValidation(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.skipValidate[strings.ToLower(name)]
}

// TotalCount returns the number of registered games.
func (r *Registry) TotalCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.games)
}

// AllGames returns all registered games.
func (r *Registry) AllGames() []Game {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]Game, 0, len(r.games))
	for _, g := range r.games {
		res = append(res, g)
	}
	return res
}

// Search searches games by name/label and optional category with a limit.
func (r *Registry) Search(query string, category string, limit int) []Game {
	r.mu.RLock()
	defer r.mu.RUnlock()

	q := strings.ToLower(strings.TrimSpace(query))
	cat := strings.ToLower(strings.TrimSpace(category))
	var matches []Game

	for _, g := range r.games {
		if cat != "" && cat != "all" && strings.ToLower(g.Category) != cat {
			continue
		}
		if q == "" || strings.Contains(strings.ToLower(g.Label), q) || strings.Contains(strings.ToLower(g.Gamename), q) {
			matches = append(matches, g)
			if limit > 0 && len(matches) >= limit {
				break
			}
		}
	}
	return matches
}
