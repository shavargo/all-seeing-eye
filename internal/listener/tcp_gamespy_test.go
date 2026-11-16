package listener

import (
	"net"
	"strings"
	"testing"
	"time"

	"master-server/internal/games"
	"master-server/internal/registry"
)

func TestTCPGameSpyListener_MultiStepHandshake(t *testing.T) {
	store := registry.NewStore(true)
	store.RegisterHeartbeat("1.2.3.4", 7777, "ut")

	reg, err := games.NewRegistry()
	if err != nil {
		t.Fatalf("failed to init games registry: %v", err)
	}

	listener := NewTCPGameSpyListener("127.0.0.1:0", store, reg)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on random port: %v", err)
	}
	listener.listener = ln
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go listener.handleConnection(conn)
		}
	}()

	addr := ln.Addr().String()

	// 1. Test multi-packet handshake:
	// Client receives \basic\\secure\TXKOAT\
	// Client sends \gamename\ut\location\0\validate\Sen5c0WR\final\
	// Client sends \list\cmp\gamename\ut\final\
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read initial challenge: %v", err)
	}
	challenge := string(buf[:n])
	if !strings.Contains(challenge, "\\basic\\\\secure\\") {
		t.Errorf("expected \\basic\\\\secure\\ in initial greeting, got %q", challenge)
	}

	// Send Packet 1: validate token
	_, err = conn.Write([]byte("\\gamename\\ut\\location\\0\\validate\\Sen5c0WR\\final\\"))
	if err != nil {
		t.Fatalf("failed to write validate packet: %v", err)
	}

	// Small pause simulating network latency between client commands
	time.Sleep(50 * time.Millisecond)

	// Send Packet 2: list request
	_, err = conn.Write([]byte("\\list\\cmp\\gamename\\ut\\final\\"))
	if err != nil {
		t.Fatalf("failed to write list packet: %v", err)
	}

	listBuf := make([]byte, 2048)
	n, err = conn.Read(listBuf)
	if err != nil {
		t.Fatalf("failed to read server list: %v", err)
	}
	listResp := string(listBuf[:n])
	if !strings.Contains(listResp, "\\ip\\1.2.3.4:7777") || !strings.Contains(listResp, "\\final\\") {
		t.Errorf("expected server list with 1.2.3.4:7777 and \\final\\, got %q", listResp)
	}
}

func TestTCPGameSpyListener_DirectListQuery(t *testing.T) {
	store := registry.NewStore(true)
	store.RegisterHeartbeat("5.6.7.8", 28070, "jk2")

	reg, err := games.NewRegistry()
	if err != nil {
		t.Fatalf("failed to init games registry: %v", err)
	}

	listener := NewTCPGameSpyListener("127.0.0.1:0", store, reg)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on random port: %v", err)
	}
	listener.listener = ln
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go listener.handleConnection(conn)
		}
	}()

	conn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer conn.Close()

	buf := make([]byte, 1024)
	_, _ = conn.Read(buf) // read greeting

	// Send direct list query without validate
	_, err = conn.Write([]byte("\\list\\cmp\\gamename\\jk2\\final\\"))
	if err != nil {
		t.Fatalf("failed to send direct list: %v", err)
	}

	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}
	resp := string(buf[:n])
	if !strings.Contains(resp, "\\ip\\5.6.7.8:28070") {
		t.Errorf("expected jk2 server list, got %q", resp)
	}
}
