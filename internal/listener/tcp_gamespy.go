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

// TCPGameSpyListener serves GameSpy v0 server lists and 333Networks sync requests on port 28900.
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
	log.Printf("[TCP 28900] GameSpy List & Sync listener active on %s", l.addr)

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

	// 1. Send GameSpy challenge to client
	challenge := "TXKOAT"
	initialGreeting := fmt.Sprintf("\\basic\\\\secure\\%s", challenge)
	if _, err := conn.Write([]byte(initialGreeting)); err != nil {
		return
	}

	// 2. Read client request
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		return
	}
	if n <= 0 {
		return
	}

	raw := string(buf[:n])
	msg := gamespy.ParseMessage(raw)

	// 3a. Handle 333Networks \sync\ request
	if msg.Has("sync") {
		l.handleSyncRequest(conn, msg)
		return
	}

	// 3b. Handle GameSpy \list\ request
	if msg.Has("list") || strings.Contains(raw, "\\list\\") {
		l.handleListRequest(conn, msg)
		return
	}
}

// handleListRequest writes out the GameSpy server list.
func (l *TCPGameSpyListener) handleListRequest(conn net.Conn, msg *gamespy.Message) {
	gamename := msg.Get("gamename")
	if gamename == "" || strings.EqualFold(gamename, "gspylite") {
		if msg.Has("all") {
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

// handleSyncRequest formats lists for 333Networks federation:
// \gamename1\ip:port ip:port\gamename2\ip:port...\final\
func (l *TCPGameSpyListener) handleSyncRequest(conn net.Conn, msg *gamespy.Message) {
	requestedGame := msg.Get("sync")
	snap := l.store.GetSnapshot()

	// Group servers by gamename
	byGame := make(map[string][]string)
	for _, srv := range snap.Servers {
		gn := srv.Gamename
		if requestedGame != "all" && requestedGame != "" && !strings.Contains(requestedGame, gn) {
			continue
		}
		byGame[gn] = append(byGame[gn], fmt.Sprintf("%s:%d", srv.IP, srv.GamePort))
	}

	var sb strings.Builder
	for gn, addrs := range byGame {
		sb.WriteString(fmt.Sprintf("\\%s\\%s ", gn, strings.Join(addrs, " ")))
	}
	sb.WriteString("\\final\\")

	_, _ = conn.Write([]byte(sb.String()))
}
