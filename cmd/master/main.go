package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"master-server/internal/api"
	"master-server/internal/checker"
	"master-server/internal/games"
	"master-server/internal/listener"
	"master-server/internal/registry"
	"master-server/internal/seeder"
)

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		b, err := strconv.ParseBool(val)
		if err == nil {
			return b
		}
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			return n
		}
	}
	return defaultVal
}

func main() {
	log.Println("==================================================")
	log.Println(" Starting GameSpy & Quake 2 Universal Master Daemon")
	log.Println("==================================================")

	udpAddr := getEnv("UDP_ADDR", ":27900")
	kpq3Addr := getEnv("KPQ3_ADDR", ":27950")
	tcpAddr := getEnv("TCP_ADDR", ":28900")
	apiAddr := getEnv("API_ADDR", "0.0.0.0:8085")
	allowPrivate := getEnvBool("ALLOW_PRIVATE", false)
	seedURLsRaw := getEnv("SEED_URLS", "https://raw.githubusercontent.com/shavargo/retro-server-mirror/main/servers.json")
	probeIntervalSec := getEnvInt("PROBE_INTERVAL_SECONDS", 180) // Gentler default: 3 minutes (180s) prevents Anti-DDoS rate-limits
	proberWorkers := getEnvInt("PROBER_WORKERS", 16)             // Conservative concurrency prevents UDP packet bursts
	probeTimeoutMs := getEnvInt("PROBE_TIMEOUT_MS", 2000)
	probePacingMs := getEnvInt("PROBE_PACING_MS", 20)            // 20ms pacing delay between probes per worker

	// 1. Initialize Game Database
	gameReg, err := games.NewRegistry()
	if err != nil {
		log.Fatalf("Fatal: failed to load game catalog: %v", err)
	}
	log.Printf("Loaded %d games from GameSpy catalog", gameReg.TotalCount())

	// 2. Initialize In-Memory Store
	store := registry.NewStore(allowPrivate)

	// 3. Initialize Background Health Prober
	log.Printf("[Prober] Initialized: interval=%ds, workers=%d, timeout=%dms, pacing=%dms",
		probeIntervalSec, proberWorkers, probeTimeoutMs, probePacingMs)
	prober := checker.NewProber(
		store,
		gameReg,
		time.Duration(probeIntervalSec)*time.Second,
		time.Duration(probeTimeoutMs)*time.Millisecond,
		proberWorkers,
		time.Duration(probePacingMs)*time.Millisecond,
	)

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 4. Initialize Remote Seeder
	var seedURLs []string
	if seedURLsRaw != "" {
		seedURLs = strings.Split(seedURLsRaw, ",")
	}
	serverSeeder := seeder.NewSeeder(store, gameReg, seedURLs, 10*time.Minute)
	// When mirror seeding finishes or updates, immediately trigger a probe pass
	serverSeeder.SetOnSyncComplete(func() {
		log.Printf("[Seeder] Mirror sync complete (%d total registered servers). Triggering health probes...", store.TotalCount())
		prober.Trigger(ctx)
	})

	// 5. Initialize Listeners
	dualUDP := listener.NewDualUDPListener(udpAddr, store, gameReg)
	kpq3UDP := listener.NewKPQ3Listener(kpq3Addr, store)
	tcpMaster := listener.NewTCPGameSpyListener(tcpAddr, store, gameReg)
	apiServer := api.NewServer(apiAddr, store, gameReg)

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start Prober & Seeder
	go prober.Start(ctx)
	go serverSeeder.Start(ctx)

	// Start Listeners
	go func() {
		if err := dualUDP.Start(ctx); err != nil {
			log.Printf("[Dual UDP] Stopped: %v", err)
		}
	}()

	go func() {
		if err := kpq3UDP.Start(ctx); err != nil {
			log.Printf("[KPQ3 UDP] Stopped: %v", err)
		}
	}()

	go func() {
		if err := tcpMaster.Start(ctx); err != nil {
			log.Printf("[TCP Master] Stopped: %v", err)
		}
	}()

	go func() {
		if err := apiServer.Start(ctx); err != nil {
			log.Printf("[API Server] Stopped: %v", err)
		}
	}()

	log.Println("All master services running smoothly.")

	<-sigChan
	log.Println("Shutdown signal received, draining connections...")
	cancel()

	time.Sleep(1 * time.Second)
	log.Println("Master server terminated cleanly.")
}
