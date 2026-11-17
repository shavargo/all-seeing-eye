package listener

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"

	"master-server/internal/registry"
)

// IDTech3PortConfig defines a listening port and its associated default game.
type IDTech3PortConfig struct {
	Port        int
	DefaultGame string
	Label       string
}

// DefaultIDTech3Ports contains the standard ports for idTech 3 and Activision titles.
var DefaultIDTech3Ports = []IDTech3PortConfig{
	{Port: 27950, DefaultGame: "quake3", Label: "Quake 3 / idTech 3 / OpenArena"},
	{Port: 20510, DefaultGame: "callofduty", Label: "Call of Duty"},
	{Port: 20610, DefaultGame: "coduo", Label: "Call of Duty: United Offensive"},
	{Port: 20710, DefaultGame: "cod2", Label: "Call of Duty 2"},
	{Port: 20810, DefaultGame: "cod4", Label: "Call of Duty 4: Modern Warfare"},
	{Port: 28060, DefaultGame: "jk2", Label: "Star Wars Jedi Knight II: Jedi Outcast"},
	{Port: 29060, DefaultGame: "jka", Label: "Star Wars Jedi Knight: Jedi Academy"},
	{Port: 20110, DefaultGame: "sof2", Label: "Soldier of Fortune II"},
	{Port: 27953, DefaultGame: "eliteforce", Label: "Star Trek: Voyager Elite Force"},
	{Port: 29250, DefaultGame: "eliteforce2", Label: "Star Trek: Elite Force II"},
	{Port: 27777, DefaultGame: "xonotic", Label: "DarkPlaces / Xonotic / Nexuiz"},
}

// KPQ3Listener handles Quake 3, idTech 3, Call of Duty, and Jedi Knight UDP queries.
type KPQ3Listener struct {
	primaryAddr string
	store       *registry.Store
	ports       []IDTech3PortConfig
	conns       []*net.UDPConn
	mu          sync.Mutex
}

// NewKPQ3Listener creates the Quake 3 / idTech3 multi-port listener.
func NewKPQ3Listener(primaryAddr string, store *registry.Store) *KPQ3Listener {
	return &KPQ3Listener{
		primaryAddr: primaryAddr,
		store:       store,
		ports:       DefaultIDTech3Ports,
	}
}

// Start begins listening on all configured idTech 3 and Activision UDP ports.
func (l *KPQ3Listener) Start(ctx context.Context) error {
	var wg sync.WaitGroup

	for _, cfg := range l.ports {
		portCfg := cfg
		bindAddr := fmt.Sprintf(":%d", portCfg.Port)
		// If primaryAddr overrides default 27950, honor primaryAddr
		if portCfg.Port == 27950 && l.primaryAddr != "" {
			bindAddr = l.primaryAddr
		}

		udpAddr, err := net.ResolveUDPAddr("udp", bindAddr)
		if err != nil {
			log.Printf("[idTech 3] Failed to resolve %s (%s): %v", bindAddr, portCfg.Label, err)
			continue
		}

		conn, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			log.Printf("[idTech 3] Could not bind UDP %s (%s): %v", bindAddr, portCfg.Label, err)
			continue
		}

		l.mu.Lock()
		l.conns = append(l.conns, conn)
		l.mu.Unlock()

		log.Printf("[UDP %d] %s listener active on %s", portCfg.Port, portCfg.Label, bindAddr)

		wg.Add(1)
		go func(c *net.UDPConn, p IDTech3PortConfig) {
			defer wg.Done()
			l.listenOnConn(ctx, c, p)
		}(conn, portCfg)
	}

	go func() {
		<-ctx.Done()
		l.mu.Lock()
		for _, c := range l.conns {
			_ = c.Close()
		}
		l.mu.Unlock()
	}()

	wg.Wait()
	return nil
}

func (l *KPQ3Listener) listenOnConn(ctx context.Context, conn *net.UDPConn, cfg IDTech3PortConfig) {
	buf := make([]byte, 4096)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}

		if n <= 0 {
			continue
		}

		packet := make([]byte, n)
		copy(packet, buf[:n])

		go l.handlePacket(conn, packet, remoteAddr, cfg.DefaultGame)
	}
}

const maxServersPerQ3Packet = 112

func (l *KPQ3Listener) handlePacket(conn *net.UDPConn, packet []byte, remoteAddr *net.UDPAddr, defaultGame string) {
	text := string(packet)
	// Strip Quake 0xFF 0xFF 0xFF 0xFF prefix if present, plus trailing C null bytes and whitespace
	text = strings.TrimPrefix(text, "\xff\xff\xff\xff")
	text = strings.Trim(text, "\x00 \t\r\n")
	lower := strings.ToLower(text)

	// Graceful shutdown from idTech 3 server: "flatline" or "shutdown"
	if strings.HasPrefix(lower, "flatline") || strings.HasPrefix(lower, "shutdown") {
		l.store.RemoveServer(remoteAddr.IP.String(), remoteAddr.Port, defaultGame)
		return
	}

	// In-game client query: getservers [gamename/protocol] [protocol] [empty] [full]
	if strings.HasPrefix(lower, "getservers") {
		parts := strings.Fields(text)
		game := defaultGame
		if len(parts) > 1 {
			candidate := strings.ToLower(strings.Trim(parts[1], "\x00"))
			// If not a pure number (protocol version), map engine/mod identifiers
			if _, err := strconv.Atoi(candidate); err != nil && candidate != "empty" && candidate != "full" {
				game = resolveIDTech3Game(candidate, defaultGame)
			}
		}
		l.sendQ3ServerList(conn, remoteAddr, game)
		return
	}

	// Server heartbeat: heartbeat <gamename>\n
	if strings.HasPrefix(lower, "heartbeat") {
		parts := strings.Fields(text)
		game := defaultGame
		if len(parts) > 1 {
			candidate := strings.ToLower(strings.Trim(parts[1], "\x00"))
			if candidate == "flatline" || candidate == "dead" || candidate == "shutdown" {
				l.store.RemoveServer(remoteAddr.IP.String(), remoteAddr.Port, defaultGame)
				return
			}
			if _, err := strconv.Atoi(candidate); err != nil {
				game = resolveIDTech3Game(candidate, defaultGame)
			}
		}
		if !l.store.IsBlacklisted(remoteAddr.IP.String()) {
			l.store.RegisterHeartbeat(remoteAddr.IP.String(), remoteAddr.Port, game)
		}
		return
	}
}

func resolveIDTech3Game(candidate, defaultGame string) string {
	switch candidate {
	case "kingpinq3-1":
		return "kingpin"
	case "darkplaces":
		if defaultGame == "xonotic" {
			return "xonotic"
		}
		return "kingpin"
	case "quakearena-1", "q3a", "quake3arena":
		return "quake3"
	case "wolf-1", "wolfmp":
		return "rtcw"
	case "et-1", "enemyterritory", "wolfet":
		return "et"
	case "cod-1":
		return "cod"
	case "coduo-1":
		return "coduo"
	case "cod2-1":
		return "cod2"
	case "cod4-1":
		return "cod4"
	case "jk2-1", "jedioutcast":
		return "jk2"
	case "jka-1", "jediacademy":
		return "jka"
	case "sof2-1", "sof2mp-1":
		return "sof2"
	case "ef-1", "stvef-1":
		return "eliteforce"
	default:
		return candidate
	}
}

func (l *KPQ3Listener) sendQ3ServerList(conn *net.UDPConn, remoteAddr *net.UDPAddr, gamename string) {
	servers := l.store.GetServersByGame(gamename)

	// Filter to valid IPv4 servers
	type addrEntry struct {
		ip   net.IP
		port uint16
	}
	valid := make([]addrEntry, 0, len(servers))
	for _, srv := range servers {
		ip := net.ParseIP(srv.IP).To4()
		if ip == nil {
			continue
		}
		port := srv.GamePort
		if port <= 0 {
			port = srv.QueryPort
		}
		valid = append(valid, addrEntry{ip: ip, port: uint16(port)})
	}

	if len(valid) == 0 {
		_, _ = conn.WriteToUDP([]byte("\xff\xff\xff\xffgetserversResponse\\EOT"), remoteAddr)
		return
	}

	// Send in MTU-safe chunks (<= 112 servers per UDP packet, ~810 bytes)
	// so packets never fragment on the internet or exceed Quake 3's MAX_SERVERSPERPACKET (256).
	for start := 0; start < len(valid); start += maxServersPerQ3Packet {
		end := start + maxServersPerQ3Packet
		if end > len(valid) {
			end = len(valid)
		}

		var buf bytes.Buffer
		buf.Write([]byte("\xff\xff\xff\xffgetserversResponse"))
		for _, entry := range valid[start:end] {
			buf.WriteByte('\\')
			buf.Write(entry.ip)
			var portBytes [2]byte
			binary.BigEndian.PutUint16(portBytes[:], entry.port)
			buf.Write(portBytes[:])
		}
		buf.Write([]byte("\\EOT"))
		_, _ = conn.WriteToUDP(buf.Bytes(), remoteAddr)
	}
}
