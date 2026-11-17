package listener

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"master-server/internal/registry"
)

// TCPUT2004Listener serves Unreal Tournament 2004 (and UT2003) master server requests on port 28902.
type TCPUT2004Listener struct {
	addr     string
	store    *registry.Store
	listener net.Listener
}

// NewTCPUT2004Listener creates a new UT2004 master listener on port 28902.
func NewTCPUT2004Listener(addr string, store *registry.Store) *TCPUT2004Listener {
	return &TCPUT2004Listener{
		addr:  addr,
		store: store,
	}
}

// Start begins accepting UT2004 client and server connections.
func (l *TCPUT2004Listener) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", l.addr)
	if err != nil {
		return fmt.Errorf("listen tcp %s: %w", l.addr, err)
	}
	l.listener = ln
	log.Printf("[TCP 28902] Unreal Tournament 2004 Master listener active on %s", l.addr)

	go func() {
		<-ctx.Done()
		l.listener.Close()
	}()

	for {
		conn, err := l.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				continue
			}
		}

		go l.handleConnection(conn)
	}
}

func (l *TCPUT2004Listener) handleConnection(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	remoteAddr := conn.RemoteAddr().String()

	// 1. Send initial challenge: [length uint32 LE] [0x0a] "111111111\0"
	challenge := append([]byte{0x0a}, []byte("111111111")...)
	challenge = append(challenge, 0x00)
	if err := l.writePacket(conn, challenge); err != nil {
		return
	}

	// 2. Read client/server auth response
	if _, err := l.readPacket(conn); err != nil {
		return
	}

	// 3. Send APPROVED response: [0x09] "APPROVED\0" [0x03 0x00 0x00 0x00]
	approved := []byte{0x09, 'A', 'P', 'P', 'R', 'O', 'V', 'E', 'D', 0x00, 0x03, 0x00, 0x00, 0x00}
	if err := l.writePacket(conn, approved); err != nil {
		return
	}

	// 4. Read handshake packet to determine if connection is a game client or server uplink
	nextData, err := l.readPacket(conn)
	if err != nil {
		return
	}

	// ─── CASE A: Game Client (browser query) ───
	if len(nextData) > 0 && nextData[0] == '!' {
		// Send VERIFIED confirmation: [0x09] "VERIFIED\0"
		if err := l.writePacket(conn, []byte{0x09, 'V', 'E', 'R', 'I', 'F', 'I', 'E', 'D', 0x00}); err != nil {
			return
		}

		// Read client request type
		reqData, err := l.readPacket(conn)
		if err != nil {
			return
		}

		// Request type 0x01: MOTD / News tab request
		if len(reqData) == 1 && reqData[0] == 0x01 {
			motd := "\x40\x11\x1b\xff\xff\xffWelcome to OpenMaster!\r\n\r\n" +
				"\x1b\xff\xa5\x01*\x1b\xff\xff\xff Universal Retro Master Server.\r\n" +
				"\x1b\xff\xa5\x01*\x1b\xff\xff\xff Serving active UT2004 community servers.\r\n" +
				"\x00\x00\x00\x00\x00"
			_ = l.writePacket(conn, []byte(motd))
			return
		}

		// Otherwise: Server list request
		servers := l.store.GetServersByGame("ut2004")
		if len(servers) == 0 {
			// Fallback to ut2 or all servers matching Unreal 2003/2004
			servers = l.store.GetServersByGame("ut2")
		}

		resp := l.buildServerListResponse(servers)
		_, _ = conn.Write(resp)
		log.Printf("[TCP 28902] %s <- Served %d UT2004 servers", remoteAddr, len(servers))
		return
	}

	// ─── CASE B: Game Server Uplink (dedicated server registering) ───
	host, _, _ := net.SplitHostPort(remoteAddr)
	if !l.store.IsBlacklisted(host) {
		l.store.RegisterHeartbeat(host, 7787, "ut2004")
	}

	// Send state 2 response: [0x01] continue flag + 4 dwords
	state2Resp := []byte{
		0x01,
		0x01, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
	}
	if _, err := conn.Write(state2Resp); err != nil {
		return
	}

	// Keep connection open for server refresh packets
	_ = conn.SetDeadline(time.Now().Add(120 * time.Second))
	for {
		data, err := l.readPacket(conn)
		if err != nil {
			break
		}
		if len(data) == 0 {
			continue
		}
		// 0x01 = Server Info Refresh packet
		if data[0] == 0x01 {
			info := parseUT2004ServerInfo(data)
			if info != nil && info.port > 0 {
				queryPort := info.port + 10 // UT2004 query port standard is GamePort + 10
				srv, _ := l.store.RegisterSeededServer(host, queryPort, info.port, "ut2004")
				if srv != nil {
					l.store.UpdateServerInfo(host, queryPort, info.port, info.name, "", 0, 0, 0, nil, map[string]string{"gametype": info.gametype})
				}
				log.Printf("[TCP 28902] %s UT2004 server registered: name=%q port=%d gametype=%q", remoteAddr, info.name, info.port, info.gametype)
			}
		}
	}
}

// ─── Protocol Packet Helpers ────────────────────────────────────────────────

func (l *TCPUT2004Listener) readPacket(r io.Reader) ([]byte, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}
	plen := binary.LittleEndian.Uint32(lenBuf[:])
	if plen > 1024*1024 {
		return nil, fmt.Errorf("packet too large: %d", plen)
	}
	data := make([]byte, plen)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (l *TCPUT2004Listener) writePacket(w io.Writer, data []byte) error {
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(data)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}

func writeUTPacketBuf(data []byte) []byte {
	result := make([]byte, 4)
	binary.LittleEndian.PutUint32(result, uint32(len(data)))
	return append(result, data...)
}

func (l *TCPUT2004Listener) buildServerEntry(srv registry.Server) []byte {
	gamePort := srv.GamePort
	if gamePort <= 0 {
		gamePort = srv.QueryPort
	}
	queryPort := srv.QueryPort
	if queryPort <= 0 {
		queryPort = gamePort + 10
	}

	entry := make([]byte, 0, 128)
	ip := net.ParseIP(srv.IP).To4()
	if ip == nil {
		ip = net.IPv4zero.To4()
	}
	entry = append(entry, ip...)
	entry = append(entry, byte(gamePort), byte(gamePort>>8))
	entry = append(entry, byte(queryPort), byte(queryPort>>8))

	name := srv.Hostname
	if name == "" {
		name = "UT2004 Server"
	}
	entry = append(entry, []byte(name)...)
	entry = append(entry, 0x00)
	entry = append(entry, 0x01, 0x00)

	gt := srv.Gametype
	if gt == "" {
		gt = "xDeathMatch"
	}
	entry = append(entry, byte(len(gt)+1))
	entry = append(entry, []byte(gt)...)
	entry = append(entry, 0x00)
	entry = append(entry, 0x00, 0x00)
	entry = append(entry, 0x20, 0x00, 0x00, 0x00)
	entry = append(entry, 0x01, 0x00)

	result := make([]byte, 4)
	binary.LittleEndian.PutUint32(result, uint32(len(entry)))
	return append(result, entry...)
}

func (l *TCPUT2004Listener) buildServerListResponse(servers []registry.Server) []byte {
	var buf []byte
	count := uint16(len(servers))
	header := []byte{byte(count), byte(count >> 8), 0x00, 0x00, 0x01}
	buf = append(buf, writeUTPacketBuf(header)...)

	for _, s := range servers {
		buf = append(buf, l.buildServerEntry(s)...)
	}
	return buf
}

type ut2004Info struct {
	name     string
	gametype string
	port     int
}

func parseUT2004ServerInfo(data []byte) *ut2004Info {
	info := &ut2004Info{port: 7777}
	if len(data) < 14 {
		return info
	}

	// Port at offset 8 (uint16 LE)
	info.port = int(binary.LittleEndian.Uint16(data[8:10]))

	// Parse strings starting at offset 14
	pos := 14
	for i := 0; i < 3 && pos < len(data); i++ {
		strLen := int(data[pos])
		pos++
		if strLen == 0 || pos+strLen > len(data) {
			break
		}
		str := string(data[pos : pos+strLen-1])
		pos += strLen

		switch i {
		case 0:
			info.name = str
		case 2:
			info.gametype = str
		}
	}
	return info
}
