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

func (l *KPQ3Listener) handlePacket(conn *net.UDPConn, packet []byte, remoteAddr *net.UDPAddr, defaultGame string) {
	text := string(packet)
	// Strip Quake 0xFF 0xFF 0xFF 0xFF prefix if present
	text = strings.TrimPrefix(text, "\xff\xff\xff\xff")
	text = strings.TrimSpace(text)

	// In-game client query: getservers [gamename/protocol] [protocol] [empty] [full]
	if strings.HasPrefix(text, "getservers") {
		parts := strings.Fields(text)
		game := defaultGame
		if len(parts) > 1 {
			candidate := strings.ToLower(parts[1])
			// If not a pure number (protocol version), map engine/mod identifiers
			if _, err := strconv.Atoi(candidate); err != nil {
				switch candidate {
				case "kingpinq3-1", "darkplaces":
					game = "kingpin"
				case "quakearena-1", "q3a":
					game = "quake3"
				case "wolf-1":
					game = "rtcw"
				case "et-1":
					game = "et"
				case "cod-1":
					game = "cod"
				case "cod2-1":
					game = "cod2"
				case "cod4-1":
					game = "cod4"
				default:
					game = candidate
				}
			}
		}
		l.sendQ3ServerList(conn, remoteAddr, game)
		return
	}

	// Server heartbeat: heartbeat <gamename>\n
	if strings.HasPrefix(text, "heartbeat") {
		parts := strings.Fields(text)
		game := defaultGame
		if len(parts) > 1 {
			candidate := strings.ToLower(parts[1])
			if _, err := strconv.Atoi(candidate); err != nil {
				switch candidate {
				case "kingpinq3-1", "darkplaces":
					game = "kingpin"
				case "quakearena-1", "q3a":
					game = "quake3"
				case "wolf-1":
					game = "rtcw"
				case "et-1":
					game = "et"
				case "cod-1":
					game = "cod"
				case "cod2-1":
					game = "cod2"
				case "cod4-1":
					game = "cod4"
				default:
					game = candidate
				}
			}
		}
		if !l.store.IsBlacklisted(remoteAddr.IP.String()) {
			l.store.RegisterHeartbeat(remoteAddr.IP.String(), remoteAddr.Port, game)
		}
		return
	}
}

func (l *KPQ3Listener) sendQ3ServerList(conn *net.UDPConn, remoteAddr *net.UDPAddr, gamename string) {
	servers := l.store.GetServersByGame(gamename)

	var buf bytes.Buffer
	// Quake 3 getserversResponse header: \xff\xff\xff\xffgetserversResponse\
	buf.Write([]byte("\xff\xff\xff\xffgetserversResponse"))

	for _, srv := range servers {
		ip := net.ParseIP(srv.IP).To4()
		if ip == nil {
			continue
		}
		buf.WriteByte('\\')
		buf.Write(ip)

		portBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portBytes, uint16(srv.GamePort))
		buf.Write(portBytes)
	}
	buf.Write([]byte("\\EOT"))

	_, _ = conn.WriteToUDP(buf.Bytes(), remoteAddr)
}
