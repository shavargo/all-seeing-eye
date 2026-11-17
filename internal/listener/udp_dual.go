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
	"time"

	"master-server/internal/games"
	"master-server/internal/gamespy"
	"master-server/internal/registry"
)

const challengeTTL = 60 * time.Second

type pendingChallenge struct {
	ip         string
	queryPort  int
	sourcePort int
	gamename   string
	cipher     string
	challenge  string
	expiresAt  time.Time
}

// DualUDPListener listens on port 27900 and demuxes between GameSpy and Quake 2.
type DualUDPListener struct {
	addr       string
	store      *registry.Store
	registry   *games.Registry
	conn       *net.UDPConn
	chalMu     sync.Mutex
	challenges map[string]pendingChallenge
}

// NewDualUDPListener initializes the dual-protocol UDP listener.
func NewDualUDPListener(addr string, store *registry.Store, reg *games.Registry) *DualUDPListener {
	return &DualUDPListener{
		addr:       addr,
		store:      store,
		registry:   reg,
		challenges: make(map[string]pendingChallenge),
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

		// Graceful shutdown (\statechanged\ or \gamemode\exiting) removes immediately
		if isExitingHeartbeat(msg) {
			l.store.RemoveServer(remoteAddr.IP.String(), port, gamename)
			l.clearPendingChallenge(remoteAddr.IP.String(), port)
			return
		}

		// If the game does not support \secure\ (e.g. Kingpin, Deus Ex, Rune), accept and auto-verify directly
		if l.registry != nil && l.registry.SkipsValidation(gamename) {
			l.store.RegisterHeartbeat(remoteAddr.IP.String(), port, gamename)
			return
		}

		// Register as unverified (pending challenge validation)
		srv, _ := l.store.RegisterPendingHeartbeat(remoteAddr.IP.String(), port, gamename)
		if srv == nil {
			return
		}

		// Generate random 6-char alphanumeric challenge per GameSpy spec (Appendix B)
		challenge := gamespy.GenerateChallenge()
		if l.registry != nil {
			if g, ok := l.registry.Lookup(gamename); ok && g.Cipher != "" {
				l.storePendingChallenge(pendingChallenge{
					ip:         remoteAddr.IP.String(),
					queryPort:  srv.QueryPort,
					sourcePort: remoteAddr.Port,
					gamename:   strings.ToLower(gamename),
					cipher:     g.Cipher,
					challenge:  challenge,
					expiresAt:  time.Now().Add(challengeTTL),
				})
			}
		}

		var resp string
		if msg.Has("echo") {
			resp = fmt.Sprintf("\\basic\\\\secure\\%s\\echo_reply\\%s\\", challenge, msg.Get("echo"))
		} else {
			resp = fmt.Sprintf("\\basic\\\\secure\\%s\\", challenge)
		}

		// Send to the source socket
		_, _ = l.conn.WriteToUDP([]byte(resp), remoteAddr)

		// Also send to advertised query port if different from source port
		if port > 0 && port != remoteAddr.Port {
			queryAddr := &net.UDPAddr{IP: remoteAddr.IP, Port: port}
			_, _ = l.conn.WriteToUDP([]byte(resp), queryAddr)
		}
		return
	}

	// Standalone graceful exit packet without heartbeat key
	if isExitingHeartbeat(msg) {
		l.store.RemoveServer(remoteAddr.IP.String(), remoteAddr.Port, msg.Get("gamename"))
		l.clearPendingChallenge(remoteAddr.IP.String(), remoteAddr.Port)
		return
	}

	// Validation response from game server: \validate\<hash>
	if msg.Has("validate") {
		l.handleValidateResponse(remoteAddr, msg.Get("gamename"), msg.Get("validate"))
		return
	}

	// Ping/Echo query directed at master: \echo\<msg>
	if msg.Has("echo") {
		resp := fmt.Sprintf("\\echo_reply\\%s\\", msg.Get("echo"))
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

// isExitingHeartbeat reports whether a GameSpy packet indicates graceful server shutdown.
// Per GameSpy spec: \gamemode\exiting with \statechanged\, or empty \statechanged\ shutdown convention,
// or \statechanged\2 / \statechanged\exiting removes the server immediately.
// Non-exiting statechanged values (e.g. \statechanged\1 or \gamemode\openplaying) are normal refreshes.
func isExitingHeartbeat(msg *gamespy.Message) bool {
	if strings.EqualFold(msg.Get("gamemode"), "exiting") {
		return true
	}
	if !msg.Has("statechanged") {
		return false
	}
	if gm := strings.ToLower(strings.TrimSpace(msg.Get("gamemode"))); gm != "" {
		return gm == "exiting"
	}
	sc := strings.ToLower(strings.TrimSpace(msg.Get("statechanged")))
	if sc == "gamemode" {
		return msg.Has("exiting")
	}
	switch sc {
	case "", "exiting", "2", "0", "shutdown":
		return true
	default:
		return false
	}
}

func (l *DualUDPListener) storePendingChallenge(pc pendingChallenge) {
	key := fmt.Sprintf("%s:%d", pc.ip, pc.queryPort)
	now := time.Now()

	l.chalMu.Lock()
	defer l.chalMu.Unlock()

	for k, existing := range l.challenges {
		if now.After(existing.expiresAt) {
			delete(l.challenges, k)
		}
	}
	l.challenges[key] = pc
}

func (l *DualUDPListener) clearPendingChallenge(ip string, port int) {
	l.chalMu.Lock()
	defer l.chalMu.Unlock()

	for k, pc := range l.challenges {
		if pc.ip == ip && (pc.queryPort == port || pc.sourcePort == port) {
			delete(l.challenges, k)
		}
	}
}

func (l *DualUDPListener) handleValidateResponse(remoteAddr *net.UDPAddr, pktGamename, token string) {
	if token == "" {
		return
	}
	ip := remoteAddr.IP.String()
	now := time.Now()

	l.chalMu.Lock()
	// Prune expired challenges and collect candidates for this IP
	var exactKeys []string
	var fallbackKeys []string
	for k, pc := range l.challenges {
		if now.After(pc.expiresAt) {
			delete(l.challenges, k)
			continue
		}
		if pc.ip != ip {
			continue
		}
		if pc.queryPort == remoteAddr.Port || pc.sourcePort == remoteAddr.Port {
			exactKeys = append(exactKeys, k)
		} else if pktGamename == "" || strings.EqualFold(pc.gamename, pktGamename) {
			fallbackKeys = append(fallbackKeys, k)
		}
	}

	candidates := append(exactKeys, fallbackKeys...)
	for _, k := range candidates {
		pc := l.challenges[k]
		if l.matchesCipher(pc.gamename, pktGamename, pc.cipher, pc.challenge, token) {
			delete(l.challenges, k)
			l.chalMu.Unlock()
			l.store.VerifyServer(pc.ip, pc.queryPort)
			return
		}
	}
	l.chalMu.Unlock()
}

func (l *DualUDPListener) matchesCipher(hbGamename, pktGamename, primaryCipher, challenge, token string) bool {
	if gamespy.VerifyChallenge(primaryCipher, challenge, token) {
		return true
	}
	if l.registry == nil {
		return false
	}
	if pktGamename != "" {
		if g, ok := l.registry.Lookup(pktGamename); ok && g.Cipher != "" && g.Cipher != primaryCipher {
			if gamespy.VerifyChallenge(g.Cipher, challenge, token) {
				return true
			}
		}
	}
	// Handle UT / UT99 catalog alias where UT99 servers report gamename "ut" with cipher Z5Nfb0
	if strings.EqualFold(hbGamename, "ut") || strings.EqualFold(pktGamename, "ut") {
		if g, ok := l.registry.Lookup("ut99"); ok && gamespy.VerifyChallenge(g.Cipher, challenge, token) {
			return true
		}
	}
	if strings.EqualFold(hbGamename, "ut99") || strings.EqualFold(pktGamename, "ut99") {
		if g, ok := l.registry.Lookup("ut"); ok && gamespy.VerifyChallenge(g.Cipher, challenge, token) {
			return true
		}
	}
	return false
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
