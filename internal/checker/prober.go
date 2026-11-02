package checker

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"master-server/internal/games"
	"master-server/internal/gamespy"
	"master-server/internal/registry"
)

// Prober runs background UDP health checks on registered game servers.
type Prober struct {
	store        *registry.Store
	gameReg      *games.Registry
	pollInterval time.Duration
	probeTimeout time.Duration
	workerCount  int
	pacingDelay  time.Duration
	isProbing    atomic.Bool
}

// NewProber initializes a health checker.
func NewProber(
	store *registry.Store,
	gameReg *games.Registry,
	pollInterval, probeTimeout time.Duration,
	workers int,
	pacingDelay time.Duration,
) *Prober {
	if workers <= 0 {
		workers = 16
	}
	if probeTimeout <= 0 {
		probeTimeout = 2000 * time.Millisecond
	}
	if pollInterval <= 0 {
		pollInterval = 3 * time.Minute
	}
	if pacingDelay < 0 {
		pacingDelay = 20 * time.Millisecond
	}
	return &Prober{
		store:        store,
		gameReg:      gameReg,
		pollInterval: pollInterval,
		probeTimeout: probeTimeout,
		workerCount:  workers,
		pacingDelay:  pacingDelay,
	}
}

// Start begins the background check loop until context is canceled.
func (p *Prober) Start(ctx context.Context) {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	// Initial check immediately
	p.probeAll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.probeAll(ctx)
			p.store.Prune(30 * time.Minute)
			p.store.RebuildSnapshot()
		}
	}
}

// ProbeOnce runs a single synchronous probing pass across all registered servers.
func (p *Prober) ProbeOnce(ctx context.Context) {
	p.probeAll(ctx)
}

// Trigger asynchronously schedules a probe pass if not already in progress.
func (p *Prober) Trigger(ctx context.Context) {
	go p.probeAll(ctx)
}

// probeAll distributes probe jobs across worker goroutines.
func (p *Prober) probeAll(ctx context.Context) {
	if !p.isProbing.CompareAndSwap(false, true) {
		return // probe pass already running
	}
	defer p.isProbing.Store(false)

	servers := p.store.GetAllForProbing()
	if len(servers) == 0 {
		return
	}

	jobs := make(chan registry.Server, len(servers))
	for _, s := range servers {
		jobs <- s
	}
	close(jobs)

	var wg sync.WaitGroup
	for i := 0; i < p.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for srv := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
					p.probeSingle(srv)
					if p.pacingDelay > 0 {
						time.Sleep(p.pacingDelay)
					}
				}
			}
		}()
	}
	wg.Wait()
	p.store.RebuildSnapshot()
}

// probeSingle pings a single server over UDP and updates its status.
func (p *Prober) probeSingle(srv registry.Server) {
	proto := ""
	if p.gameReg != nil {
		proto = p.gameReg.ProtocolFor(srv.Gamename)
	}

	switch proto {
	case "quake3":
		if p.probeQuake3(srv) {
			return
		}
		if p.probeGameSpy(srv) {
			return
		}
	case "quake2":
		if p.probeQuake2(srv) {
			return
		}
		if p.probeGameSpy(srv) {
			return
		}
	default:
		// Default: try GameSpy first
		if p.probeGameSpy(srv) {
			return
		}
		// If GameSpy fails, try Quake3 / Quake2
		if p.probeQuake3(srv) {
			return
		}
		if p.probeQuake2(srv) {
			return
		}
	}

	p.store.MarkOffline(srv.IP, srv.QueryPort)
}

// probeGameSpy sends \status\ to the query port.
func (p *Prober) probeGameSpy(srv registry.Server) bool {
	addr := net.JoinHostPort(srv.IP, strconv.Itoa(srv.QueryPort))
	conn, err := net.DialTimeout("udp", addr, p.probeTimeout)
	if err != nil {
		return false
	}
	defer conn.Close()

	start := time.Now()
	if _, err := conn.Write([]byte("\\status\\")); err != nil {
		return false
	}

	_ = conn.SetReadDeadline(time.Now().Add(p.probeTimeout))
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return false
	}

	elapsed := time.Since(start)
	pingMs := int(elapsed.Milliseconds())
	if pingMs < 1 {
		pingMs = 1
	}

	raw := string(buf[:n])
	if strings.HasPrefix(raw, "\\") {
		msg := gamespy.ParseMessage(raw)
		p.handleGameSpyResponse(srv, msg, pingMs)
		return true
	}

	if bytes.HasPrefix(buf[:n], []byte{0xFF, 0xFF, 0xFF, 0xFF}) {
		p.handleQuakeResponse(srv, buf[:n], pingMs)
		return true
	}

	return false
}

// probeQuake3 sends \xff\xff\xff\xffgetstatus\n to the server.
func (p *Prober) probeQuake3(srv registry.Server) bool {
	addr := net.JoinHostPort(srv.IP, strconv.Itoa(srv.QueryPort))
	conn, err := net.DialTimeout("udp", addr, p.probeTimeout)
	if err != nil {
		return false
	}
	defer conn.Close()

	start := time.Now()
	payload := append([]byte{0xFF, 0xFF, 0xFF, 0xFF}, []byte("getstatus\n")...)
	if _, err := conn.Write(payload); err != nil {
		return false
	}

	_ = conn.SetReadDeadline(time.Now().Add(p.probeTimeout))
	buf := make([]byte, 8192)
	n, err := conn.Read(buf)
	if err != nil {
		return false
	}

	elapsed := time.Since(start)
	pingMs := int(elapsed.Milliseconds())
	if pingMs < 1 {
		pingMs = 1
	}

	if bytes.HasPrefix(buf[:n], []byte{0xFF, 0xFF, 0xFF, 0xFF}) || strings.Contains(string(buf[:n]), "statusResponse") {
		p.handleQuakeResponse(srv, buf[:n], pingMs)
		return true
	}

	if strings.HasPrefix(string(buf[:n]), "\\") {
		msg := gamespy.ParseMessage(string(buf[:n]))
		p.handleGameSpyResponse(srv, msg, pingMs)
		return true
	}

	return false
}

// probeQuake2 sends \xff\xff\xff\xffstatus\n to the server.
func (p *Prober) probeQuake2(srv registry.Server) bool {
	addr := net.JoinHostPort(srv.IP, strconv.Itoa(srv.QueryPort))
	conn, err := net.DialTimeout("udp", addr, p.probeTimeout)
	if err != nil {
		return false
	}
	defer conn.Close()

	start := time.Now()
	payload := append([]byte{0xFF, 0xFF, 0xFF, 0xFF}, []byte("status\n")...)
	if _, err := conn.Write(payload); err != nil {
		return false
	}

	_ = conn.SetReadDeadline(time.Now().Add(p.probeTimeout))
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return false
	}

	elapsed := time.Since(start)
	pingMs := int(elapsed.Milliseconds())
	if pingMs < 1 {
		pingMs = 1
	}

	if bytes.HasPrefix(buf[:n], []byte{0xFF, 0xFF, 0xFF, 0xFF}) || strings.Contains(string(buf[:n]), "print\n") {
		p.handleQuakeResponse(srv, buf[:n], pingMs)
		return true
	}

	if strings.HasPrefix(string(buf[:n]), "\\") {
		msg := gamespy.ParseMessage(string(buf[:n]))
		p.handleGameSpyResponse(srv, msg, pingMs)
		return true
	}

	return false
}

func (p *Prober) handleGameSpyResponse(srv registry.Server, msg *gamespy.Message, ping int) {
	hostname := msg.Get("hostname")
	if hostname == "" {
		hostname = msg.Get("sv_hostname")
	}
	mapname := msg.Get("mapname")
	if mapname == "" {
		mapname = msg.Get("map")
	}

	hostport, _ := strconv.Atoi(msg.Get("hostport"))
	if hostport == 0 {
		hostport = srv.GamePort
	}
	maxclients, _ := strconv.Atoi(msg.Get("maxclients"))
	if maxclients == 0 {
		maxclients, _ = strconv.Atoi(msg.Get("sv_maxclients"))
	}
	if maxclients == 0 {
		maxclients, _ = strconv.Atoi(msg.Get("maxplayers"))
	}

	var roster []registry.Player
	for i := 0; i < 64; i++ {
		pName := msg.Get(fmt.Sprintf("player_%d", i))
		if pName == "" {
			pName = msg.Get(fmt.Sprintf("name_%d", i))
		}
		if pName == "" {
			break
		}
		pFrags, _ := strconv.Atoi(msg.Get(fmt.Sprintf("frags_%d", i)))
		pPing, _ := strconv.Atoi(msg.Get(fmt.Sprintf("ping_%d", i)))
		roster = append(roster, registry.Player{
			Name:  registry.StripQuakeColors(pName),
			Frags: pFrags,
			Ping:  pPing,
		})
	}

	numPlayers, _ := strconv.Atoi(msg.Get("numplayers"))
	if numPlayers == 0 && len(roster) > 0 {
		numPlayers = len(roster)
	}

	p.store.UpdateProbeResult(
		srv.IP,
		srv.QueryPort,
		hostport,
		hostname,
		mapname,
		numPlayers,
		maxclients,
		ping,
		roster,
		msg.KV,
	)
}

func (p *Prober) handleQuakeResponse(srv registry.Server, buf []byte, ping int) {
	text := string(buf)
	text = strings.TrimPrefix(text, "\xff\xff\xff\xffprint\n")
	text = strings.TrimPrefix(text, "????print\n")
	text = strings.TrimPrefix(text, "\xff\xff\xff\xffstatusResponse\n")
	text = strings.TrimPrefix(text, "????statusResponse\n")
	text = strings.TrimPrefix(text, "\xff\xff\xff\xff")
	text = strings.TrimLeft(text, "\r\n")

	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return
	}

	// Line 0 is \key\value\ string
	infoMsg := gamespy.ParseMessage(lines[0])
	hostname := infoMsg.Get("hostname")
	if hostname == "" {
		hostname = infoMsg.Get("sv_hostname")
	}
	mapname := infoMsg.Get("mapname")
	if mapname == "" {
		mapname = infoMsg.Get("map")
	}

	hostport, _ := strconv.Atoi(infoMsg.Get("hostport"))
	if hostport == 0 {
		hostport = srv.GamePort
	}

	maxclients, _ := strconv.Atoi(infoMsg.Get("maxclients"))
	if maxclients == 0 {
		maxclients, _ = strconv.Atoi(infoMsg.Get("sv_maxclients"))
	}
	if maxclients == 0 {
		maxclients, _ = strconv.Atoi(infoMsg.Get("maxplayers"))
	}

	// Subsequent lines are "frags ping \"name\"" or "score ping \"name\""
	var roster []registry.Player
	for _, l := range lines[1:] {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		parts := strings.SplitN(l, " ", 3)
		if len(parts) >= 3 {
			frags, _ := strconv.Atoi(parts[0])
			pPing, _ := strconv.Atoi(parts[1])
			pName := strings.Trim(parts[2], "\"")
			roster = append(roster, registry.Player{
				Name:  registry.StripQuakeColors(pName),
				Frags: frags,
				Ping:  pPing,
			})
		}
	}

	p.store.UpdateProbeResult(
		srv.IP,
		srv.QueryPort,
		hostport,
		hostname,
		mapname,
		len(roster),
		maxclients,
		ping,
		roster,
		infoMsg.KV,
	)
}
