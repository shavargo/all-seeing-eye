package listener

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"master-server/internal/registry"
)

func TestKPQ3Listener_MultiPortQueries(t *testing.T) {
	store := registry.NewStore(true)

	// Seed servers for Quake 3 and Call of Duty
	q3Srv, _ := store.RegisterHeartbeat("1.2.3.4", 27960, "quake3")
	q3Srv.Verified = true

	codSrv, _ := store.RegisterHeartbeat("5.6.7.8", 28960, "callofduty")
	codSrv.Verified = true

	// Custom listener on high ports for testing
	listener := &KPQ3Listener{
		primaryAddr: "127.0.0.1:0",
		store:       store,
		ports: []IDTech3PortConfig{
			{Port: 0, DefaultGame: "quake3", Label: "Test Q3"},
			{Port: 0, DefaultGame: "callofduty", Label: "Test CoD"},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Manually bind 2 dynamic UDP sockets to simulate the ports
	q3Conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("failed to bind q3 udp: %v", err)
	}
	defer q3Conn.Close()

	codConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("failed to bind cod udp: %v", err)
	}
	defer codConn.Close()

	go listener.listenOnConn(ctx, q3Conn, IDTech3PortConfig{Port: q3Conn.LocalAddr().(*net.UDPAddr).Port, DefaultGame: "quake3", Label: "Test Q3"})
	go listener.listenOnConn(ctx, codConn, IDTech3PortConfig{Port: codConn.LocalAddr().(*net.UDPAddr).Port, DefaultGame: "callofduty", Label: "Test CoD"})

	// Query Q3 listener: \xff\xff\xff\xffgetservers 68 empty full
	clientConn, err := net.DialUDP("udp", nil, q3Conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("dial udp failed: %v", err)
	}
	defer clientConn.Close()

	req := []byte("\xff\xff\xff\xffgetservers 68 empty full\x00")
	if _, err := clientConn.Write(req); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	resp := make([]byte, 2048)
	n, err := clientConn.Read(resp)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	rawResp := string(resp[:n])
	if len(rawResp) < 22 || rawResp[:22] != "\xff\xff\xff\xffgetserversResponse" {
		t.Fatalf("invalid Q3 response header: %q", rawResp)
	}

	// Verify server IP 1.2.3.4 was included in response
	if !containsIP(resp[:n], net.ParseIP("1.2.3.4").To4()) {
		t.Fatalf("expected IP 1.2.3.4 in Q3 response")
	}

	// Test null-terminated heartbeat & flatline shutdown
	hbConn, err := net.DialUDP("udp", nil, q3Conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("dial hbConn failed: %v", err)
	}
	defer hbConn.Close()

	_, _ = hbConn.Write([]byte("\xff\xff\xff\xffheartbeat QuakeArena-1\n\x00"))
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if len(store.GetServersByGame("quake3")) == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(store.GetServersByGame("quake3")) != 2 {
		t.Fatalf("expected null-terminated heartbeat QuakeArena-1 to register as quake3, got %d", len(store.GetServersByGame("quake3")))
	}

	// Send flatline shutdown from hbConn
	_, _ = hbConn.Write([]byte("\xff\xff\xff\xffheartbeat flatline\n\x00"))
	deadline = time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if len(store.GetServersByGame("quake3")) == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(store.GetServersByGame("quake3")) != 1 {
		t.Fatalf("expected flatline heartbeat to remove server, got %d", len(store.GetServersByGame("quake3")))
	}
}

func TestKPQ3Listener_ChunkedMTU(t *testing.T) {
	store := registry.NewStore(true)
	// Seed 250 Quake 3 servers (> 2 chunks of 112)
	for i := 1; i <= 250; i++ {
		ip := fmt.Sprintf("10.1.%d.%d", (i>>8)&0xff, i&0xff)
		store.RegisterHeartbeat(ip, 27960+i, "quake3")
	}

	listener := NewKPQ3Listener("127.0.0.1:0", store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q3Conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Fatalf("failed to bind q3 udp: %v", err)
	}
	defer q3Conn.Close()

	go listener.listenOnConn(ctx, q3Conn, IDTech3PortConfig{Port: q3Conn.LocalAddr().(*net.UDPAddr).Port, DefaultGame: "quake3", Label: "Test Q3"})

	clientConn, err := net.DialUDP("udp", nil, q3Conn.LocalAddr().(*net.UDPAddr))
	if err != nil {
		t.Fatalf("dial udp failed: %v", err)
	}
	defer clientConn.Close()

	_, _ = clientConn.Write([]byte("\xff\xff\xff\xffgetservers 68 empty full"))
	_ = clientConn.SetReadDeadline(time.Now().Add(1 * time.Second))

	packets := 0
	for {
		buf := make([]byte, 2048)
		n, err := clientConn.Read(buf)
		if err != nil {
			break
		}
		if n > 1400 {
			t.Fatalf("packet %d exceeded 1400-byte MTU limit: %d bytes", packets+1, n)
		}
		packets++
		if packets == 3 {
			break
		}
	}
	if packets != 3 {
		t.Fatalf("expected 3 chunked UDP packets for 250 servers, got %d", packets)
	}
}

func containsIP(data []byte, ip net.IP) bool {
	for i := 0; i <= len(data)-4; i++ {
		if data[i] == ip[0] && data[i+1] == ip[1] && data[i+2] == ip[2] && data[i+3] == ip[3] {
			return true
		}
	}
	return false
}
