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

	"master-server/internal/games"
	"master-server/internal/gamespy"
	"master-server/internal/registry"
)

// DualUDPListener listens on port 27900 and demuxes between GameSpy and Quake 2.
type DualUDPListener struct {
	addr     string
	store    *registry.Store
	registry *games.Registry
	conn     *net.UDPConn
}

// NewDualUDPListener initializes the dual-protocol UDP listener.
func NewDualUDPListener(addr string, store *registry.Store, reg *games.Registry) *DualUDPListener {
	return &DualUDPListener{
		addr:     addr,
		store:    store,
		registry: reg,
	}
}

// Start begins receiving UDP packets.
func (l *DualUDPListener) Start(ctx context.Context) error {
	udpAddr, err := net.ResolveUDPAddr("udp", l.addr)
	if err != nil {
		return fmt.Errorf("resolve udp addr: %w", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("listen udp %s: %w", l.addr, err)
	}
	l.conn = conn
	log.Printf("[UDP 27900] Dual-protocol listener active on %s", l.addr)

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
				log.Printf("[UDP 27900] Read error: %v", err)
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

func (l *DualUDPListener) handlePacket(packet []byte, remoteAddr *net.UDPAddr) {
	// 1. Quake 2 / Kingpin out-of-band packet (starts with 0xFF 0xFF 0xFF 0xFF)
	if bytes.HasPrefix(packet, []byte{0xFF, 0xFF, 0xFF, 0xFF}) {
		l.handleQuake2Packet(packet[4:], remoteAddr)
		return
	}

	// 2. GameSpy v0 packet (starts with '\', ASCII 0x5C)
	if packet[0] == '\\' {
		l.handleGameSpyPacket(string(packet), remoteAddr)
		return
	}

	// 3. ASCII Quake text packet (e.g. "heartbeat KingpinQ3-1" or "getservers ...")
	text := string(packet)
	if strings.HasPrefix(text, "getservers ") {
		l.sendQuakeServerList(remoteAddr, "kingpin")
		return
	}
	if strings.HasPrefix(text, "heartbeat ") {
		l.store.RegisterHeartbeat(remoteAddr.IP.String(), remoteAddr.Port, "kingpin")
		return
	}
}

// handleGameSpyPacket handles standard \key\value\ GameSpy packets.
func (l *DualUDPListener) handleGameSpyPacket(raw string, remoteAddr *net.UDPAddr) {
	msg := gamespy.ParseMessage(raw)

	// Heartbeat: \heartbeat\<port>\gamename\<game>
	if msg.Has("heartbeat") {
		portStr := msg.Get("heartbeat")
		port, _ := strconv.Atoi(portStr)
		if port == 0 {
			port = remoteAddr.Port
		}
		gamename := msg.Get("gamename")
		if gamename == "" {
			gamename = "kingpin"
		}

		srv, _ := l.store.RegisterHeartbeat(remoteAddr.IP.String(), port, gamename)
		if srv == nil {
			return
		}

		// If the game does not support \secure\ (e.g. Kingpin, Deus Ex, Rune), we accept directly
		if l.registry.SkipsValidation(gamename) {
			return
		}

		// Otherwise, send GameSpy challenge: \secure\<challenge>
		challenge := "ABCDEF" // 6-char challenge
		resp := fmt.Sprintf("\\secure\\%s", challenge)
		_, _ = l.conn.WriteToUDP([]byte(resp), remoteAddr)
		return
	}

	// Validation response from game server: \validate\<hash>
	if msg.Has("validate") {
		// Validated, mark server
		return
	}

	// Ping/Echo query directed at master: \echo\<msg>
	if msg.Has("echo") {
		resp := fmt.Sprintf("\\echo_reply\\%s", msg.Get("echo"))
		_, _ = l.conn.WriteToUDP([]byte(resp), remoteAddr)
		return
	}

	// Status query directed at master server
	if msg.Has("status") || msg.Has("info") {
		snap := l.store.GetSnapshot()
		resp := fmt.Sprintf("\\status\\online\\servers\\%d\\players\\%d\\final\\",
			snap.TotalServers, snap.TotalPlayers)
		_, _ = l.conn.WriteToUDP([]byte(resp), remoteAddr)
		return
	}
}

// handleQuake2Packet handles Quake 2 / Kingpin in-game binary queries.
func (l *DualUDPListener) handleQuake2Packet(payload []byte, remoteAddr *net.UDPAddr) {
	msg := string(payload)

	// In-game browser query: "query\n" or "getservers"
	if strings.HasPrefix(msg, "query") || strings.HasPrefix(msg, "getservers") {
		l.sendQuakeServerList(remoteAddr, "kingpin")
		return
	}

	// Heartbeat from server: "heartbeat\n"
	if strings.HasPrefix(msg, "heartbeat") {
		l.store.RegisterHeartbeat(remoteAddr.IP.String(), remoteAddr.Port, "kingpin")
		return
	}
}

// sendQuakeServerList sends back the standard Quake 2 binary server list:
// \xff\xff\xff\xffservers\n[IP1(4B)+Port1(2B)][IP2+Port2]...
func (l *DualUDPListener) sendQuakeServerList(remoteAddr *net.UDPAddr, gamename string) {
	servers := l.store.GetServersByGame(gamename)

	var buf bytes.Buffer
	buf.Write([]byte("\xff\xff\xff\xffservers\n"))

	for _, srv := range servers {
		ip := net.ParseIP(srv.IP).To4()
		if ip == nil {
			continue
		}
		buf.Write(ip)

		portBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portBytes, uint16(srv.GamePort))
		buf.Write(portBytes)
	}

	_, _ = l.conn.WriteToUDP(buf.Bytes(), remoteAddr)
}
