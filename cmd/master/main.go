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
		if n, err := strconv.Atoi(val); err == nil && n >= 0 {
			return n
		}
	}
	return defaultVal
}

func main() {
	log.Println("==================================================")
	log.Println(" Starting OpenMaster Universal Master Daemon")
	log.Println("==================================================")

	udpAddr := getEnv("UDP_ADDR", ":27900")
	kpq3Addr := getEnv("KPQ3_ADDR", ":27950")
	tcpAddr := getEnv("TCP_ADDR", ":28900")
	ut2004Addr := getEnv("UT2004_ADDR", ":28902")
	tcpV1Addr := getEnv("TCP_V1_ADDR", ":28910")
	apiAddr := getEnv("API_ADDR", "0.0.0.0:8085")
	allowPrivate := getEnvBool("ALLOW_PRIVATE", false)
	seedURLsRaw := getEnv("SEED_URLS", "https://raw.githubusercontent.com/shavargo/retro-server-mirror/main/servers.json")
	dbFile := getEnv("DB_FILE", "data/servers.json")
	blacklistRaw := getEnv("BLACKLIST_IPS", "88.97.18.58")

	// 1. Initialize Game Database
	gameReg, err := games.NewRegistry()
	if err != nil {
		log.Fatalf("Fatal: failed to load game catalog: %v", err)
	}
	log.Printf("Loaded %d games from GameSpy catalog", gameReg.TotalCount())

	// 2. Initialize In-Memory Store & Apply Blacklist
	store := registry.NewStore(allowPrivate)
	if blacklistRaw != "" {
		for _, ip := range strings.Split(blacklistRaw, ",") {
			store.AddBlacklist(ip)
		}
	}
	// Also load data/blacklist.txt if present
	if bData, err := os.ReadFile("data/blacklist.txt"); err == nil {
		for _, line := range strings.Split(string(bData), "\n") {
			store.AddBlacklist(line)
		}
	}

	if dbFile != "" {
		loaded, err := store.LoadFromFile(dbFile)
		if err != nil {
			log.Printf("[Store] Warning loading database from %s: %v", dbFile, err)
		} else if loaded > 0 {
			log.Printf("[Store] Successfully restored %d servers from JSON DB (%s)", loaded, dbFile)
		} else {
			log.Printf("[Store] No previous JSON DB found at %s (starting fresh)", dbFile)
		}
	}

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. Initialize Remote Seeder
	var seedURLs []string
	if seedURLsRaw != "" {
		seedURLs = strings.Split(seedURLsRaw, ",")
	}
	serverSeeder := seeder.NewSeeder(store, gameReg, seedURLs, 10*time.Minute)
	// When mirror seeding finishes or updates, rebuild snapshot and persist DB
	serverSeeder.SetOnSyncComplete(func() {
		log.Printf("[Seeder] Mirror sync complete (%d total registered servers). Rebuilding snapshot...", store.TotalCount())
		store.RebuildSnapshot()
		if dbFile != "" {
			if err := store.SaveToFile(dbFile); err != nil {
				log.Printf("[Store] Failed to save DB after seeder sync: %v", err)
			} else {
				log.Printf("[Store] Persisted %d servers to JSON DB", store.TotalCount())
			}
		}
	})

	// 4. Initialize Listeners
	dualUDP := listener.NewDualUDPListener(udpAddr, store, gameReg)
	kpq3UDP := listener.NewKPQ3Listener(kpq3Addr, store)
	tcpMaster := listener.NewTCPGameSpyListener(tcpAddr, store, gameReg)
	tcpV1Master := listener.NewTCPGameSpyListener(tcpV1Addr, store, gameReg)
	ut2004Master := listener.NewTCPUT2004Listener(ut2004Addr, store)
	apiServer := api.NewServer(apiAddr, store, gameReg)

	// Handle OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start Seeder & AutoSave
	go serverSeeder.Start(ctx)
	if dbFile != "" {
		go store.StartAutoSave(ctx, 30*time.Second, dbFile)
	}

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
			log.Printf("[TCP Master 28900] Stopped: %v", err)
		}
	}()

	go func() {
		if err := tcpV1Master.Start(ctx); err != nil {
			log.Printf("[TCP Master 28910] Stopped: %v", err)
		}
	}()

	go func() {
		if err := ut2004Master.Start(ctx); err != nil {
			log.Printf("[TCP UT2004 28902] Stopped: %v", err)
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

	// Persist database before exit
	if dbFile != "" {
		log.Printf("[Store] Persisting JSON database to %s...", dbFile)
		if err := store.SaveToFile(dbFile); err != nil {
			log.Printf("[Store] Error saving database on exit: %v", err)
		} else {
			log.Printf("[Store] Database successfully saved (%d servers preserved).", store.TotalCount())
		}
	}

	time.Sleep(1 * time.Second)
	log.Println("Master server terminated cleanly.")
}
