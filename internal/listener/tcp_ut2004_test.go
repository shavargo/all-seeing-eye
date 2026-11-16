package listener

import (
	"encoding/binary"
	"io"
	"net"
	"testing"

	"master-server/internal/registry"
)

func TestTCPUT2004Listener_ClientQuery(t *testing.T) {
	store := registry.NewStore(true)
	// Seed a test UT2004 server
	srv, _ := store.RegisterHeartbeat("192.168.1.50", 7787, "ut2004")
	if srv == nil {
		t.Fatalf("RegisterHeartbeat failed")
	}
	srv.GamePort = 7777
	srv.Hostname = "FragTown UT2004"
	srv.Gametype = "xDeathMatch"
	srv.Verified = true

	l := NewTCPUT2004Listener("127.0.0.1:0", store)

	// Find free port and bind
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	l.listener = ln
	l.addr = ln.Addr().String()

	go func() {
		for {
			conn, err := l.listener.Accept()
			if err != nil {
				return
			}
			go l.handleConnection(conn)
		}
	}()

	conn, err := net.Dial("tcp", l.addr)
	if err != nil {
		t.Fatalf("failed to dial listener: %v", err)
	}
	defer conn.Close()

	// 1. Read challenge packet
	chalData, err := readTestPacket(conn)
	if err != nil {
		t.Fatalf("failed to read challenge: %v", err)
	}
	if len(chalData) == 0 || chalData[0] != 0x0a {
		t.Fatalf("expected 0x0a header, got %x", chalData)
	}

	// 2. Send client response (dummy cd key auth)
	if err := writeTestPacket(conn, []byte{0x00, 0x01, 0x02}); err != nil {
		t.Fatalf("failed to write auth: %v", err)
	}

	// 3. Read APPROVED response
	appData, err := readTestPacket(conn)
	if err != nil {
		t.Fatalf("failed to read approved: %v", err)
	}
	if string(appData[:9]) != "\x09APPROVED" {
		t.Fatalf("expected APPROVED, got %q", string(appData))
	}

	// 4. Send client verification '!'
	if err := writeTestPacket(conn, []byte("!")); err != nil {
		t.Fatalf("failed to write '!': %v", err)
	}

	// 5. Read VERIFIED response
	verData, err := readTestPacket(conn)
	if err != nil {
		t.Fatalf("failed to read verified: %v", err)
	}
	if string(verData[:9]) != "\x09VERIFIED" {
		t.Fatalf("expected VERIFIED, got %q", string(verData))
	}

	// 6. Send list query (request type != 0x01)
	if err := writeTestPacket(conn, []byte{0x00}); err != nil {
		t.Fatalf("failed to write list query: %v", err)
	}

	// 7. Read server list response
	listData, err := readTestPacket(conn)
	if err != nil {
		t.Fatalf("failed to read server list header: %v", err)
	}
	if len(listData) < 2 {
		t.Fatalf("list data too short: %x", listData)
	}
	count := binary.LittleEndian.Uint16(listData[:2])
	if count != 1 {
		t.Fatalf("expected 1 server, got %d", count)
	}

	// Read individual server entry
	entryData, err := readTestPacket(conn)
	if err != nil {
		t.Fatalf("failed to read server entry: %v", err)
	}
	// Check IP (192.168.1.50)
	if entryData[0] != 192 || entryData[1] != 168 || entryData[2] != 1 || entryData[3] != 50 {
		t.Fatalf("expected IP 192.168.1.50, got %d.%d.%d.%d", entryData[0], entryData[1], entryData[2], entryData[3])
	}
	gamePort := binary.LittleEndian.Uint16(entryData[4:6])
	if gamePort != 7777 {
		t.Fatalf("expected gamePort 7777, got %d", gamePort)
	}
}

func readTestPacket(r io.Reader) ([]byte, error) {
	var lenBuf [4]byte
	if _, err := io.ReadFull(r, lenBuf[:]); err != nil {
		return nil, err
	}
	plen := binary.LittleEndian.Uint32(lenBuf[:])
	data := make([]byte, plen)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}
	return data, nil
}

func writeTestPacket(w io.Writer, data []byte) error {
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(data)))
	if _, err := w.Write(lenBuf[:]); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}
