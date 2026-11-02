package listener

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strings"

	"master-server/internal/registry"
)

// KPQ3Listener handles Quake 3 / idTech3 / DarkPlaces UDP queries on port 27950.
type KPQ3Listener struct {
	addr  string
	store *registry.Store
	conn  *net.UDPConn
}

// NewKPQ3Listener creates the Quake 3 / idTech3 listener.
func NewKPQ3Listener(addr string, store *registry.Store) *KPQ3Listener {
	return &KPQ3Listener{
		addr:  addr,
		store: store,
	}
}

// Start begins listening on port 27950.
func (l *KPQ3Listener) Start(ctx context.Context) error {
	udpAddr, err := net.ResolveUDPAddr("udp", l.addr)
	if err != nil {
		return fmt.Errorf("resolve q3 udp addr: %w", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("listen q3 udp %s: %w", l.addr, err)
	}
	l.conn = conn
	log.Printf("[UDP 27950] Quake 3 / idTech3 / KPQ3 listener active on %s", l.addr)

	go func() {
		<-ctx.Done()
		l.conn.Close()
	}()

	buf := make([]byte, 4096)
	for {
		n, remoteAddr, err := l.conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				continue
			}
		}

		if n <= 0 {
			continue
		}

		packet := make([]byte, n)
		copy(packet, buf[:n])

		go l.handlePacket(packet, remoteAddr)
	}
}

func (l *KPQ3Listener) handlePacket(packet []byte, remoteAddr *net.UDPAddr) {
	text := string(packet)
	// Strip Quake 0xFF 0xFF 0xFF 0xFF if present
	text = strings.TrimPrefix(text, "\xff\xff\xff\xff")
	text = strings.TrimSpace(text)

	// In-game client query: getservers [gamename] [protocol] [empty] [full]
	if strings.HasPrefix(text, "getservers") {
		parts := strings.Fields(text)
		game := "kingpin"
		if len(parts) > 1 {
			candidate := strings.ToLower(parts[1])
			// If not a pure number (which is a protocol version), use as gamename
			if candidate == "kingpinq3-1" || candidate == "darkplaces" {
				game = "kingpin"
			} else if candidate == "quakearena-1" {
				game = "quake3"
			} else if candidate == "wolf-1" {
				game = "rtcw"
			} else if candidate == "et-1" {
				game = "et"
			} else if candidate == "cod-1" {
				game = "cod"
			} else if candidate != "" {
				game = candidate
			}
		}
		l.sendQ3ServerList(remoteAddr, game)
		return
	}

	// Server heartbeat: heartbeat <gamename>\n
	if strings.HasPrefix(text, "heartbeat") {
		parts := strings.Fields(text)
		game := "kingpin"
		if len(parts) > 1 {
			candidate := strings.ToLower(parts[1])
			if candidate == "kingpinq3-1" || candidate == "darkplaces" {
				game = "kingpin"
			} else if candidate == "quakearena-1" {
				game = "quake3"
			} else if candidate == "wolf-1" {
				game = "rtcw"
			} else if candidate == "et-1" {
				game = "et"
			} else if candidate != "" {
				game = candidate
			}
		}
		l.store.RegisterHeartbeat(remoteAddr.IP.String(), remoteAddr.Port, game)
		return
	}
}

func (l *KPQ3Listener) sendQ3ServerList(remoteAddr *net.UDPAddr, gamename string) {
	servers := l.store.GetServersByGame(gamename)
	if len(servers) == 0 && gamename == "kingpin" {
		servers = l.store.GetServersByGame("kingpin")
	}

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

	_, _ = l.conn.WriteToUDP(buf.Bytes(), remoteAddr)
}
