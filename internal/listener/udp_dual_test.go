package listener

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"master-server/internal/games"
	"master-server/internal/gamespy"
	"master-server/internal/registry"
)

func startTestUDPListener(t *testing.T) (*DualUDPListener, *registry.Store, *games.Registry, *net.UDPAddr, func()) {
	t.Helper()
	store := registry.NewStore(true)
	reg, err := games.NewRegistry()
	if err != nil {
		t.Fatalf("failed to init games registry: %v", err)
	}

	listener := NewDualUDPListener("127.0.0.1:0", store, reg)

	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		t.Fatalf("listen error: %v", err)
	}
	listener.conn = conn

	go func() {
		buf := make([]byte, 4096)
		for {
			n, rAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				return
			}
			packet := make([]byte, n)
			copy(packet, buf[:n])
			go listener.handlePacket(packet, rAddr)
		}
	}()

	return listener, store, reg, conn.LocalAddr().(*net.UDPAddr), func() {
		_ = conn.Close()
	}
}

func TestDualUDPListener_HeartbeatChallengeAndValidate(t *testing.T) {
	_, store, reg, serverAddr, cleanup := startTestUDPListener(t)
	defer cleanup()

	clientConn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		t.Fatalf("client dial error: %v", err)
	}
	defer clientConn.Close()

	// 1. Send heartbeat for cipher-validated game (333networks)
	heartbeat := "\\heartbeat\\27900\\gamename\\333networks\\echo\\test"
	if _, err := clientConn.Write([]byte(heartbeat)); err != nil {
		t.Fatalf("write heartbeat error: %v", err)
	}

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	respBuf := make([]byte, 1024)
	n, err := clientConn.Read(respBuf)
	if err != nil {
		t.Fatalf("read challenge error: %v", err)
	}

	resp := string(respBuf[:n])
	if !strings.Contains(resp, "\\basic\\\\secure\\") {
		t.Fatalf("expected \\basic\\\\secure\\ in response, got %q", resp)
	}
	if !strings.Contains(resp, "\\echo_reply\\test\\") {
		t.Fatalf("expected \\echo_reply\\test\\ in response, got %q", resp)
	}

	parsed := gamespy.ParseMessage(resp)
	challenge := parsed.Get("secure")
	if len(challenge) != 6 {
		t.Fatalf("expected 6-char random challenge, got %q", challenge)
	}

	// 2. Pending heartbeat must NOT be returned by GetServersByGame or GetSnapshot yet
	if srvs := store.GetServersByGame("333networks"); len(srvs) != 0 {
		t.Fatalf("expected pending server to be excluded from GetServersByGame before validate, got %d", len(srvs))
	}
	if snap := store.GetSnapshot(); snap.TotalServers != 0 {
		t.Fatalf("expected pending server to be excluded from GetSnapshot before validate, got %d", snap.TotalServers)
	}

	// 3. Send WRONG \validate\ token — must stay unverified
	wrongPkt := "\\gamename\\333networks\\validate\\BADTOKEN\\final\\"
	if _, err := clientConn.Write([]byte(wrongPkt)); err != nil {
		t.Fatalf("write wrong validate error: %v", err)
	}
	time.Sleep(30 * time.Millisecond)

	if srvs := store.GetServersByGame("333networks"); len(srvs) != 0 {
		t.Fatalf("expected server to remain unverified after wrong validate, got %d", len(srvs))
	}
	if snap := store.GetSnapshot(); snap.TotalServers != 0 {
		t.Fatalf("expected snapshot to remain empty after wrong validate, got %d", snap.TotalServers)
	}

	// 4. Send valid \validate\ token computed with catalog cipher (enctype 0)
	game, ok := reg.Lookup("333networks")
	if !ok || game.Cipher == "" {
		t.Fatalf("expected 333networks cipher in catalog")
	}
	validToken := gamespy.GenerateValidate(game.Cipher, challenge, 0)
	validPkt := fmt.Sprintf("\\gamename\\333networks\\validate\\%s\\final\\", validToken)
	if _, err := clientConn.Write([]byte(validPkt)); err != nil {
		t.Fatalf("write valid validate error: %v", err)
	}

	// Wait briefly for async UDP handler
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if len(store.GetServersByGame("333networks")) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	srvs := store.GetServersByGame("333networks")
	if len(srvs) != 1 {
		t.Fatalf("expected 1 verified 333networks server after valid validate, got %d", len(srvs))
	}
	snap := store.GetSnapshot()
	if snap.TotalServers != 1 {
		t.Fatalf("expected GetSnapshot to include verified server immediately, got %d", snap.TotalServers)
	}
}

func TestDualUDPListener_SkipsValidationImmediate(t *testing.T) {
	_, store, _, serverAddr, cleanup := startTestUDPListener(t)
	defer cleanup()

	clientConn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		t.Fatalf("client dial error: %v", err)
	}
	defer clientConn.Close()

	// Kingpin is in SkipsValidation — should appear immediately without \validate\
	if _, err := clientConn.Write([]byte("\\heartbeat\\31510\\gamename\\kingpin")); err != nil {
		t.Fatalf("write heartbeat error: %v", err)
	}

	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if len(store.GetServersByGame("kingpin")) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if srvs := store.GetServersByGame("kingpin"); len(srvs) != 1 {
		t.Fatalf("expected SkipsValidation game kingpin to appear immediately, got %d", len(srvs))
	}
	if snap := store.GetSnapshot(); snap.TotalServers != 1 {
		t.Fatalf("expected GetSnapshot to include SkipsValidation server immediately, got %d", snap.TotalServers)
	}
}

func TestDualUDPListener_StateChangedExiting(t *testing.T) {
	_, store, _, serverAddr, cleanup := startTestUDPListener(t)
	defer cleanup()

	clientConn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		t.Fatalf("client dial error: %v", err)
	}
	defer clientConn.Close()

	// 1. Register a server
	_, _ = clientConn.Write([]byte("\\heartbeat\\31510\\gamename\\kingpin"))
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if len(store.GetServersByGame("kingpin")) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if store.GetSnapshot().TotalServers != 1 {
		t.Fatalf("expected 1 server registered")
	}

	// 2. Send statechanged WITHOUT exiting (e.g. \statechanged\1) -> treated as normal refresh
	_, _ = clientConn.Write([]byte("\\heartbeat\\31510\\gamename\\kingpin\\statechanged\\1"))
	time.Sleep(30 * time.Millisecond)
	if len(store.GetServersByGame("kingpin")) != 1 {
		t.Fatalf("expected non-exiting statechanged to keep server active")
	}

	// 3. Send statechanged with gamemode=exiting -> removes immediately and updates snapshot
	_, _ = clientConn.Write([]byte("\\heartbeat\\31510\\gamename\\kingpin\\gamemode\\exiting\\statechanged\\"))
	deadline = time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if len(store.GetServersByGame("kingpin")) == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(store.GetServersByGame("kingpin")) != 0 {
		t.Fatalf("expected server to be removed immediately on gamemode=exiting")
	}
	if store.GetSnapshot().TotalServers != 0 {
		t.Fatalf("expected snapshot to be updated on exiting removal, got %d", store.GetSnapshot().TotalServers)
	}

	// 4. Re-register and test empty \statechanged\ shutdown convention
	_, _ = clientConn.Write([]byte("\\heartbeat\\31510\\gamename\\kingpin"))
	deadline = time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if len(store.GetServersByGame("kingpin")) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, _ = clientConn.Write([]byte("\\heartbeat\\31510\\gamename\\kingpin\\statechanged\\"))
	deadline = time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if len(store.GetServersByGame("kingpin")) == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if store.GetSnapshot().TotalServers != 0 {
		t.Fatalf("expected empty statechanged shutdown to remove server and rebuild snapshot")
	}
}
