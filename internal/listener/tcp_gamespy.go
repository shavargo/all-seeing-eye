package listener

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"master-server/internal/games"
	"master-server/internal/gamespy"
	"master-server/internal/registry"
)

// TCPGameSpyListener serves GameSpy v0/v1 server lists on port 28900 and 28910.
type TCPGameSpyListener struct {
	addr     string
	store    *registry.Store
	registry *games.Registry
	listener net.Listener
}

// NewTCPGameSpyListener creates the TCP master listener.
func NewTCPGameSpyListener(addr string, store *registry.Store, reg *games.Registry) *TCPGameSpyListener {
	return &TCPGameSpyListener{
		addr:     addr,
		store:    store,
		registry: reg,
	}
}

// Start begins accepting TCP connections.
func (l *TCPGameSpyListener) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", l.addr)
	if err != nil {
		return fmt.Errorf("listen tcp %s: %w", l.addr, err)
	}
	l.listener = ln
	log.Printf("[TCP GameSpy] List & Sync listener active on %s", l.addr)

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

func (l *TCPGameSpyListener) handleConnection(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	// 1. Send random 6-char GameSpy challenge to client
	challenge := gamespy.GenerateChallenge()
	initialGreeting := fmt.Sprintf("\\basic\\\\secure\\%s\\", challenge)
	if _, err := conn.Write([]byte(initialGreeting)); err != nil {
		return
	}

	// 2. Read client requests in a loop (handles multi-packet handshake: \validate\ followed by \list\)
	buf := make([]byte, 4096)
	var connGamename string
	var pendingValidateToken string

	for {
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		n, err := conn.Read(buf)
		if err != nil && err != io.EOF {
			return
		}
		if n <= 0 {
			return
		}

		raw := string(buf[:n])
		msg := gamespy.ParseMessage(raw)

		if gn := msg.Get("gamename"); gn != "" && !strings.EqualFold(gn, "gspylite") {
			connGamename = gn
		}
		if msg.Has("validate") {
			pendingValidateToken = msg.Get("validate")
		}

		// If client sent \validate\ and gamename is known, verify with the game's cipher
		// (unless the game is in SkipsValidation)
		if pendingValidateToken != "" && connGamename != "" {
			if !l.verifyClientChallenge(connGamename, challenge, pendingValidateToken) {
				return
			}
			pendingValidateToken = ""
		}

		// Handle GameSpy \list\ request
		if msg.Has("list") || strings.Contains(raw, "\\list\\") {
			l.handleListRequest(conn, msg, connGamename)
			return
		}

		// If client sent \validate\ or other initial handshake without \list yet,
		// stay open and wait for the subsequent \list\ command
		if msg.Has("validate") {
			continue
		}
	}
}

func (l *TCPGameSpyListener) verifyClientChallenge(gamename, challenge, token string) bool {
	if l.registry == nil || l.registry.SkipsValidation(gamename) {
		return true
	}
	g, ok := l.registry.Lookup(gamename)
	if !ok || g.Cipher == "" {
		return true
	}
	if gamespy.VerifyChallenge(g.Cipher, challenge, token) {
		return true
	}
	// Handle UT / UT99 catalog alias where UT99 clients report gamename "ut" with cipher Z5Nfb0
	if strings.EqualFold(gamename, "ut") {
		if ut99, ok := l.registry.Lookup("ut99"); ok && gamespy.VerifyChallenge(ut99.Cipher, challenge, token) {
			return true
		}
	}
	if strings.EqualFold(gamename, "ut99") {
		if ut, ok := l.registry.Lookup("ut"); ok && gamespy.VerifyChallenge(ut.Cipher, challenge, token) {
			return true
		}
	}
	return false
}

// handleListRequest writes out the GameSpy server list.
func (l *TCPGameSpyListener) handleListRequest(conn net.Conn, msg *gamespy.Message, fallbackGamename string) {
	gamename := msg.Get("gamename")
	if gamename == "" || strings.EqualFold(gamename, "gspylite") {
		if fallbackGamename != "" {
			gamename = fallbackGamename
		} else if msg.Has("all") {
			gamename = "all"
		} else {
			gamename = "kingpin"
		}
	}

	servers := l.store.GetServersByGame(gamename)

	var sb strings.Builder
	for _, srv := range servers {
		sb.WriteString(fmt.Sprintf("\\ip\\%s:%d", srv.IP, srv.GamePort))
	}
	sb.WriteString("\\final\\")

	_, _ = conn.Write([]byte(sb.String()))
}
