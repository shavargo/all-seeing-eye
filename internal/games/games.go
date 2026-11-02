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
		"deusex",
		"rune",
		"wot",
		"undying",
		"mobileforces",
		"quake2",
		"quake3",
		"alienarena",
		"sin",
		"rtcw",
		"et",
		"cod",
		"cod2",
		"cod4",
		"coduo",
		"mohaa",
		"sof2",
		"jk2",
		"jka",
		"eliteforce",
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
	if meta, ok := r.lookoutMeta[lower]; ok && meta.Protocol != "" {
		return strings.ToLower(meta.Protocol)
	}
	if g, ok := r.games[lower]; ok && g.Protocol != "" {
		return strings.ToLower(g.Protocol)
	}
	return ""
}

// QueryPortOffset returns any known query port offset for the game (e.g. BF1942: 8433, Kingpin: -10).
func (r *Registry) QueryPortOffset(gamename string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	lower := strings.ToLower(gamename)
	if meta, ok := r.lookoutMeta[lower]; ok && meta.QueryPortOffset != 0 {
		return meta.QueryPortOffset
	}
	if lower == "kingpin" {
		return -10
	}
	return 0
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
