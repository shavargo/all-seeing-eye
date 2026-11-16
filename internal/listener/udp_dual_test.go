package listener

import (
	"net"
	"strings"
	"testing"
	"time"

	"master-server/internal/games"
	"master-server/internal/registry"
)

func TestDualUDPListener_HeartbeatChallenge(t *testing.T) {
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
	defer conn.Close()

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

	clientConn, err := net.DialUDP("udp", nil, conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("client dial error: %v", err)
	}
	defer clientConn.Close()

	// Send heartbeat with echo tag (like Dark's test)
	heartbeat := "\\heartbeat\\27900\\gamename\\333networks\\echo\\test"
	_, err = clientConn.Write([]byte(heartbeat))
	if err != nil {
		t.Fatalf("write heartbeat error: %v", err)
	}

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	respBuf := make([]byte, 1024)
	n, err := clientConn.Read(respBuf)
	if err != nil {
		t.Fatalf("read challenge error: %v", err)
	}

	resp := string(respBuf[:n])
	// Should contain \basic\\secure\ challenge and echo_reply
	if !strings.Contains(resp, "\\basic\\\\secure\\") {
		t.Errorf("expected \\basic\\\\secure\\ in response, got %q", resp)
	}
	if !strings.Contains(resp, "\\echo_reply\\test\\") {
		t.Errorf("expected \\echo_reply\\test\\ in response, got %q", resp)
	}

	// Verify server was recorded in store
	srvs := store.GetServersByGame("333networks")
	if len(srvs) == 0 {
		t.Errorf("expected 333networks server to be registered in store")
	}
}
